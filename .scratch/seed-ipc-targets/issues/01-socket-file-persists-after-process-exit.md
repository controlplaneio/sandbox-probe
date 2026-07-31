Type: research
Status: resolved

## Question

Does a bound Unix domain socket special file persist on disk after the
process that created it exits, or does it need a live listener held open
for `unix_socket_detection` (which only `stat()`s for socket-typed dir
entries — see `pkg/tasks/baseline/network.go`) to find it?

This determines whether socket seeding needs background-process lifecycle
management (start before scan, keep alive, tear down after, crash-safety)
or is a fire-and-forget file operation like the existing file decoys.

## Answer

Confirmed empirically, in a throwaway `mktemp -d` directory (nothing near
any real path):

```python
import socket
s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.bind('<tmpdir>/test.sock')
s.close()
```

After this, `<tmpdir>/test.sock` is a real socket-typed file on disk
(confirmed via `file`/`stat`) even though the process closed the socket and
no listener is running. `unix_socket_detection` would find it.

**Consequence for the mechanism ticket:** seeding a socket is a one-shot
`bind()` + `close()`, and cleanup is just deleting that one specific file —
no background process, no crash-safety/lifecycle problem to design around.
This significantly narrows the scope of
[the mechanism design ticket](02-safe-seeding-mechanism-design.md).
