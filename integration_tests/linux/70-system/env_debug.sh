#!/bin/sh
. "$(dirname "$0")/../common.sh"
echo "Dumping ID-like env vars:"
env | grep -iE "ID|ORCA|RUN|JOB"
exit 1
