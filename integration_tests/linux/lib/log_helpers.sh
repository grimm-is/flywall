#!/bin/sh
# Log Helper Functions
# Provides ways to show log content without breaking TAP format

# Show last N lines of a log file with TAP-safe formatting
show_log_tail() {
    local file="$1"
    local lines="${2:-10}"
    local prefix="${3:-#}"
    
    if [ -f "$file" ]; then
        echo "$prefix Last $lines lines of $(basename "$file"):"
        tail -n "$lines" "$file" | sed "s/^/$prefix /"
    else
        echo "$prefix Log file not found: $file"
    fi
}

# Show log lines matching a pattern
show_log_matches() {
    local file="$1"
    local pattern="$2"
    local prefix="${3:-#}"
    
    if [ -f "$file" ]; then
        local matches=$(grep "$pattern" "$file" | wc -l)
        if [ "$matches" -gt 0 ]; then
            echo "$prefix Found $matches matching lines in $(basename "$file"):"
            grep "$pattern" "$file" | sed "s/^/$prefix /"
        else
            echo "$prefix No matches found for pattern: $pattern"
        fi
    else
        echo "$prefix Log file not found: $file"
    fi
}

# Show error messages from log
show_log_errors() {
    local file="$1"
    local prefix="${2:-#}"
    
    show_log_matches "$file" "[Ee]rror|[Ff]ail|[Ww]arn" "$prefix"
}

# Show recent log entries (last minute)
show_log_recent() {
    local file="$1"
    local prefix="${2:-#}"
    
    if [ -f "$file" ]; then
        # Get timestamp from last minute (adjust as needed for your log format)
        local recent=$(find "$file" -mmin -1 2>/dev/null)
        if [ -n "$recent" ]; then
            echo "$prefix Recent log entries (last minute):"
            tail -n 20 "$file" | sed "s/^/$prefix /"
        else
            echo "$prefix No recent entries (file older than 1 minute)"
        fi
    else
        echo "$prefix Log file not found: $file"
    fi
}

# Show log summary (count of error/warning/info lines)
show_log_summary() {
    local file="$1"
    local prefix="${2:-#}"
    
    if [ -f "$file" ]; then
        local errors=$(grep -c "[Ee]rror" "$file" 2>/dev/null || echo "0")
        local warnings=$(grep -c "[Ww]arn" "$file" 2>/dev/null || echo "0")
        local info=$(grep -c "[Ii]nfo" "$file" 2>/dev/null || echo "0")
        
        echo "$prefix Log summary for $(basename "$file"):"
        echo "$prefix   Errors: $errors"
        echo "$prefix   Warnings: $warnings"
        echo "$prefix   Info: $info"
    else
        echo "$prefix Log file not found: $file"
    fi
}

# TAP-safe way to show logs on failure
show_logs_on_failure() {
    local test_result="$1"
    shift
    
    if [ "$test_result" != "0" ]; then
        for file in "$@"; do
            echo "# --- Log: $file ---"
            show_log_tail "$file" 5 "#"
            echo "# --- End Log ---"
        done
    fi
}
