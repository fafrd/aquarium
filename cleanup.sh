#!/bin/bash
[ -f aquarium.log ] && rm aquarium.log && echo "Removed aquarium.log"
[ -f debug.log ] && rm debug.log && echo "Removed debug.log"
[ -f terminal.log ] && rm terminal.log && echo "Removed terminal.log"

echo "Cleanup complete."
