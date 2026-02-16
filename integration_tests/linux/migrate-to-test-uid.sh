#!/bin/sh
# Migrate from TEST_PID to TEST_UID for better uniqueness
# TEST_UID uses ORCA_TEST_ID + PID for guaranteed uniqueness across VMs

set -e

echo "Migrating from TEST_PID to TEST_UID..."

# Count files to migrate
total=$(grep -l "TEST_PID}" /Users/ben/projects/flywall/integration_tests/linux/*/*.sh | wc -l)
echo "Found $total files using TEST_PID"

if [ "$total" -eq 0 ]; then
    echo "No files need migration!"
    exit 0
fi

# Migrate each file
grep -l "TEST_PID}" /Users/ben/projects/flywall/integration_tests/linux/*/*.sh | while read file; do
    echo "Migrating: $file"
    
    # Create backup
    backup="${file}.bak.migrate2uid.$(date +%Y%m%d_%H%M%S)"
    cp "$file" "$backup"
    
    # Replace TEST_PID with TEST_UID
    sed -i.tmp 's/${TEST_PID}/${TEST_UID}/g' "$file"
    
    rm -f "${file}.tmp.tmp"
    
    # Show changes
    if diff -q "$backup" "$file" >/dev/null; then
        echo "  No changes needed"
        rm -f "$backup"
    else
        echo "  Migrated! Backup: $backup"
        # Show a sample change
        diff -u "$backup" "$file" | grep "^-\|^+" | head -6 || true
    fi
done

echo ""
echo "Migration complete!"
echo ""
echo "Benefits of TEST_UID:"
echo "- Uses ORCA_TEST_ID for guaranteed uniqueness across test runs"
echo "- More traceable (can trace files back to specific test runs)"
echo "- Still includes PID for VM uniqueness"
echo ""
echo "Example IDs:"
echo "- TEST_PID: 12345-ff"
echo "- TEST_UID: 20260203-abc123-12345"
echo ""
echo "Verifying migration..."
remaining=$(grep -l "TEST_PID}" /Users/ben/projects/flywall/integration_tests/linux/*/*.sh | wc -l)
echo "Files still using TEST_PID: $remaining (should be 0)"

if [ "$remaining" -gt 0 ]; then
    echo ""
    echo "Remaining files with TEST_PID:"
    grep -l "TEST_PID}" /Users/ben/projects/flywall/integration_tests/linux/*/*.sh
fi
