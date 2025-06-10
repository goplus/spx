#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd $SCRIPT_DIR
# Check if port is provided as a command line argument
if [ -z "$1" ]
then
    PORT=8005
else
    PORT=$1
fi
echo "Killing run.py if running..."
PIDS=$(pgrep -f run.py); 
if [ -n "$PIDS" ]; then 
    echo "Killing process: $PIDS"; 
    kill -9 $PIDS; 
else 
    echo "No run.py process found."; 
fi	

# Determine python executable (python3 preferred, fallback to python)
if command -v python3 >/dev/null 2>&1; then
    PYTHON_CMD=python3
elif command -v python >/dev/null 2>&1; then
    PYTHON_CMD=python
else
    echo "Error: Python interpreter not found. Please install Python."
    exit 1
fi

# Start python in the background
$PYTHON_CMD run.py -p $PORT &