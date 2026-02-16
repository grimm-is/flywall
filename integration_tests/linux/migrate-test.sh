#!/bin/sh
# Test Migration Helper
# Converts existing tests to use the new framework

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEST_FILE="$1"

if [ -z "$TEST_FILE" ]; then
    echo "Usage: $0 <test-file.sh>"
    echo ""
    echo "Migrates an existing test to use the new framework."
    echo "Creates a backup of the original file."
    exit 1
fi

if [ ! -f "$TEST_FILE" ]; then
    echo "Error: Test file '$TEST_FILE' not found"
    exit 1
fi

# Create backup
backup_file="${TEST_FILE}.bak.$(date +%Y%m%d_%H%M%S)"
cp "$TEST_FILE" "$backup_file"
echo "Created backup: $backup_file"

# Migration patterns
migrate_test() {
    local temp_file=$(mktemp)
    
    # Apply transformations
    sed -e 's|\. "$(dirname "$0")/../common\.sh"|. "$(dirname "$0")/../test-framework.sh"|g' \
        -e 's|^require_root|# require_root (handled by framework)|g' \
        -e 's|^require_binary|# require_binary (handled by framework)|g' \
        -e 's|^cleanup_on_exit|# cleanup_on_exit (handled by framework)|g' \
        -e 's|plan \([0-9]*\)|TEST_PLAN=\1|g' \
        -e 's|^diag \(.*\)|tap_diag "\1"|g' \
        -e 's|ok \$? "\([^"]*\)"|tap_ok $? "\1"|g' \
        -e 's|not ok "\([^"]*\)"|tap_not_ok "\1"|g' \
        -e 's|fail "\([^"]*\)"|tap_not_ok "\1"|g' \
        -e 's|pass "\([^"]*\)"|tap_ok 0 "\1"|g' \
        -e 's|skip "\([^"]*\)"|tap_skip "\1"|g' \
        "$TEST_FILE" > "$temp_file"
    
    # Add framework-specific improvements
    cat > "$TEST_FILE" <<'EOF'
#!/bin/sh
# Migrated to use Flywall Test Framework

# Source the framework
. "$(dirname "$0")/../test-framework.sh"

EOF
    
    # Append the migrated content (skipping the shebang and common.sh sourcing)
    tail -n +3 "$temp_file" >> "$TEST_FILE"
    
    rm -f "$temp_file"
    
    echo "Migration complete!"
    echo ""
    echo "Next steps:"
    echo "1. Review the migrated test"
    echo "2. Replace hardcoded ports with allocate_port calls"
    echo "3. Replace start_ctl/start_api with enhanced versions"
    echo "4. Add proper setup() and main() functions"
    echo "5. Test the migrated version"
}

# Show migration suggestions
show_suggestions() {
    echo ""
    echo "Migration Suggestions for $TEST_FILE:"
    echo "====================================="
    echo ""
    
    # Check for hardcoded ports
    if grep -q ":808[0-9]\|listen.*=" "$TEST_FILE"; then
        echo "✓ Found hardcoded ports - replace with allocate_port:"
        grep -n ":808[0-9]\|listen.*=" "$TEST_FILE" | head -5
        echo ""
    fi
    
    # Check for port randomization
    if grep -q "rand.*10000" "$TEST_FILE"; then
        echo "✓ Found manual port randomization - can be simplified:"
        grep -n "rand.*10000" "$TEST_FILE" | head -3
        echo ""
    fi
    
    # Check for service starts
    if grep -q "start_ctl\|start_api" "$TEST_FILE"; then
        echo "✓ Found service starts - consider enhanced versions:"
        grep -n "start_ctl\|start_api" "$TEST_FILE" | head -3
        echo ""
    fi
    
    # Check for manual cleanup
    if grep -q "rm -f.*CONFIG\|ip netns del" "$TEST_FILE"; then
        echo "✓ Found manual cleanup - can use add_cleanup hooks:"
        grep -n "rm -f.*CONFIG\|ip netns del" "$TEST_FILE" | head -3
        echo ""
    fi
    
    # Check for TAP issues
    if grep -q "ok \$?" "$TEST_FILE"; then
        echo "✓ Found ok \$? pattern - converted to tap_ok"
        echo ""
    fi
}

# Perform migration
migrate_test

# Show suggestions
show_suggestions

echo ""
echo "Example improvements to consider:"
echo "--------------------------------"
echo ""
echo "# Instead of:"
echo "API_PORT=\$(awk 'BEGIN{srand(); print int(rand()*10000 + 10000)}')"
echo ""
echo "# Use:"
echo "allocate_port API_PORT"
echo ""
echo ""
echo "# Instead of:"
echo "start_ctl \"\$CONFIG_FILE\""
echo "rm -f \"\$CONFIG_FILE\""
echo ""
echo "# Use:"
echo "start_ctl_enhanced \"\$CONFIG_FILE\""
echo "# Cleanup is automatic!"
echo ""
echo ""
echo "# Instead of:"
echo "if [ \"\$response\" = \"200\" ]; then"
echo "    ok 0 \"Test passed\""
echo "else"
echo "    ok 1 \"Test failed\""
echo "fi"
echo ""
echo "# Use:"
echo "run_test \"Test description\" \"[ \$response = 200 ]\""
