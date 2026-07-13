package tasks

// import of
// https://pkg.go.dev/golang.org/x/tools@v0.14.0/internal/fastwalk

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// ErrTraverseLink is used as a return value from WalkFuncs to indicate that the
// symlink named in the call may be traversed.
var ErrTraverseLink = errors.New("fastwalk: traverse symlink, assuming target is a directory")

// ErrSkipFiles is a used as a return value from WalkFuncs to indicate that the
// callback should not be called for any other files in the current directory.
// Child directories will still be traversed.
var ErrSkipFiles = errors.New("fastwalk: skip remaining files in directory")

func Walk(root string, fast bool, walkFn func(path string, typ os.FileMode) error) error {
	numWorkers := 4
	if n := runtime.NumCPU(); n*2 > numWorkers {
		numWorkers = n * 2
	}

	// Make sure to wait for all workers to finish, otherwise
	// walkFn could still be called after returning. This Wait call
	// runs after close(e.donec) below.
	var wg sync.WaitGroup
	defer wg.Wait()

	w := &walker{
		fn:       walkFn,
		enqueuec: make(chan walkItem, numWorkers), // buffered for performance
		workc:    make(chan walkItem, numWorkers), // buffered for performance
		donec:    make(chan struct{}),

		// buffered for correctness & not leaking goroutines:
		resc: make(chan error, numWorkers),

		fast: fast,
	}
	defer close(w.donec)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go w.doWork(&wg)
	}
	todo := []walkItem{{dir: root}}
	out := 0
	for {
		workc := w.workc
		var workItem walkItem
		if len(todo) == 0 {
			workc = nil
		} else {
			workItem = todo[len(todo)-1]
		}
		select {
		case workc <- workItem:
			todo = todo[:len(todo)-1]
			out++
		case it := <-w.enqueuec:
			todo = append(todo, it)
		case err := <-w.resc:
			out--
			if err != nil {
				return err
			}
			if out == 0 && len(todo) == 0 {
				// It's safe to quit here, as long as the buffered
				// enqueue channel isn't also readable, which might
				// happen if the worker sends both another unit of
				// work and its result before the other select was
				// scheduled and both w.resc and w.enqueuec were
				// readable.
				select {
				case it := <-w.enqueuec:
					todo = append(todo, it)
				default:
					return nil
				}
			}
		}
	}
}

// doWork reads directories as instructed (via workc) and runs the
// user's callback function.
func (w *walker) doWork(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-w.donec:
			return
		case it := <-w.workc:
			select {
			case <-w.donec:
				return
			case w.resc <- w.walk(it.dir, !it.callbackDone):
			}
		}
	}
}

type walker struct {
	fn func(path string, typ os.FileMode) error

	donec    chan struct{} // closed on fastWalk's return
	workc    chan walkItem // to workers
	enqueuec chan walkItem // from workers
	resc     chan error    // from workers

	// skip some known large locations which are unlikely to have sockets
	fast bool
}

type walkItem struct {
	dir          string
	callbackDone bool // callback already called; don't do it again
}

func (w *walker) enqueue(it walkItem) {
	select {
	case w.enqueuec <- it:
	case <-w.donec:
	}
}

func (w *walker) onDirEnt(dirName, baseName string, typ os.FileMode) error {
	// Skip macOS APFS firmlink aliases (/System/Volumes/Data etc.): they re-expose volumes already
	// reachable from /, so descending re-walks the whole tree for no new sockets. macOS-only path.
	if typ == os.ModeDir && dirName == "/System/Volumes" {
		return nil
	}

	// ! added
	if w.fast {
		// skip logic to speed up processing
		if dirName == "/nix" && baseName == "store" {
			return nil
		}
		if baseName == "node_modules" || baseName == "" {
			return nil
		}
	}

	// don't join / to / making // prefix for everything
	osSeparator := string(os.PathSeparator)
	var joined string
	if dirName != osSeparator {
		joined = dirName
	}
	joined = joined + string(os.PathSeparator) + baseName
	if typ == os.ModeDir {
		w.enqueue(walkItem{dir: joined})
		return nil
	}

	err := w.fn(joined, typ)
	if typ == os.ModeSymlink {
		if err == ErrTraverseLink {
			// Set callbackDone so we don't call it twice for both the
			// symlink-as-symlink and the symlink-as-directory later:
			w.enqueue(walkItem{dir: joined, callbackDone: true})
			return nil
		}
		if err == filepath.SkipDir {
			// Permit SkipDir on symlinks too.
			return nil
		}
	}
	return err
}

func (w *walker) walk(root string, runUserCallback bool) error {
	if runUserCallback {
		err := w.fn(root, os.ModeDir)
		if err == filepath.SkipDir {
			return nil
		}
		if err != nil {
			return err
		}
	}

	err := readDir(root, w.onDirEnt)
	if err != nil {
		return err
	}
	return err
}

func isAllowedWalkError(err error) bool {
	return true
	// return errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrInvalid)
}

// readDir calls fn for each directory entry in dirName.
// It does not descend into directories or follow symlinks.
// If fn returns a non-nil error, readDir returns with that error
// immediately.
// ! adjusted to not return immediately
func readDir(dirName string, fn func(dirName, entName string, typ os.FileMode) error) error {
	fis, err := os.ReadDir(dirName)
	if err != nil {
		// ! added
		if isAllowedWalkError(err) {
			return nil
		}

		return err
	}
	skipFiles := false
	for _, fi := range fis {
		info, err := fi.Info()
		if err != nil {
			// ! added
			if isAllowedWalkError(err) {
				return nil
			}

			return err
		}
		if info.Mode().IsRegular() && skipFiles {
			continue
		}
		if err := fn(dirName, fi.Name(), info.Mode()&os.ModeType); err != nil {
			if err == ErrSkipFiles {
				skipFiles = true
				continue
			}

			// ! added
			if isAllowedWalkError(err) {
				continue
			}

			return err
		}
	}
	return nil
}
