#!/usr/bin/env bash

STATE_FILE="/rauc/rauc-active-slot"

# Initialize default state to 'A' if the file doesn't exist yet
if [ ! -f "$STATE_FILE" ]; then
    echo "A" > "$STATE_FILE"
fi

case "$1" in
    get-primary)
        # Read and return the current active slot
        cat "$STATE_FILE"
        ;;
    set-primary)
        # RAUC passes the target bootname (A or B) as the second argument
        # We write this new target to our state file
        echo "$2" > "$STATE_FILE"
        exit 0
        ;;
    get-state)
        # RAUC checks if the booted slot is marked as 'good' or 'bad'
        echo "good"
        ;;
    set-state)
        # RAUC marks a slot as good/bad here, we just accept it
        exit 0
        ;;
esac
exit 0
