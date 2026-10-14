#!/bin/sh
# Disposable diagnostic container: no app volumes, credentials or remote display.
set -eu
export DISPLAY=:99
Xvfb "$DISPLAY" -screen 0 1280x1024x24 -nolisten tcp >/dev/null 2>&1 &
display_pid=$!
trap 'kill "$display_pid" 2>/dev/null || true; wait "$display_pid" 2>/dev/null || true' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
attempt=0
while [ ! -S /tmp/.X11-unix/X99 ]; do
    kill -0 "$display_pid" 2>/dev/null || { echo 'Virtual display failed to start' >&2; exit 1; }
    attempt=$((attempt + 1))
    [ "$attempt" -lt 100 ] || { echo 'Virtual display startup timed out' >&2; exit 1; }
    sleep 0.1
done
node "/opt/sente/fnb/$1" --visible
