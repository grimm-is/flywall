#!/usr/bin/env bash

# Flywall Firewall - Unified Build & Test System
# A robust replacement for the legacy Makefile

set -e

# --- Configuration & Colors ---
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# --- Project Paths ---
# Robustly find the script's directory, following symlinks
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ]; do
    DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE"
done
PROJECT_ROOT="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

# Always run from the project root to ensure relative paths work regardless of where fw is called
cd "${PROJECT_ROOT}"

BUILD_DIR="${PROJECT_ROOT}/build"
UI_DIR="${PROJECT_ROOT}/ui"
SCRIPTS_DIR="${PROJECT_ROOT}/scripts"

# --- Brand Configuration ---
if [[ -f "${PROJECT_ROOT}/internal/brand/brand.json" ]]; then
    BRAND_NAME=$(jq -r '.name' "${PROJECT_ROOT}/internal/brand/brand.json")
    BRAND_BINARY=$(jq -r '.binaryName' "${PROJECT_ROOT}/internal/brand/brand.json")
else
    BRAND_NAME="Flywall"
    BRAND_BINARY="flywall"
fi

# --- Architecture Detection ---
HOST_ARCH=$(uname -m)
case "${HOST_ARCH}" in
    x86_64)  LINUX_ARCH="amd64" ;;
    arm64|aarch64) LINUX_ARCH="arm64" ;;
    *)       LINUX_ARCH="amd64" ;;
esac

RAW_OS=$(uname -s)
case "${RAW_OS}" in
    Darwin) HOST_OS="darwin" ;;
    Linux)  HOST_OS="linux" ;;
    *)      HOST_OS="linux" ;; # Fallback
esac

# --- Build Metadata ---
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
GIT_MERGE_BASE=$(git merge-base HEAD origin/main 2>/dev/null | head -c 7 || echo "unknown")

LDFLAGS="-X 'grimm.is/flywall/internal/brand.Version=${VERSION}' \
         -X 'grimm.is/flywall/internal/brand.BuildTime=${BUILD_TIME}' \
         -X 'grimm.is/flywall/internal/brand.BuildArch=linux/${LINUX_ARCH}' \
         -X 'grimm.is/flywall/internal/brand.GitCommit=${GIT_COMMIT}' \
         -X 'grimm.is/flywall/internal/brand.GitBranch=${GIT_BRANCH}' \
         -X 'grimm.is/flywall/internal/brand.GitMergeBase=${GIT_MERGE_BASE}'"

TOOL_LDFLAGS="-X 'grimm.is/flywall/tools/pkg/brand.Version=${VERSION}' \
         -X 'grimm.is/flywall/tools/pkg/brand.BuildTime=${BUILD_TIME}' \
         -X 'grimm.is/flywall/tools/pkg/brand.BuildArch=linux/${LINUX_ARCH}' \
         -X 'grimm.is/flywall/tools/pkg/brand.GitCommit=${GIT_COMMIT}' \
         -X 'grimm.is/flywall/tools/pkg/brand.GitBranch=${GIT_BRANCH}' \
         -X 'grimm.is/flywall/tools/pkg/brand.GitMergeBase=${GIT_MERGE_BASE}'"

# --- Build-Time Path Overrides ---
# Allow standard FHS paths to be injected via environment variables
if [[ -n "${FW_CONFIG_DIR}" ]]; then LDFLAGS="${LDFLAGS} -X 'grimm.is/flywall/internal/brand.BuildDefaultConfigDir=${FW_CONFIG_DIR}'"; fi
if [[ -n "${FW_STATE_DIR}" ]]; then LDFLAGS="${LDFLAGS} -X 'grimm.is/flywall/internal/brand.BuildDefaultStateDir=${FW_STATE_DIR}'"; fi
if [[ -n "${FW_LOG_DIR}" ]]; then LDFLAGS="${LDFLAGS} -X 'grimm.is/flywall/internal/brand.BuildDefaultLogDir=${FW_LOG_DIR}'"; fi
if [[ -n "${FW_CACHE_DIR}" ]]; then LDFLAGS="${LDFLAGS} -X 'grimm.is/flywall/internal/brand.BuildDefaultCacheDir=${FW_CACHE_DIR}'"; fi
if [[ -n "${FW_RUN_DIR}" ]]; then LDFLAGS="${LDFLAGS} -X 'grimm.is/flywall/internal/brand.BuildDefaultRunDir=${FW_RUN_DIR}'"; fi
if [[ -n "${FW_SHARE_DIR}" ]]; then LDFLAGS="${LDFLAGS} -X 'grimm.is/flywall/internal/brand.BuildDefaultShareDir=${FW_SHARE_DIR}'"; fi

# --- Helper Functions ---
log_info() { echo -e "${BLUE}INFO: $1${NC}"; }
log_success() { echo -e "${GREEN}SUCCESS: $1${NC}"; }
log_warn() { echo -e "${YELLOW}WARN: $1${NC}"; }
log_error() { echo -e "${RED}ERROR: $1${NC}"; exit 1; }

# Check if a target needs rebuilding based on source directory
needs_rebuild() {
    local target=$1
    local src_dir=$2
    [[ -f "$target" ]] || return 0 # Rebuild if missing

    # Check if any .go file in src_dir (recursive) or internal/ is newer than target
    # Limit depth for performance, integration tests often don't change core types
    if [[ -n $(find "$src_dir" "${PROJECT_ROOT}/internal" -name "*.go" -newer "$target" 2>/dev/null | head -n 1) ]]; then
        return 0 # Rebuild needed
    fi
    return 1 # Fresh
}

usage() {
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  ${BRAND_NAME} - Project Management Script${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "${YELLOW}Usage:${NC} $0 <subcommand> [options]"
    echo ""
    echo -e "${YELLOW}Subcommands:${NC}"
    echo "  build [target]   Build the project (targets: native, ui, linux, toolbox, all) [--cover]"
    echo "  test [type]      Run tests (types: unit, int, ui, all)"
    echo "                   Utilities: log, history, list, diff"
    echo "  coverage         Merge coverage profiles and generate report"
    echo "  dev [mode]       Start dev environment (modes: web, api, tui, vm)"
    echo "  vm [command]     VM management (commands: setup, start, stop, restart, status)"
    echo "  deploy <host>    Deploy to a remote host"
    echo "  docs [format]    Generate documentation (formats: hugo, all, serve)"
    echo "  clean            Remove build artifacts"
    echo "  stats            Calculate code statistics"
    echo "  demo             Run the Orca demo environment"
    echo "  help             Show this help message"
    echo ""
    echo -e "Use '$0 <subcommand> help' for more details."
}

# Wrapper for go build to auto-recover dependencies
run_go_build() {
    if ! go build "$@"; then
        log_warn "Build failed. Attempting 'go mod tidy' and retrying..."
        go mod tidy
        go build "$@"
    fi
}

# --- Subcommand: Build ---
cmd_build() {
    local target="native"
    local cover_flag=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            --cover) cover_flag="-cover -coverpkg=./..." ;;
            *) target=$1 ;;
        esac
        shift
    done

    mkdir -p "${BUILD_DIR}"

    case "${target}" in
        native)
            BINARY_NAME="${BRAND_BINARY}-${HOST_OS}-${LINUX_ARCH}"
            if needs_rebuild "${BUILD_DIR}/${BINARY_NAME}" "."; then
                log_info "Building native binary (${BINARY_NAME})..."
                run_go_build $cover_flag -ldflags "${LDFLAGS}" -o "${BUILD_DIR}/${BINARY_NAME}" .
                ln -sf "${BINARY_NAME}" "${BUILD_DIR}/${BRAND_BINARY}"
                log_success "Native binary built: ${BUILD_DIR}/${BINARY_NAME} (symlinked to ${BRAND_BINARY})"
            else
                # Ensure symlink exists even if binary is fresh
                if [[ ! -L "${BUILD_DIR}/${BRAND_BINARY}" ]] || [[ "$(readlink "${BUILD_DIR}/${BRAND_BINARY}")" != "${BINARY_NAME}" ]]; then
                    ln -sf "${BINARY_NAME}" "${BUILD_DIR}/${BRAND_BINARY}"
                    log_info "Restored symlink: ${BRAND_BINARY} -> ${BINARY_NAME}"
                fi
            fi
            ;;
        ui)
            log_info "Building UI..."
            (
                cd "${UI_DIR}"
                if [[ ! -d node_modules ]]; then npm install --silent; fi
                npm run build --silent
            )
            log_success "UI built"
            ;;
        linux)
            if needs_rebuild "${BUILD_DIR}/${BRAND_BINARY}-linux-${LINUX_ARCH}" "."; then
                log_info "Building Linux binary (${LINUX_ARCH})..."
                CGO_ENABLED=0 GOOS=linux GOARCH="${LINUX_ARCH}" run_go_build $cover_flag -ldflags "${LDFLAGS}" -o "${BUILD_DIR}/${BRAND_BINARY}-linux-${LINUX_ARCH}" .
                log_success "Linux binary built: ${BUILD_DIR}/${BRAND_BINARY}-linux-${LINUX_ARCH}"
            fi
            ;;
        builder)
            if needs_rebuild "${BUILD_DIR}/flywall-builder" "./tools/cmd/flywall-builder"; then
                log_info "Building flywall-builder..."
                run_go_build -o "${BUILD_DIR}/flywall-builder" ./tools/cmd/flywall-builder
            fi
            ;;
        all)
            cmd_build ui
            cmd_build linux $([[ -n "$cover_flag" ]] && echo "--cover")
            cmd_build native $([[ -n "$cover_flag" ]] && echo "--cover")
            cmd_build toolbox
            cmd_build tests
            ;;
        toolbox)
            # Host Toolbox
            TOOLBOX_HOST_NAME="toolbox-${HOST_OS}-${LINUX_ARCH}"
            if needs_rebuild "${BUILD_DIR}/${TOOLBOX_HOST_NAME}" "./tools/cmd/toolbox"; then
                log_info "Building Toolbox (Host: ${TOOLBOX_HOST_NAME})..."
                run_go_build -ldflags "${TOOL_LDFLAGS}" -o "${BUILD_DIR}/${TOOLBOX_HOST_NAME}" ./tools/cmd/toolbox
                ln -sf "${TOOLBOX_HOST_NAME}" "${BUILD_DIR}/toolbox"
                log_success "Toolbox (Host) built: ${BUILD_DIR}/${TOOLBOX_HOST_NAME}"
            else
                 # Ensure symlink
                if [[ ! -L "${BUILD_DIR}/toolbox" ]] || [[ "$(readlink "${BUILD_DIR}/toolbox")" != "${TOOLBOX_HOST_NAME}" ]]; then
                    ln -sf "${TOOLBOX_HOST_NAME}" "${BUILD_DIR}/toolbox"
                fi
            fi

            # Guest Toolbox (Static Linux)
            TOOLBOX_GUEST_NAME="toolbox-linux-${LINUX_ARCH}"
            if needs_rebuild "${BUILD_DIR}/${TOOLBOX_GUEST_NAME}" "./tools/cmd/toolbox"; then
                log_info "Building Toolbox (Guest: ${TOOLBOX_GUEST_NAME})..."
                CGO_ENABLED=0 GOOS=linux GOARCH="${LINUX_ARCH}" run_go_build -ldflags "${TOOL_LDFLAGS}" -o "${BUILD_DIR}/${TOOLBOX_GUEST_NAME}" ./tools/cmd/toolbox
                ln -sf "${TOOLBOX_GUEST_NAME}" "${BUILD_DIR}/toolbox-linux"
                log_success "Toolbox (Guest) built: ${BUILD_DIR}/${TOOLBOX_GUEST_NAME}"
            else
                 # Ensure symlink
                if [[ ! -L "${BUILD_DIR}/toolbox-linux" ]] || [[ "$(readlink "${BUILD_DIR}/toolbox-linux")" != "${TOOLBOX_GUEST_NAME}" ]]; then
                    ln -sf "${TOOLBOX_GUEST_NAME}" "${BUILD_DIR}/toolbox-linux"
                fi
            fi
            ;;
        tests)
            local test_sentinel="${BUILD_DIR}/tests/linux/${LINUX_ARCH}/.test_sentinel"
            if needs_rebuild "$test_sentinel" "internal"; then
                log_info "Building unit test binaries for VM (linux/${LINUX_ARCH})..."
                bash "${SCRIPTS_DIR}/build/build_tests.sh" linux "${LINUX_ARCH}"
                log_success "Unit test binaries built"
            fi
            ;;

        *)
            log_error "Unknown build target: ${target}"
            ;;
    esac
}

# --- Subcommand: Test ---
cmd_test() {
    local type="unit"
    local args=""
    local cover=false
    local vm_mode=false
    local failed_runs=0
    local check_modified=0
    local pool=""

    local passthrough_args=""

    if [[ $# -gt 0 ]]; then
        type=$1
        shift
    fi

    # Utility subcommands handle their own args — skip the main arg parser
    case "$type" in
        log|history|list|diff) ;; # Fall through to the main case with $@ intact
        *)

    # Parse remaining arguments
    while [[ $# -gt 0 ]]; do
        if [[ "$1" == "--" ]]; then
            shift
            # Capture everything else as passthrough args
            passthrough_args="$*"
            break
        fi

        case $1 in
            --cover) cover=true ;;
            --vm) vm_mode=true ;;
            --filter) args="$args -filter $2"; shift ;;
            --verbose|-v) args="$args -v -j 1" ;;
            --failed) failed_runs=1 ;;
            --failed=*) failed_runs="${1#*=}" ;;
            --changed|--modified) check_modified=1 ;;
            -j*) args="$args $1" ;; # Pass through -j explicitly if needed, but handled by default
            --pool) pool="$2"; shift ;;
            --pool=*) pool="${1#*=}" ;;
            *) args="$args $1" ;;
        esac
        shift
    done

    # Handle --failed logic
    if [[ "$failed_runs" -gt 0 ]]; then
        if [[ "$failed_runs" -eq 1 ]]; then
            log_info "Searching for tests whose most recent run was not successful..."
        else
            log_info "Searching for tests with failures in their last $failed_runs runs..."
        fi
        local history_file="build/test-history.json"
        local failed_tests=""

        if [[ ! -f "$history_file" ]]; then
            log_warn "Test history file ($history_file) not found. Cannot determine failed tests."
        elif ! command -v jq >/dev/null 2>&1; then
             log_warn "jq not found. Cannot parse test history."
        else
            # Extract tests that have failed in their last N runs
            local candidates=$(jq -r --argjson n "$failed_runs" '
                .tests | to_entries[] |
                select(
                    .value.executions | sort_by(.timestamp) | reverse | .[:$n] |
                    any(.status != "pass")
                ) | .key
            ' "$history_file")

            for test_path in $candidates; do
                if [[ "$test_path" != *.sh ]] && [[ "$test_path" != *"*"* ]]; then
                    continue
                fi

                if [[ -e "$test_path" ]]; then
                    failed_tests="$failed_tests $test_path"
                elif [[ "$test_path" == *"*"* ]]; then
                     failed_tests="$failed_tests $test_path"
                elif [[ -f "integration_tests/linux/$test_path" ]]; then
                     failed_tests="$failed_tests integration_tests/linux/$test_path"
                else
                     :
                fi
            done
        fi

        if [[ -z "$failed_tests" ]]; then
            log_success "No failed tests found!"
            return 0
        else
            failed_tests=$(echo "$failed_tests" | tr ' ' '\n' | sort -u | tr '\n' ' ')
            log_info "Found failed tests: $failed_tests"
            args="$args $failed_tests"
        fi
    fi

    # Handle --changed/--modified logic
    if [[ "$check_modified" -gt 0 ]]; then
        log_info "Searching for tests modified since their last passing run..."
        local history_file="build/test-history.json"
        local modified_tests=""

        if [[ ! -f "$history_file" ]]; then
            log_warn "Test history file ($history_file) not found. Cannot determine modified tests."
        elif ! command -v jq >/dev/null 2>&1; then
             log_warn "jq not found. Cannot parse test history."
        else
            local candidates=$(jq -r '
                .tests | to_entries[] |
                select(
                    .value.executions | length > 0
                ) |
                .key as $test |
                (.value.executions | sort_by(.timestamp) | reverse | .[0] | .timestamp) as $last_run |
                (.value.executions | map(select(.status == "pass")) | sort_by(.timestamp) | reverse | .[0] | .timestamp) as $last_pass |
                if $last_pass != null then
                    $test + "|" + $last_pass
                else
                    empty
                end
            ' "$history_file")

            for test_info in $candidates; do
                test_path="${test_info%|*}"
                last_pass_time="${test_info#*|}"
                pass_timestamp=$(date -d "$last_pass_time" +%s 2>/dev/null || echo 0)

                if [[ -f "$test_path" ]]; then
                    file_timestamp=$(stat -c %Y "$test_path" 2>/dev/null || echo 0)
                    if [[ "$file_timestamp" -gt "$pass_timestamp" ]]; then
                        modified_tests="$modified_tests $test_path"
                    fi
                elif [[ -f "integration_tests/linux/$test_path" ]]; then
                    file_timestamp=$(stat -c %Y "integration_tests/linux/$test_path" 2>/dev/null || echo 0)
                    if [[ "$file_timestamp" -gt "$pass_timestamp" ]]; then
                        modified_tests="$modified_tests integration_tests/linux/$test_path"
                    fi
                fi
            done
        fi

        if [[ -z "$modified_tests" ]]; then
            log_success "No modified tests found (all tests are up to date)!"
            return 0
        else
            modified_tests=$(echo "$modified_tests" | tr ' ' '\n' | sort -u | tr '\n' ' ')
            log_info "Found modified tests: $modified_tests"
            args="$args $modified_tests"
        fi
    fi

    # Helper function to find packages containing a test
    find_packages_for_test() {
        local filter="$1"
        local matching_files=""
        
        if command -v rg &>/dev/null; then
            matching_files=$(rg -l -g '**/*_test.go' "$filter" 2>/dev/null || true)
        elif command -v grep &>/dev/null; then
            matching_files=$(grep -rl --include='*_test.go' "$filter" . 2>/dev/null || true)
        else
            log_error "Neither rg nor grep found. Cannot perform smart filtering."
            return 1
        fi
        
        if [[ -z "$matching_files" ]]; then
            return 1
        fi
        
        local pkg_dirs=$(echo "$matching_files" | xargs dirname | sort -u)
        local import_paths=""
        
        for dir in $pkg_dirs; do
            if import_path=$(cd "$dir" && go list . 2>/dev/null); then
                import_paths="$import_paths $import_path"
            fi
        done
        
        echo "$import_paths" | xargs
    }

    ;; esac  # end utility-bypass

    case "${type}" in
        unit)
            if [[ "$vm_mode" == true ]]; then
                log_info "Running unit tests in VM (Cross-Compiled)..."
                
                cmd_build toolbox
                cmd_vm ensure
                
                local filter_value=""
                local target_packages=""
                
                if [[ "$args" =~ -filter[[:space:]]+([^[:space:]]+) ]]; then
                    filter_value="${BASH_REMATCH[1]}"
                fi
                
                if [[ -n "$filter_value" ]]; then
                    log_info "Searching for tests matching: $filter_value"
                    if target_packages=$(find_packages_for_test "$filter_value"); then
                        log_info "Found matching packages: $target_packages"
                    else
                        log_error "No tests found matching filter: $filter_value"
                        return 1
                    fi
                fi
                
                mkdir -p "build/tests/linux/${LINUX_ARCH}"
                if [[ -n "$target_packages" ]]; then
                    log_info "Building tests for specific packages..."
                    rm -f "build/tests/linux/${LINUX_ARCH}"/*.test
                    bash "${SCRIPTS_DIR}/build/build_tests.sh" linux "${LINUX_ARCH}" $target_packages
                else
                    log_info "Compiling unit tests for linux/${LINUX_ARCH}..."
                    bash "${SCRIPTS_DIR}/build/build_tests.sh" linux "${LINUX_ARCH}"
                fi
                
                local pool_args=""
                if [[ -n "$pool" ]]; then pool_args="--pool $pool"; fi
                
                if [[ -n "$passthrough_args" ]]; then
                    ./build/toolbox orca unit-test $pool_args $args --bin-dir "build/tests/linux/${LINUX_ARCH}" -- $passthrough_args
                else
                     ./build/toolbox orca unit-test $pool_args $args --bin-dir "build/tests/linux/${LINUX_ARCH}"
                fi
            else
                local cover_flags=""
                if [[ "$cover" == true ]]; then
                    cover_flags="-coverprofile=coverage.out -coverpkg=./..."
                fi
                
                local filter_value=""
                local target_packages=""
                
                if [[ "$args" =~ -filter[[:space:]]+([^[:space:]]+) ]]; then
                    filter_value="${BASH_REMATCH[1]}"
                fi
                
                if [[ -n "$filter_value" ]]; then
                    log_info "Searching for tests matching: $filter_value"
                    if target_packages=$(find_packages_for_test "$filter_value"); then
                        log_info "Running tests in: $target_packages"
                        go test -v $cover_flags -run "$filter_value" $target_packages
                    else
                        log_error "No tests found matching filter: $filter_value"
                        return 1
                    fi
                else
                    log_info "Running unit tests $([[ "$cover" == true ]] && echo "with coverage")..."
                    go test -v $cover_flags ./internal/... | grep -E '^(ok|FAIL|---|===|PASS)' || true
                fi
                
                if [[ "$cover" == true ]]; then
                    go tool cover -func=coverage.out
                fi
            fi
            ;;
        int)
            log_info "Running integration tests..."
            local vm_ensure_pid=""
            if [[ "$vm_mode" == false ]]; then
                # Even for integration tests, we need VMs ensured
                log_info "Ensuring VMs in parallel..."
                # Run ensure in subshell to capture logs/errors if needed, but for now just background it
                (cmd_vm ensure) &
                vm_ensure_pid=$!
            fi

            local build_args=""
            if [[ "$cover" == true ]]; then
                build_args="--cover"
                log_info "Building with coverage instrumentation..."
            fi

            cmd_build linux $build_args
            cmd_build toolbox

            # Wait for VM ensure to complete
            if [[ -n "$vm_ensure_pid" ]]; then
                log_info "Waiting for VM setup to complete..."
                wait "$vm_ensure_pid" || log_error "VM setup failed"
            else
                cmd_vm ensure
            fi

            mkdir -p build/test-artifacts
            cp "${BUILD_DIR}/${BRAND_BINARY}-linux-${LINUX_ARCH}" build/test-artifacts/flywall-v1
            cp "${BUILD_DIR}/${BRAND_BINARY}-linux-${LINUX_ARCH}" build/test-artifacts/flywall-v2

            log_info "Executing parallel orca tests..."

            if [[ "$cover" == true ]]; then
                args="$args --cover"
            fi

            if [[ -n "$pool" ]]; then
                args="$args --pool $pool"
            fi

            if [[ -z "$args" ]] && [[ ! -t 0 ]]; then
                while read -r line; do
                    args="$args $line"
                done < /dev/stdin
            fi

            local resolved_args=""
            local int_test_root="integration_tests/linux"

            local skip_next=false
            for arg in $args; do
                if [[ "$skip_next" == "true" ]]; then
                     resolved_args="$resolved_args $arg"
                     skip_next=false
                     continue
                fi

                if [[ "$arg" == -* ]]; then
                    resolved_args="$resolved_args $arg"
                    case "$arg" in
                        -j|-filter|-streak-max|--target) skip_next=true ;;
                    esac
                    continue
                fi

                if [[ "$arg" == "unit_tests" ]]; then
                    resolved_args="$resolved_args ${int_test_root}/05-golang/unit_test.sh"
                    continue
                fi

                if [[ -e "$arg" ]]; then
                    resolved_args="$resolved_args $arg"
                    continue
                fi

                if [[ -e "${int_test_root}/${arg}" ]]; then
                    resolved_args="$resolved_args ${int_test_root}/${arg}"
                    continue
                fi

                local matches=$(find "${int_test_root}" -type f -name "*${arg}*.sh" 2>/dev/null)

                if [[ -n "$matches" ]]; then
                    log_info "Resolved '$arg' to:"
                    echo "$matches" | while read -r match; do
                         echo "  - $match"
                    done
                    resolved_args="$resolved_args $matches"
                    continue
                fi

                if [[ "$arg" == *"*"* ]]; then
                     if ls $arg >/dev/null 2>&1; then
                         resolved_args="$resolved_args $(ls -d $arg)"
                         continue
                     fi
                fi

                log_warn "Could not resolve '$arg' to a test file. Passing as-is."
                resolved_args="$resolved_args $arg"
            done

            if [[ -z "$resolved_args" ]]; then
                resolved_args="${int_test_root}"
            fi

            if [[ "$resolved_args" == "${int_test_root}" ]] || \
               [[ "$resolved_args" == *"05-golang/unit_test.sh"* ]]; then
                cmd_build tests
            fi

            local unit_test_scripts=""
            local integration_test_scripts=""
            for script in $resolved_args; do
                if [[ "$script" == *"05-golang/unit_test.sh" ]]; then
                    unit_test_scripts="$unit_test_scripts $script"
                else
                    integration_test_scripts="$integration_test_scripts $script"
                fi
            done

            if [[ -n "$unit_test_scripts" ]]; then
                log_info "--- Phase 1: VM Unit Tests ---"
                ./build/toolbox orca test $unit_test_scripts
            fi

            if [[ -n "$integration_test_scripts" ]]; then
                if [[ -n "$unit_test_scripts" ]]; then
                    log_info "--- Phase 2: Integration Tests ---"
                else
                    log_info "Executing parallel orca tests..."
                fi
                ./build/toolbox orca test $integration_test_scripts
            fi
            ;;
        ui)
            log_info "Running UI tests..."
            "${SCRIPTS_DIR}/run-ui-tests.sh"
            ;;
        all)
            cmd_test unit
            cmd_test ui
            ;;
        log)
            # fw test log <name> [N] [--fail]
            # Dump the Nth most recent log for a test (default: most recent)
            local test_name="${1:-}"
            shift 2>/dev/null || true
            local nth=1
            local only_fail=false

            while [[ $# -gt 0 ]]; do
                case "$1" in
                    --fail|--failed) only_fail=true ;;
                    [0-9]*) nth="$1" ;;
                esac
                shift
            done

            if [[ -z "$test_name" ]]; then
                log_error "Usage: $0 test log <name> [N] [--fail]"
                log_info "  Dump the Nth most recent log (default: latest)"
                log_info "  --fail    Show the most recent failing log"
                return 1
            fi

            # Resolve fuzzy test name to a result directory
            local int_test_root="integration_tests/linux"
            local result_root="build/test-results"
            local result_dir=""

            # Try exact match first
            if [[ -d "${result_root}/${test_name}" ]]; then
                result_dir="${result_root}/${test_name}"
            elif [[ -d "${result_root}/${int_test_root}/${test_name}" ]]; then
                result_dir="${result_root}/${int_test_root}/${test_name}"
            else
                # Fuzzy match
                local matches=$(find "${result_root}" -type d -name "*${test_name}*" 2>/dev/null)
                local match_count=$(echo "$matches" | grep -c . 2>/dev/null || echo 0)

                if [[ "$match_count" -eq 0 ]] || [[ -z "$matches" ]]; then
                    log_error "No test results found matching '$test_name'"
                    log_info "Try: $0 test list"
                    return 1
                elif [[ "$match_count" -gt 1 ]]; then
                    log_warn "Multiple matches for '$test_name':"
                    echo "$matches" | sed "s|${result_root}/||" | while read -r m; do echo "  - $m"; done
                    log_info "Be more specific."
                    return 1
                fi
                result_dir="$matches"
            fi

            if [[ "$only_fail" == true ]]; then
                # Use test-history.json to find the most recent failing log
                local history_file="build/test-history.json"
                if [[ -f "$history_file" ]] && command -v jq >/dev/null 2>&1; then
                    local test_key=$(echo "$result_dir" | sed "s|${result_root}/||")
                    local fail_log=$(jq -r --arg key "$test_key" '
                        .tests[$key].executions // [] |
                        sort_by(.timestamp) | reverse |
                        map(select(.status != "pass")) |
                        .[0].log_path // empty
                    ' "$history_file")

                    if [[ -n "$fail_log" ]]; then
                        local full_path="build/${fail_log}"
                        if [[ -f "$full_path" ]]; then
                            log_info "Last failing log: $full_path"
                            echo ""
                            cat "$full_path"
                            return 0
                        fi
                    fi
                fi
                log_warn "No failing logs found for this test."
                return 1
            fi

            # Get the Nth most recent log file
            local log_file=$(ls -t "$result_dir"/*.log 2>/dev/null | sed -n "${nth}p")

            if [[ -z "$log_file" ]]; then
                log_error "No log files found in $result_dir"
                return 1
            fi

            log_info "Log: $log_file"
            echo ""
            cat "$log_file"
            ;;
        history)
            # fw test history <name>
            # Show pass/fail timeline from test-history.json
            local test_name="${1:-}"

            if [[ -z "$test_name" ]]; then
                log_error "Usage: $0 test history <name>"
                return 1
            fi

            local history_file="build/test-history.json"
            if [[ ! -f "$history_file" ]]; then
                log_error "Test history file not found: $history_file"
                return 1
            fi

            if ! command -v jq >/dev/null 2>&1; then
                log_error "jq is required for history lookups"
                return 1
            fi

            # Find matching test key
            local matching_keys=$(jq -r --arg pat "$test_name" '
                .tests | keys[] | select(test($pat))
            ' "$history_file")

            if [[ -z "$matching_keys" ]]; then
                log_error "No history found matching '$test_name'"
                return 1
            fi

            local key_count=$(echo "$matching_keys" | wc -l | tr -d ' ')
            if [[ "$key_count" -gt 1 ]]; then
                log_warn "Multiple matches:"
                echo "$matching_keys" | while read -r k; do echo "  - $k"; done
                log_info "Be more specific."
                return 1
            fi

            local test_key="$matching_keys"
            log_info "History for: $test_key"
            echo ""
            printf "  %-24s  %-6s  %s\n" "TIMESTAMP" "STATUS" "DURATION"
            printf "  %-24s  %-6s  %s\n" "────────────────────────" "──────" "────────"

            jq -r --arg key "$test_key" '
                .tests[$key].executions |
                sort_by(.timestamp) | reverse |
                .[] |
                "\(.timestamp | split("T") | .[0] + " " + (.[1] | split(".")[0]))  \(if .status == "pass" then "✅" else "❌" end) \(.status | ascii_upcase)  \((.duration / 1000000000 * 100 | round / 100 | tostring) + "s")"
            ' "$history_file" | while IFS= read -r line; do
                printf "  %s\n" "$line"
            done
            ;;
        list)
            # fw test list - list all integration tests grouped by category
            local int_test_root="integration_tests/linux"
            local current_group=""

            log_info "Integration tests:"
            echo ""

            find "$int_test_root" -name "*_test.sh" -type f 2>/dev/null | sort | while read -r test_file; do
                local group=$(dirname "$test_file" | sed "s|${int_test_root}/||")
                local name=$(basename "$test_file" .sh | sed 's/_test$//')

                if [[ "$group" != "$current_group" ]]; then
                    current_group="$group"
                    echo -e "  ${YELLOW}${group}/${NC}"
                fi
                echo "    $name"
            done
            ;;
        diff)
            # fw test diff <name>
            # Show what changed between the last passing run and the first subsequent failure
            local test_name="${1:-}"

            if [[ -z "$test_name" ]]; then
                log_error "Usage: $0 test diff <name>"
                log_info "  Shows git changes since the last passing run of a test"
                return 1
            fi

            local history_file="build/test-history.json"
            if [[ ! -f "$history_file" ]]; then
                log_error "Test history file not found: $history_file"
                return 1
            fi

            if ! command -v jq >/dev/null 2>&1; then
                log_error "jq is required for diff lookups"
                return 1
            fi

            # Find matching test key
            local matching_keys=$(jq -r --arg pat "$test_name" '
                .tests | keys[] | select(test($pat))
            ' "$history_file")

            if [[ -z "$matching_keys" ]]; then
                log_error "No history found matching '$test_name'"
                return 1
            fi

            local key_count=$(echo "$matching_keys" | wc -l | tr -d ' ')
            if [[ "$key_count" -gt 1 ]]; then
                log_warn "Multiple matches:"
                echo "$matching_keys" | while read -r k; do echo "  - $k"; done
                log_info "Be more specific."
                return 1
            fi

            local test_key="$matching_keys"

            # Find the last passing run's timestamp
            local last_pass_ts=$(jq -r --arg key "$test_key" '
                .tests[$key].executions |
                sort_by(.timestamp) | reverse |
                map(select(.status == "pass")) |
                .[0].timestamp // empty
            ' "$history_file")

            if [[ -z "$last_pass_ts" ]]; then
                log_warn "No passing runs found for '$test_key'. Showing full diff against HEAD."
                git diff HEAD --stat
                return 0
            fi

            # Find the git commit closest to that timestamp
            local pass_commit=$(git log --until="$last_pass_ts" --format="%H" -1 2>/dev/null)

            if [[ -z "$pass_commit" ]]; then
                log_warn "Could not find git commit at timestamp $last_pass_ts"
                log_info "Showing uncommitted changes instead:"
                git diff --stat
                return 0
            fi

            local short_commit=$(git rev-parse --short "$pass_commit")
            local commit_msg=$(git log --format="%s" -1 "$pass_commit")

            log_info "Last pass: $(echo "$last_pass_ts" | cut -dT -f1,2 | tr T ' ')"
            log_info "Commit at that time: ${short_commit} (${commit_msg})"
            echo ""
            echo -e "${YELLOW}Changes since last passing run:${NC}"
            git diff "$pass_commit" --stat
            echo ""
            echo -e "${YELLOW}Files changed:${NC}"
            git diff "$pass_commit" --name-only | head -30
            echo ""
            log_info "Full diff: git diff ${short_commit}"
            ;;
        *)
            log_error "Unknown test type: ${type}"
            ;;
    esac
}

# --- Subcommand: Coverage ---
cmd_coverage() {
    log_info "Merging coverage profiles..."
    mkdir -p "${BUILD_DIR}/coverage"

    # Check if we have coverage data
    if [[ ! -d "${BUILD_DIR}/coverage" ]] || [[ -z $(ls -A "${BUILD_DIR}/coverage" 2>/dev/null) ]]; then
        log_error "No coverage data found in ${BUILD_DIR}/coverage. Run tests with --cover first."
    fi

    # Merge binary coverage data
    # Go 1.20+ uses 'go tool covdata' for binary coverage
    log_info "Generating reports from ${BUILD_DIR}/coverage..."

    rm -rf "${BUILD_DIR}/coverage-merged"
    mkdir -p "${BUILD_DIR}/coverage-merged"
    go tool covdata merge -i "${BUILD_DIR}/coverage" -o "${BUILD_DIR}/coverage-merged"
    go tool covdata textfmt -i "${BUILD_DIR}/coverage-merged" -o "${BUILD_DIR}/coverage.out"

    log_info "Generating HTML report..."
    go tool cover -html="${BUILD_DIR}/coverage.out" -o "${BUILD_DIR}/coverage.html"

    log_success "Coverage report generated: ${BUILD_DIR}/coverage.html"
}

# --- Subcommand: Clean ---
cmd_clean() {
    log_info "Cleaning build artifacts..."
    rm -rf "${BUILD_DIR}"
    rm -f coverage.out
    find . -name "*.test" -delete
    find "${UI_DIR}/dist" -mindepth 1 ! -name 'index.html' -delete 2>/dev/null || true
    log_success "Clean complete"
}

# --- Subcommand: VM ---
cmd_vm() {
    local command=${1:-status}
    case "${command}" in
        setup)
            log_info "Building Alpine VM image..."
            cmd_build builder
            "${BUILD_DIR}/flywall-builder" build
            log_success "VM setup complete"
            ;;
        start)
            cmd_vm ensure
            cmd_build linux
            log_info "Starting VM..."
            "${SCRIPTS_DIR}/vm/dev.sh" &
            log_success "VM started in background"
            ;;
        stop)
            log_info "Stopping VM..."
            if pgrep -f "qemu.*rootfs" >/dev/null 2>&1; then
                pkill -f "qemu.*rootfs" 2>/dev/null
                log_success "VM stopped"
            else
                log_warn "No VM is currently running"
            fi
            ;;
        restart)
            log_info "Restarting VM..."
            cmd_vm stop
            sleep 1
            cmd_vm start
            ;;
        ensure)
            if [[ ! -f "${BUILD_DIR}/rootfs.qcow2" ]] || [[ ! -f "${BUILD_DIR}/vmlinuz" ]] || [[ ! -f "${BUILD_DIR}/initramfs" ]]; then
                log_warn "One or more VM assets missing, running setup..."
                cmd_vm setup
            fi
            ;;
        status|pool-status)
            log_info "Checking Orca VM Pool status..."
            cmd_build toolbox
            ./build/toolbox orca status
            ;;
        *)
            log_error "Unknown VM command: ${command}"
            ;;
    esac
}

# --- Subcommand: Exec ---
cmd_exec() {
    cmd_vm ensure
    cmd_build linux
    log_info "Executing command in VM..."
    "${BUILD_DIR}/flywall-builder" exec "$@"
}

# --- Subcommand: Dev ---
cmd_dev() {
    local mode=${1:-full}
    case "${mode}" in
        web)
            log_info "Starting Web UI demo..."
            cd "${UI_DIR}" && npm run dev
            ;;
        api)
            log_info "Starting mock API server..."
            go run ./cmd/api-dev
            ;;
        tui)
            log_info "Starting TUI demo..."
            go run ./cmd/tuidemo
            ;;
        full|vm)
            log_info "Starting full development environment..."
            cmd_build linux
            cmd_vm ensure
            "${SCRIPTS_DIR}/dev/run.sh"
            ;;
        *)
            log_error "Unknown dev mode: ${mode}"
            ;;
    esac
}

# --- Subcommand: Deploy ---
cmd_deploy() {
    local host=$1
    if [[ -z "$host" ]]; then
        log_error "Usage: $0 deploy <user@host>"
    fi

    log_info "Building Linux binary for deployment..."
    cmd_build linux

    local binary="${BUILD_DIR}/${BRAND_BINARY}-linux-${LINUX_ARCH}"
    log_info "Deploying to ${host}..."
    "${SCRIPTS_DIR}/deploy/remote.sh" "${host}" "${binary}"
}

# --- Subcommand: Stats ---
cmd_stats() {
    log_info "Calculating code statistics..."
    "${SCRIPTS_DIR}/dev/stats.sh"
}

# --- Subcommand: Demo ---
cmd_demo() {
    local arg=$1

    if [[ "$arg" == "upgrade" ]]; then
        log_info "Forcing full upgrade of Demo environment (UI + Linux Binary)..."
        cmd_build ui
        # Force rebuild of linux binary (which watcher will pick up if running, or VM will use on start)
        # Actually, for 'fw demo', we just need to make sure the assets are fresh.
        # But if the user wants to trigger the *hot upgrade*, they might want to just build the binary.
        # Let's assume 'upgrade' means "Make sure everything is fresh before starting or for the watcher".

        # If we just want to trigger the hot upgrade, we just need to build linux.
        # If we want to refresh UI, we build UI.
        # Let's do both to be safe.
        cmd_build linux
        shift # consume 'upgrade'
    else
        # Standard check
        if [[ ! -d "${UI_DIR}/dist" ]]; then
             log_info "UI build missing, building..."
             cmd_build ui
        fi
    fi

    # log_info "Starting Orca Demo Environment..."
    cmd_build toolbox
    "${BUILD_DIR}/toolbox" orca demo "$@"
}

# --- Subcommand: Orca (Direct Access) ---
cmd_orca() {
    # Ensure toolbox is built
    if [[ ! -f "${BUILD_DIR}/toolbox" ]]; then
        cmd_build toolbox
    fi
    "${BUILD_DIR}/toolbox" orca "$@"
}

# --- Subcommand: Test Demo ---
cmd_test_demo() {
    log_info "Preparing Integrated Demo Test Run..."

    # 1. Build Dependencies (UI + Linux Binary + Toolbox)
    # Force rebuild of UI and Linux binary to ensure assets are fresh
    cmd_build ui
    # Remove existing binary to force rebuild (bypass needs_rebuild for UI changes)
    rm -f "${BUILD_DIR}/${BRAND_BINARY}-linux-${LINUX_ARCH}"
    cmd_build linux

    if [[ ! -f "${BUILD_DIR}/toolbox" ]]; then
        cmd_build toolbox
    fi

    # 2. Find Free Port (simple linear search from 8443)
    local port=8443
    while lsof -i :$port >/dev/null 2>&1; do
        ((port++))
    done

    # Calculate http port (to pass explicitly)
    # Logic: If https=8444, http=8081 (8444-363)
    local http_port=8080
    if [[ $port -ne 8443 ]]; then
      http_port=$((port - 363))
    fi

    log_info "Found free HTTPS port: $port (HTTP: $http_port)"

    # 2.5. Create per-run artifact directory for logs/state
    local run_id=$(date +%Y%m%d-%H%M%S)
    local run_dir="${BUILD_DIR}/test-runs/run-${run_id}"
    mkdir -p "$run_dir"
    log_info "Artifact directory: $run_dir"

    # 3. Start VM in background
    log_info "Starting ephemeral Demo VM..."
    # Log to a temp file
    local log_file="${run_dir}/demo.log"
    "${BUILD_DIR}/toolbox" orca demo --headless --port $port --http-port $http_port --run-dir "$run_dir" > "$log_file" 2>&1 &
    local vm_pid=$!

    # Register cleanup to kill VM on exit (keep log file for debugging)
    trap "log_info 'Stopping Demo VM...'; kill $vm_pid 2>/dev/null; log_info 'Artifacts at: $run_dir'" EXIT

    # 4. Wait for Healthy (curl loop)
    log_info "Waiting for VM to become ready at https://localhost:$port..."
    local retries=60 # 2 minutes max
    local ready=false
    while [[ $retries -gt 0 ]]; do
        if curl -k -s -o /dev/null "https://localhost:$port"; then
            ready=true
            break
        fi
        sleep 2
        ((retries--))
        echo -n "."
    done
    echo ""

    if [[ "$ready" == "false" ]]; then
        log_error "VM failed to start on port $port. Log tail:"
        tail -n 20 "$log_file"
        kill $vm_pid 2>/dev/null
        exit 1
    fi

    log_success "VM Ready! Running Tests..."

    # 5. Run Tests
    # Export DEMO_PORT for Playwright config
    export DEMO_PORT=$port

    # Check if we should just run tests or specific spec
    local args="$@"
    if [[ -z "$args" ]]; then
        cd "${UI_DIR}" && npm run test:e2e:demo
    else
        # Allow passing args like specific test files
         cd "${UI_DIR}" && npm run test:e2e:demo -- $args
    fi
    local test_exit=$?

    if [[ $test_exit -eq 0 ]]; then
        log_success "All Tests Passed!"
    else
        log_error "Tests Failed!"
    fi
    # Trap will handle cleanup
    exit $test_exit
}

# --- Subcommand: Docs ---
cmd_docs() {
    local format=${1:-hugo}
    case "${format}" in
        hugo)
            log_info "Generating Hugo documentation..."
            go run ./cmd/gen-config-docs -format=hugo
            log_success "Hugo docs generated in docs-site/content/docs/configuration/reference/"
            ;;
        all)
            log_info "Generating all documentation formats..."
            go run ./cmd/gen-config-docs -format=all
            go run ./cmd/gen-config-docs -format=hugo
            log_success "All docs generated"
            ;;
        serve)
            log_info "Starting Hugo development server..."
            cd docs-site && hugo server -D
            ;;
        *)
            log_error "Unknown docs format: ${format}"
            ;;
    esac
}

# --- Entry Point ---
if [[ $# -lt 1 ]]; then
    usage
    exit 0
fi

SUBCOMMAND=$1
shift

case "${SUBCOMMAND}" in
    build) cmd_build "$@" ;;
    test)  cmd_test "$@" ;;
    test-int) cmd_test int "$@" ;;
    coverage) cmd_coverage "$@" ;;
    vm)    cmd_vm "$@" ;;
    exec)  cmd_exec "$@" ;;
    dev)   cmd_dev "$@" ;;
    deploy) cmd_deploy "$@" ;;
    docs)  cmd_docs "$@" ;;
    clean) cmd_clean "$@" ;;
    stats) cmd_stats "$@" ;;
    demo)  cmd_demo "$@" ;;
    test-demo) cmd_test_demo "$@" ;;
    orca) cmd_orca "$@" ;;
    help|--help|-h) usage ;;
    *)
        log_error "Unknown subcommand: ${SUBCOMMAND}"
        ;;
esac
