#!/bin/bash
set -e

# Usage: ./build_tests.sh <GOOS> <GOARCH>
GOOS=${1:-linux}
GOARCH=${2:-amd64}

# Output directory: build/tests/<GOOS>/<GOARCH>
OUT_DIR="build/tests/${GOOS}/${GOARCH}"
mkdir -p "$OUT_DIR"

echo "Building test binaries for ${GOOS}/${GOARCH} into ${OUT_DIR}..."

# If specific packages are provided as arguments (after GOOS and GOARCH), use those
# Otherwise, find all internal packages
if [[ $# -gt 2 ]]; then
    # Use packages from arguments
    PKGS="${@:3}"
else
    # Find all internal packages, excluding those with cgo/pcap dependencies
    # scanner: depends on gopacket which transitively pulls pcap (requires cgo)
    PKGS=$(go list ./internal/... | grep -v '/scanner$')
fi

# Function to build a single package
build_pkg() {
    local pkg=$1
    local out_dir=$2
    local goos=$3
    local goarch=$4

    local name=$(basename "$pkg")
    local out_path="${out_dir}/${name}.test"

    # We use go test -c which leverages the build cache.
    if ! output=$(CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go test -c -o "$out_path" "$pkg" 2>&1); then
        # Check if failure is due to no test files
        if echo "$output" | grep -q "no test files"; then
            return 0
        fi
        echo ""
        echo "Failed to build $pkg:"
        echo "$output"
        return 1
    fi
    echo -n "."
}

export -f build_pkg

# Use parallel execution
# We need to pass the function and variables to the subshell
# xargs -P 0 uses all available cores
echo "$PKGS" | xargs -P 0 -I {} bash -c "build_pkg '{}' '$OUT_DIR' '$GOOS' '$GOARCH'"

# Sentinel for flywall.sh incremental builds
touch "${OUT_DIR}/.test_sentinel"

echo ""
echo "Test build complete."