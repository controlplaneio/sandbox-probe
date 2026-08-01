#!/usr/bin/env bash

# Shared validation for legacy run-* harnesses.
validate_runner_inputs() {
  if [[ "$#" -ne 2 ]]; then
    echo "usage: $0 PROBE OUTDIR" >&2
    return 64
  fi

  local probe=$1 outdir=$2
  if [[ ! -f "$probe" ]]; then
    echo "ERROR: probe does not exist or is not a regular file: $probe" >&2
    return 66
  fi
  if [[ ! -x "$probe" ]]; then
    echo "ERROR: probe is not executable: $probe" >&2
    return 66
  fi
  if [[ ! -d "$outdir" ]]; then
    echo "ERROR: output directory does not exist: $outdir" >&2
    return 73
  fi
}
