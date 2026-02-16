#!/bin/sh
# Convert PID-based unique paths to TEST_PID-based paths
# TEST_PID is unique across VMs

set -e

echo "Converting PID-based paths to TEST_PID-based paths..."

# Find all files with PID-based paths
grep -l "_\$\.hcl\|_\$\.json\|_\$\.log" /Users/ben/projects/flywall/integration_tests/linux/*/*.sh | while read file; do
    echo "Converting: $file"
    
    # Skip if already using TEST_PID
    if grep -q "TEST_PID}" "$file"; then
        echo "  Already using TEST_PID"
        continue
    fi
    
    # Create backup
    backup="${file}.bak.pid2testpid.$(date +%Y%m%d_%H%M%S)"
    cp "$file" "$backup"
    
    # Convert $$ to ${TEST_PID}
    sed -i.tmp \
        -e 's|_\$\.|_${TEST_PID}.|g' \
        "$file"
    
    rm -f "${file}.tmp.tmp"
    
    # Show changes
    if diff -q "$backup" "$file" >/dev/null; then
        echo "  No changes needed"
        rm -f "$backup"
    else
        echo "  Converted! Backup: $backup"
        diff -u "$backup" "$file" | head -10 || true
    fi
done

echo ""
echo "Conversion complete!"
echo ""
echo "Verifying all files now use TEST_PID..."
grep -l "_\$\.hcl\|_\$\.json\|_\$\.log" /Users/ben/projects/flywall/integration_tests/linux/*/*.sh | wc -l
echo "files still using PID-based paths (should be 0)"
