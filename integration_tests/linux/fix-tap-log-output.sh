#!/bin/sh
# Fix TAP-breaking log output in tests
# Replaces raw 'cat' with TAP-safe log helpers

set -e

echo "Fixing TAP-breaking log output..."

# Find tests that cat log files
grep -l "cat.*LOG\|cat.*\.log" /Users/ben/projects/flywall/integration_tests/linux/*/*.sh | while read file; do
    echo "Checking: $file"
    
    # Skip if already using log helpers
    if grep -q "log_helpers.sh" "$file"; then
        echo "  Already using log helpers"
        continue
    fi
    
    # Show problematic patterns
    echo "  Found cat patterns:"
    grep -n "cat.*LOG\|cat.*\.log" "$file" | head -3
    
    # Create backup
    backup="${file}.bak.taplogs.$(date +%Y%m%d_%H%M%S)}"
    cp "$file" "$backup"
    
    # Add log helpers import after common.sh
    sed -i.tmp '/\. "\$.*common.sh"/a\
\
# Load log helpers for TAP-safe output\
. "$(dirname "$0")/../lib/log_helpers.sh"
' "$file"
    
    # Replace common patterns
    sed -i.tmp \
        -e 's/\[ -f "$API_LOG" \] && cat "$API_LOG"/show_log_tail "$API_LOG" 10/g' \
        -e 's/\[ -f "$CTL_LOG" \] && cat "$CTL_LOG"/show_log_errors "$CTL_LOG"/g' \
        -e 's/cat "\$API_LOG"/show_log_tail "$API_LOG" 10/g' \
        -e 's/cat "\$CTL_LOG"/show_log_tail "$CTL_LOG" 10/g' \
        -e 's/cat \$API_LOG/show_log_tail "$API_LOG" 10/g' \
        -e 's/cat \$CTL_LOG/show_log_tail "$CTL_LOG" 10/g' \
        "$file"
    
    rm -f "${file}.tmp.tmp"
    
    if diff -q "$backup" "$file" >/dev/null; then
        echo "  No changes needed"
        rm -f "$backup"
    else
        echo "  Fixed! Backup: $backup"
    fi
done

echo ""
echo "Done! Tests now use TAP-safe log output:"
echo "- show_log_tail() - Shows last N lines with # prefix"
echo "- show_log_errors() - Shows only error/warning lines"
echo "- show_log_matches() - Shows lines matching a pattern"
echo ""
echo "This prevents breaking TAP format when showing logs."
