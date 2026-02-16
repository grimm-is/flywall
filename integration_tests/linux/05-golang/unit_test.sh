# Dynamically find the test binary directory based on arch
ARCH=$(uname -m)
case $ARCH in
    x86_64) GOARCH="amd64" ;;
    aarch64) GOARCH="arm64" ;;
    *) GOARCH="amd64" ;;
esac

TEST_DIR="/mnt/flywall/build/tests/linux/$GOARCH"
set -x

if [ ! -d "$TEST_DIR" ]; then
    echo "1..1"
    echo "not ok 1 - Test directory not found: $TEST_DIR"
    exit 1
fi

# Count tests
count=$(find "$TEST_DIR" -name "*.test" | wc -l)
if [ "$count" -eq 0 ]; then
    echo "1..0 # SKIP no test binaries found in $TEST_DIR"
    exit 0
fi


# Ensure loopback is up (critical for tests binding to 127.0.0.1)
ip link set lo up 2>/dev/null || true

set +x
echo "1..$count"

i=1
for test_bin in "$TEST_DIR"/*.test; do
    name=$(basename "$test_bin" .test)

    # Use unique namespace for each test binary to isolate ports
    NS="unit_ns_$$"
    ip netns add $NS 2>/dev/null || true
    ip netns exec $NS ip link set lo up

    # Run test binary
    # -test.v: verbose output
    output=$(ip netns exec $NS "$test_bin" -test.v 2>&1)
    exit_code=$?

    ip netns del $NS 2>/dev/null || true

    if [ $exit_code -eq 0 ]; then
        echo "ok $i - $name"
    else
        echo "not ok $i - $name"
        echo "# Exit code: $exit_code"
        echo "$output" | sed 's/^/# /'
    fi
    i=$((i+1))
done
