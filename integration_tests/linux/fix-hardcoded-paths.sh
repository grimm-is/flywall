#!/bin/sh
# Fix hardcoded paths in test files
# Makes paths unique to prevent conflicts in parallel execution across VMs

set -e

# Find files with hardcoded /tmp paths
echo "Searching for hardcoded paths..."
grep -l "/tmp/.*\.hcl\|/tmp/.*\.json\|/tmp/.*\.log" /Users/ben/projects/flywall/integration_tests/linux/*/*.sh | while read file; do
    echo "Checking: $file"
    
    # Skip already fixed files
    if grep -q "_\$\.hcl\|_\$\.json\|_\$\.log" "$file"; then
        echo "  Already fixed with PID"
        continue
    fi
    
    # Skip if already using TEST_UID or TEST_PID
    if grep -q "TEST_UID}\|TEST_PID}" "$file"; then
        echo "  Already using unique ID"
        continue
    fi
    
    # Create backup
    backup="${file}.bak.$(date +%Y%m%d_%H%M%S)"
    cp "$file" "$backup"
    
    # Add TEST_PID import if not present
    if ! grep -q "common.sh" "$file"; then
        echo "  WARNING: File doesn't source common.sh, TEST_UID may not be available"
    fi
    
    # Fix common patterns using TEST_UID for cross-VM uniqueness
    sed -i.tmp \
        -e 's|/tmp/\([a-zA-Z0-9_]*\)\.hcl|/tmp/\1_${TEST_UID}.hcl|g' \
        -e 's|/tmp/\([a-zA-Z0-9_]*\)\.json|/tmp/\1_${TEST_UID}.json|g' \
        -e 's|/tmp/\([a-zA-Z0-9_]*\)\.log|/tmp/\1_${TEST_UID}.log|g' \
        -e 's|/tmp/\([a-zA-Z0-9_]*\)\.sock|/tmp/\1_${TEST_UID}.sock|g' \
        -e 's|/tmp/\([a-zA-Z0-9_]*\)\.txt|/tmp/\1_${TEST_UID}.txt|g' \
        "$file"
    
    rm -f "${file}.tmp.tmp"
    
    # Show changes
    if diff -q "$backup" "$file" >/dev/null; then
        echo "  No changes needed"
        rm -f "$backup"
    else
        echo "  Fixed! Backup: $backup"
        # Show first few changes
        diff -u "$backup" "$file" | head -20 || true
    fi
done

echo ""
echo "Done! Check for any remaining conflicts:"
echo ""
echo "Common patterns to check:"
echo "1. CONFIG_FILE= (should be unique)"
echo "2. STATE_DIR= (should be unique)"
echo "3. LOG_FILE= (should be unique)"
echo "4. /opt/flywall/var/lib (shared state - might need cleanup)"
echo ""
grep -n "CONFIG_FILE=.*[^$]" /Users/ben/projects/flywall/integration_tests/linux/*/*.sh | grep -v "\$\$$\|TEST_PID\|TEST_UID" | head -10 || echo "No remaining CONFIG_FILE issues found"
