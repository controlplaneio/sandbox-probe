package tasks

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// THE safety test, and it deliberately carries no build tag.
//
// Opening a real service's pipe is not passive: it consumes a server instance, delivers a
// connection event, and can hang a badly written server on somebody's laptop. The entire
// safety argument for this feature is that reachability only ever opens a name no real
// service can hold, so that invariant must be checkable by a contributor on Linux or macOS
// who cannot run a single line of the Windows code it protects.
//
// The most likely way to break it is a well-meaning "why not measure docker_engine too, it
// is more realistic". This is what stops that merging green.
func TestReachabilityNeverTargetsACatalogueName(t *testing.T) {
	probed := []string{ReachPipeName, reachControlPipeName()}

	for _, tg := range pipeTargets() {
		for _, p := range probed {
			if strings.EqualFold(p, tg.Path) {
				t.Fatalf("reachability would open %q, which is a real service's name: opening it "+
					"consumes a server instance and delivers a connection event to somebody "+
					"else's software", p)
			}
		}
	}

	for _, p := range probed {
		assert.True(t, strings.HasPrefix(p, pipePrefix), "%q is not a pipe path", p)
		assert.Contains(t, p, "sandbox-probe", "%q does not announce itself as the probe's own", p)
	}
}

// The reachability decoy must never reach the catalogue, or something downstream will treat
// it as a tool that exists in the world.
func TestReachDecoyIsNotACatalogueEntry(t *testing.T) {
	for _, tg := range listTargetsForHome("/home/tester", "") {
		assert.NotEqual(t, ReachPipeName, tg.Path,
			"the reachability decoy leaked into the target catalogue")
	}
}

// The `absent != empty` contract, which is the rule that keeps a false "blocked" out of the
// comparison. Portable, because the accessors are where the rule lives.
func TestOnlyAReachedRoundTripIsScored(t *testing.T) {
	for _, tt := range []struct {
		name     string
		reach    PipeReach
		want     []string
		wantOK   bool
		whyMatte string
	}{
		{
			name:     "reached",
			reach:    PipeReach{Reached: []string{ReachPipeName}, Status: map[string]any{"decoy_enumerated": true}},
			want:     []string{ReachPipeName},
			wantOK:   true,
			whyMatte: "a completed round trip is the only proof of reach",
		},
		{
			name:     "enumerated but not reachable",
			reach:    PipeReach{Reached: []string{}, Status: map[string]any{"decoy_enumerated": true}},
			want:     []string{},
			wantOK:   true,
			whyMatte: "the decoy was planted and could not be opened: a measured negative, which is a real finding",
		},
		{
			name:     "decoy never enumerated",
			reach:    PipeReach{Reached: []string{}, Status: map[string]any{"decoy_enumerated": false}},
			want:     nil,
			wantOK:   false,
			whyMatte: "nothing was planted, so publishing [] would claim a sandbox blocked something never tested",
		},
		{
			name:     "namespace denied",
			reach:    PipeReach{Reached: []string{}, Status: map[string]any{"namespace": "denied"}},
			want:     nil,
			wantOK:   false,
			whyMatte: "the enumeration itself failed, so nothing about opening is knowable",
		},
		{
			name:     "off windows",
			reach:    PipeReach{},
			want:     nil,
			wantOK:   false,
			whyMatte: "a zero PipeReach must produce no finding at all",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.reach.Names()
			assert.Equal(t, tt.wantOK, ok, tt.whyMatte)
			assert.Equal(t, tt.want, got, tt.whyMatte)
		})
	}
}

// Creation is reported by presence, never by a finding that says false — that would invert
// the whole report contract, in which an absent finding means the capability was denied.
func TestCreatedPipeReportsByPresence(t *testing.T) {
	got, ok := PipeReach{Created: `\\.\pipe\x`}.CreatedPipe()
	assert.True(t, ok)
	assert.Equal(t, []string{`\\.\pipe\x`}, got)

	_, ok = PipeReach{Created: ""}.CreatedPipe()
	assert.False(t, ok, "a failed creation must emit no finding, not a finding saying false")
}
