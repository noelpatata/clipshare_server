#!/bin/sh
set -e

# Start a virtual X server so xclip has a display to talk to.
Xvfb :99 -ac -screen 0 1x1x8 &
export DISPLAY=:99

# Give Xvfb a moment to create the socket.
sleep 0.3

exec "$@"
