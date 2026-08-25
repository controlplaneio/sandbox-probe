//go:build !windows
// +build !windows

package tasks

// Off Windows there is no pipe namespace, so the measurement returns a zero PipeReach: nil
// Status, nil Reached, empty Created. Every accessor then reports "unmeasured" and no
// named-pipe reachability finding appears in a non-Windows report at all — the same seam
// ListNamedPipes uses to keep named_pipe_detection off other platforms.
func MeasurePipeReach() PipeReach { return PipeReach{} }
