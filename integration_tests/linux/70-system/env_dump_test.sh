#!/bin/sh
. "$(dirname "$0")/../common.sh"
echo "--- ENV DUMP ---"
env | sort
echo "--- END ENV DUMP ---"
