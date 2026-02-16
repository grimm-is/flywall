# eBPF Build Strategy with QEMU Orchestration

## Overview

Leveraging Flywall's existing QEMU-based orchestration system for eBPF cross-compilation and testing. This approach provides consistency with the existing development workflow and testing infrastructure.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   macOS Host    │    │   QEMU Builder  │    │  QEMU Test VMs   │
│                 │    │    (Linux)      │    │   (Integration) │
│ - Go Code       │    │ - eBPF Compile  │    │ - Runtime Tests  │
│ - Editors       │───▶│ - Embed Programs│───▶│ - Feature Tests  │
│ - Git           │    │ - Static Analysis│    │ - Performance   │
│ - Orca CLI      │    │ - Validation    │    │ - Regression    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Integration with Existing Orca Framework

### 1. Build VM Configuration

```yaml
# configs/ebpf-builder.hcl
schema_version = "1.0"

vm {
  name = "ebpf-builder"
  image = "ubuntu-22.04"

  resources {
    cpu = 2
    memory = "2GB"
    disk = "20GB"
  }

  # Mount project directory
  mounts = [
    {
      source = ".."
      destination = "/flywall"
      readonly = false
    }
  ]

  # Provision eBPF build environment
  provision = [
    "apt-get update",
    "apt-get install -y clang llvm linux-headers-generic libelf-dev libbpf-dev make pkg-config",
    "apt-get install -y golang-go",
    "go install github.com/cilium/ebpf/cmd/bpf2go@latest"
  ]

  # Build artifacts directory
  artifacts = [
    {
      source = "/flywall/build/ebpf"
      destination = "build/ebpf"
    }
  ]
}
```

### 2. Orca Integration

```go
// internal/build/ebpf_builder.go
package build

import (
    "fmt"
    "os/exec"
    "path/filepath"

    "github.com/flywall/orca"
)

type EBPFBuilder struct {
    orca *orca.Client
    config *BuilderConfig
}

type BuilderConfig struct {
    VMName      string
    SourceDir   string
    OutputDir   string
    ParallelJobs int
}

func NewEBPFBuilder(orca *orca.Client) *EBPFBuilder {
    return &EBPFBuilder{
        orca: orca,
        config: &BuilderConfig{
            VMName:      "ebpf-builder",
            SourceDir:   "/flywall/internal/ebpf",
            OutputDir:   "/flywall/build/ebpf",
            ParallelJobs: 4,
        },
    }
}

func (b *EBPFBuilder) Build() error {
    // 1. Start or ensure builder VM is running
    vm, err := b.orca.GetVM(b.config.VMName)
    if err != nil {
        return fmt.Errorf("failed to get builder VM: %w", err)
    }

    // 2. Start VM if not running
    if !vm.IsRunning() {
        if err := vm.Start(); err != nil {
            return fmt.Errorf("failed to start builder VM: %w", err)
        }
    }

    // 3. Wait for VM to be ready
    if err := vm.WaitForReady(30); err != nil {
        return fmt.Errorf("builder VM not ready: %w", err)
    }

    // 4. Run build command in VM
    cmd := fmt.Sprintf(
        "cd %s && make -j%d ebpf-native",
        b.config.SourceDir,
        b.config.ParallelJobs,
    )

    output, err := vm.Execute(cmd)
    if err != nil {
        return fmt.Errorf("build failed: %w\nOutput: %s", err, output)
    }

    // 5. Copy artifacts back
    if err := b.copyArtifacts(vm); err != nil {
        return fmt.Errorf("failed to copy artifacts: %w", err)
    }

    fmt.Printf("eBPF build successful:\n%s\n", output)
    return nil
}

func (b *EBPFBuilder) copyArtifacts(vm *orca.VM) error {
    // Create local output directory
    if err := os.MkdirAll("build/ebpf", 0755); err != nil {
        return err
    }

    // Copy all .o files
    files, err := vm.ListFiles(b.config.OutputDir)
    if err != nil {
        return err
    }

    for _, file := range files {
        if filepath.Ext(file) == ".o" {
            if err := vm.CopyFrom(b.config.OutputDir+"/"+file, "build/ebpf/"+file); err != nil {
                return err
            }
        }
    }

    return nil
}
```

### 3. Integration with flywall.sh

```bash
# Add to flywall.sh

ebpf_build() {
    echo "Building eBPF programs..."

    if [ "$(uname)" = "Darwin" ]; then
        echo "Using Orca QEMU for cross-compilation"
        ebpf_build_orca
    else
        echo "Building natively on Linux"
        ebpf_build_native
    fi
}

ebpf_build_orca() {
    # Ensure orca is available
    if ! command -v orca &> /dev/null; then
        echo "Error: orca CLI not found. Please install orca."
        exit 1
    fi

    # Build using Orca VM
    orca run configs/ebpf-builder.hcl --command "ebpf_build_native"

    # Copy artifacts back
    mkdir -p build/ebpf
    orca cp ebpf-builder:/flywall/build/ebpf/*.o build/ebpf/

    # Generate Go embeddings
    go generate ./internal/ebpf/...
}

ebpf_build_native() {
    mkdir -p build/ebpf

    for prog in internal/ebpf/programs/*.c; do
        if [ -f "$prog" ]; then
            prog_name=$(basename "$prog" .c)
            echo "Compiling $prog_name..."

            clang -O2 -target bpf -I internal/ebpf/include \
                -c "$prog" -o "build/ebpf/$prog_name.o" || {
                echo "Failed to compile $prog_name"
                exit 1
            }
        fi
    done

    # Generate Go embeddings
    go generate ./internal/ebpf/...
}

# Development helper - start builder VM
ebpf_dev_shell() {
    if [ "$(uname)" = "Darwin" ]; then
        echo "Starting eBPF development VM..."
        orca run configs/ebpf-builder.hcl --interactive
    else
        echo "Already on Linux, using current shell"
        bash
    fi
}

# Quick rebuild without full VM restart
ebpf_quick_build() {
    if [ "$(uname)" = "Darwin" ]; then
        echo "Quick rebuild in existing VM..."
        orca exec ebpf-builder -- "cd /flywall && ebpf_build_native"
        mkdir -p build/ebpf
        orca cp ebpf-builder:/flywall/build/ebpf/*.o build/ebpf/
    else
        ebpf_build_native
    fi
}

# Add to main command handler
case "$1" in
    # ... existing commands ...
    "ebpf")
        ebpf_build
        ;;
    "ebpf-dev")
        ebpf_dev_shell
        ;;
    "ebpf-quick")
        ebpf_quick_build
        ;;
esac
```

### 4. Enhanced flywall.sh Commands

```bash
# Add these functions to flywall.sh

# Test eBPF functionality
ebpf_test() {
    echo "Running eBPF tests..."

    if [ "$(uname)" = "Darwin" ]; then
        # Run tests in Orca VM
        orca run configs/ebpf-test.hcl --command "ebpf_test_native"
    else
        ebpf_test_native
    fi
}

ebpf_test_native() {
    # Unit tests
    echo "Running unit tests..."
    go test ./internal/ebpf/...

    # Integration tests if available
    if [ -d "integration_tests/linux/ebpf" ]; then
        echo "Running integration tests..."
        ./flywall.sh test int ebpf
    fi
}

# Clean eBPF build artifacts
ebpf_clean() {
    echo "Cleaning eBPF build artifacts..."
    rm -rf build/ebpf

    if [ "$(uname)" = "Darwin" ]; then
        # Clean in builder VM too
        orca exec ebpf-builder -- "rm -rf /flywall/build/ebpf" 2>/dev/null || true
    fi
}

# Install eBPF programs (for development)
ebpf_install() {
    echo "Installing eBPF programs..."

    if [ "$(uname)" = "Darwin" ]; then
        echo "Cannot install eBPF programs on macOS"
        echo "Use a Linux VM or target system"
        exit 1
    fi

    # Load programs into kernel
    for obj in build/ebpf/*.o; do
        if [ -f "$obj" ]; then
            prog_name=$(basename "$obj" .o)
            echo "Loading $prog_name..."

            # Use bpftool to load if available
            if command -v bpftool &> /dev/null; then
                bpftool prog load "$obj" /sys/fs/bpf/$prog_name
            else
                echo "Warning: bpftool not available, skipping load"
            fi
        fi
    done
}

# Status of eBPF programs
ebpf_status() {
    echo "eBPF Program Status:"

    if [ "$(uname)" = "Darwin" ]; then
        echo "Checking builder VM status..."
        orca status ebpf-builder
    else
        if command -v bpftool &> /dev/null; then
            bpftool prog show
        else
            echo "bpftool not available"
        fi
    fi
}
```

## Development Workflow

### 1. Initial Setup

```bash
# 1. Create builder VM configuration
cat > configs/ebpf-builder.hcl <<'EOF'
schema_version = "1.0"

vm {
  name = "ebpf-builder"
  image = "ubuntu-22.04"

  resources {
    cpu = 2
    memory = "2GB"
  }

  mounts = [
    {
      source = ".."
      destination = "/flywall"
    }
  ]

  provision = [
    "apt-get update",
    "apt-get install -y clang llvm linux-headers-generic libelf-dev libbpf-dev make golang-go"
  ]
}
EOF

# 2. Add eBPF functions to flywall.sh
# (Copy the functions from the document)

# 3. Start builder VM
orca start configs/ebpf-builder.hcl

# 4. First build
./flywall.sh ebpf
```

### 2. Daily Development

```bash
# Edit eBPF program
vim internal/ebpf/programs/xdp_blocklist.c

# Quick rebuild (reuses running VM)
./flywall.sh ebpf-quick

# Run tests
./flywall.sh ebpf-test

# Need to debug? Open shell in builder
./flywall.sh ebpf-dev

# Check status
./flywall.sh ebpf-status

# Clean build
./flywall.sh ebpf-clean
```

### 3. Integration with Existing Commands

```bash
# Your existing test command now supports eBPF
./flywall.sh test int ebpf

# Build includes eBPF automatically
./flywall.sh build

# Status shows eBPF programs too
./flywall.sh status
```

### 4. Working with Multiple VMs

```yaml
# configs/ebpf-test-matrix.hcl
schema_version = "1.0"

# Define test matrix
test_matrix = [
  {
    name = "ubuntu-20.04"
    image = "ubuntu-20.04"
    kernel_version = "5.4"
  },
  {
    name = "ubuntu-22.04"
    image = "ubuntu-22.04"
    kernel_version = "5.15"
  },
  {
    name = "debian-11"
    image = "debian-11"
    kernel_version = "5.10"
  }
]

# Test configuration
test {
  parallel = true
  timeout = "10m"

  # Run eBPF compatibility tests
  commands = [
    "cd /flywall",
    "make test-ebpf-compatibility",
    "make test-integration"
  ]
}
```

## Advanced Features

### 1. Parallel Compilation

```go
// internal/build/parallel_builder.go
func (b *EBPFBuilder) BuildParallel() error {
    // Get list of eBPF programs
    programs, err := b.listPrograms()
    if err != nil {
        return err
    }

    // Create worker pool
    workers := b.config.ParallelJobs
    jobs := make(chan string, len(programs))
    results := make(chan error, len(programs))

    // Start workers
    for w := 0; w < workers; w++ {
        go b.compileWorker(jobs, results)
    }

    // Send jobs
    for _, prog := range programs {
        jobs <- prog
    }
    close(jobs)

    // Collect results
    for range programs {
        if err := <-results; err != nil {
            return err
        }
    }

    return nil
}

func (b *EBPFBuilder) compileWorker(jobs <-chan string, results chan<- error) {
    vm, _ := b.orca.GetVM(b.config.VMName)

    for prog := range jobs {
        cmd := fmt.Sprintf(
            "cd %s && clang -O2 -target bpf -c %s -o %s",
            b.config.SourceDir,
            prog,
            strings.Replace(prog, ".c", ".o", 1),
        )

        _, err := vm.Execute(cmd)
        results <- err
    }
}
```

### 2. Incremental Builds

```makefile
# Track dependencies
EBPF_DEPS_DIR = build/ebpf/deps

build/ebpf/%.o: internal/ebpf/programs/%.c
	@mkdir -p $(EBPF_DEPS_DIR)
	@# Generate dependency file
	@clang -MM -MT $@ $< > $(EBPF_DEPS_DIR)/$*.d
	@# Compile if needed
	@if [ "$(UNAME_S)" = "Darwin" ]; then \
		orca exec $(EBPF_BUILDER_VM) -- \
			"cd /flywall && make -C build/ebpf ../internal/ebpf/programs/$*.c"; \
	else \
		clang -O2 -target bpf -c $< -o $@; \
	fi

# Include dependencies
-include $(EBPF_DEPS_DIR)/*.d
```

### 3. Build Caching with Orca

```go
// internal/build/cache.go
type BuildCache struct {
    orca   *orca.Client
    vmName string
}

func (c *BuildCache) GetCacheKey(source string) (string, error) {
    // Calculate hash of source file
    hash, err := c.calculateHash(source)
    if err != nil {
        return "", err
    }

    return fmt.Sprintf("ebpf-cache-%s", hash[:8]), nil
}

func (c *BuildCache) GetCachedObject(cacheKey string) ([]byte, error) {
    vm, _ := c.orca.GetVM(c.vmName)

    // Check if cached object exists
    exists, err := vm.FileExists(fmt.Sprintf("/tmp/%s.o", cacheKey))
    if err != nil || !exists {
        return nil, fmt.Errorf("cache miss")
    }

    return vm.ReadFile(fmt.Sprintf("/tmp/%s.o", cacheKey))
}

func (c *BuildCache) StoreCachedObject(cacheKey string, data []byte) error {
    vm, _ := c.orca.GetVM(c.vmName)

    return vm.WriteFile(fmt.Sprintf("/tmp/%s.o", cacheKey), data)
}
```

## Testing Integration

### 1. Automated Testing Pipeline

```yaml
# configs/ebpf-test-pipeline.hcl
schema_version = "1.0"

pipeline {
  name = "eBPF Test Pipeline"

  stages = [
    {
      name = "build"
      vm = "ebpf-builder"
      commands = ["make ebpf-native"]
      artifacts = ["build/ebpf/*.o"]
    },

    {
      name = "unit-tests"
      vm = "ubuntu-22.04-test"
      dependencies = ["build"]
      commands = [
        "cd /flywall",
        "cp build/ebpf/*.o internal/ebpf/programs/",
        "go test ./internal/ebpf/..."
      ]
    },

    {
      name = "integration-tests"
      vm = "ubuntu-22.04-test"
      dependencies = ["unit-tests"]
      commands = [
        "cd /flywall",
        "make test-integration"
      ]
    },

    {
      name = "performance-tests"
      vm = "ubuntu-22.04-test"
      dependencies = ["integration-tests"]
      commands = [
        "cd /flywall",
        "make benchmark-ebpf"
      ]
    }
  ]
}
```

### 2. Local Testing Commands

```makefile
# Run full test suite
.PHONY: test-ebpf-full
test-ebpf-full:
	@echo "Running full eBPF test suite..."
	orca pipeline run configs/ebpf-test-pipeline.hcl

# Quick unit test (no VM restart)
.PHONY: test-ebpf-quick
test-ebpf-quick:
	@echo "Running quick eBPF tests..."
	orca exec ebpf-builder -- "cd /flywall && go test ./internal/ebpf/..."

# Test on multiple kernel versions
.PHONY: test-ebpf-matrix
test-ebpf-matrix:
	@echo "Testing eBPF on multiple kernels..."
	orca matrix run configs/ebpf-test-matrix.hcl
```

## Best Practices

### 1. VM Lifecycle Management

```go
// internal/build/vm_manager.go
type VMManager struct {
    orca *orca.Client
}

func (m *VMManager) EnsureBuilderVM() error {
    vm, err := m.orca.GetVM("ebpf-builder")
    if err != nil {
        // Create VM if doesn't exist
        return m.createBuilderVM()
    }

    // Check if VM needs update
    if m.needsUpdate(vm) {
        return m.updateBuilderVM(vm)
    }

    return nil
}

func (m *VMManager) needsUpdate(vm *orca.VM) bool {
    // Check if build tools are up to date
    output, err := vm.Execute("clang --version")
    if err != nil {
        return true
    }

    // Parse version and compare
    version := m.parseClangVersion(output)
    return version < m.minClangVersion
}
```

### 2. Resource Optimization

```yaml
# configs/ebpf-builder-optimized.hcl
vm {
  name = "ebpf-builder"
  image = "ubuntu-22.04-minimal"  # Smaller base image

  resources {
    cpu = 4          # More CPU for parallel builds
    memory = "4GB"   # More memory for large compilations
  }

  # Use tmpfs for build directory
  mounts = [
    {
      source = ".."
      destination = "/flywall"
      readonly = true  # Source is read-only
    },
    {
      type = "tmpfs"
      destination = "/tmp/build"
      size = "2GB"
    }
  ]

  # Optimized provision script
  provision = [
    "apt-get update",
    "apt-get install -y --no-install-recommends clang llvm linux-headers-generic libelf-dev libbpf-dev make golang-go",
    "rm -rf /var/lib/apt/lists/*"  # Clean up
  ]
}
```

### 3. Debugging Support

```bash
# Scripts for debugging eBPF builds
#!/bin/bash
# scripts/debug-ebpf-build.sh

VM_NAME="ebpf-builder"

echo "Connecting to eBPF builder VM for debugging..."
orca exec $VM_NAME --interactive

# Or copy debug tools
orca cp $VM_NAME:/flywall/build/ebpf/*.o .
llvm-objdump-14 -S build/ebpf/xdp_blocklist.o
```

## Migration Path

1. **Week 1**: Set up basic Orca builder VM
2. **Week 2**: Integrate with Makefile and build process
3. **Week 3**: Add caching and incremental builds
4. **Week 4**: Implement test pipeline integration
5. **Week 5**: Optimize for parallel builds and performance

This approach leverages your existing QEMU infrastructure while providing a seamless development experience for eBPF cross-compilation.
