#!/bin/bash
# Genie UI Accessibility Verification Script
# Quick wrapper to run accessibility checks on the Genie UI

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PYTHON_SCRIPT="$SCRIPT_DIR/a11y-verify.py"

# Check if Python 3 is available
if ! command -v python3 &> /dev/null; then
    echo "ERROR: python3 is required but not installed."
    exit 1
fi

# Check if the Python script exists
if [ ! -f "$PYTHON_SCRIPT" ]; then
    echo "ERROR: Accessibility verification script not found at $PYTHON_SCRIPT"
    exit 1
fi

# Run the verification script
echo "Running Genie UI Accessibility Verification..."
echo

python3 "$PYTHON_SCRIPT" "$@"
