#!/bin/bash
# Script to install git hooks.

HOOKS_DIR=$(git rev-parse --git-path hooks)
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

echo "Installing git hooks to $HOOKS_DIR..."

ln -sf "$SCRIPT_DIR/pre-commit.sh" "$HOOKS_DIR/pre-commit"
chmod +x "$SCRIPT_DIR/pre-commit.sh"

echo "Done."
