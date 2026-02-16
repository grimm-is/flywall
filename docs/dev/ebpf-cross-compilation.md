# eBPF Cross-Compilation Strategy

## Overview

eBPF programs require Linux-specific tools and headers for compilation. This document outlines the cross-compilation strategy for macOS development hosts.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   macOS Host    │    │  Linux Builder  │    │  Target System  │
│                 │    │   (VM/Container)│    │   (Linux)       │
│ - Go Code       │    │ - eBPF Compiler │    │ - eBPF Runtime  │
│ - Editors       │───▶│ - Linux Headers │───▶│ - Loaded Programs│
│ - Git           │    │ - LLVM/Clang    │    │ - Maps          │
│ - Testing       │    │ - Make          │    │ - Hooks         │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Solution Options

### Option 1: Docker-based Cross-Compilation (Recommended)

#### Dockerfile for eBPF Compilation
```dockerfile
# internal/ebpf/build/Dockerfile
FROM ubuntu:22.04

# Install eBPF dependencies
RUN apt-get update && apt-get install -y \
    clang \
    llvm \
    linux-headers-$(uname -r) \
    libelf-dev \
    libbpf-dev \
    make \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

# Install Go for embedding
ENV GO_VERSION=1.21.5
RUN wget -O /tmp/go.tar.gz https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"
ENV GO111MODULE=on
ENV CGO_ENABLED=0

# Create build directory
WORKDIR /build

# Copy source code
COPY . .

# Build eBPF programs and embed
CMD ["make", "ebpf-embed"]
```

#### Makefile Integration
```makefile
# Makefile
EBPF_BUILD_DIR = internal/ebpf/build
EBPF_OUTPUT_DIR = build/ebpf

.PHONY: ebpf
ebpf:
	@echo "Building eBPF programs..."
	@if [ "$(shell uname)" = "Darwin" ]; then \
		echo "Using Docker for cross-compilation"; \
		docker build -t flywall-ebpf-builder $(EBPF_BUILD_DIR); \
		docker run --rm -v "$(PWD)":/build -w /build flywall-ebpf-builder; \
	else \
		echo "Building natively on Linux"; \
		$(MAKE) ebpf-native; \
	fi

.PHONY: ebpf-native
ebpf-native:
	@mkdir -p $(EBPF_OUTPUT_DIR)
	@for prog in $(EBPF_DIR)/*.c; do \
		$(CLANG) -O2 -target bpf -c $$prog -o $(EBPF_OUTPUT_DIR)/$$(basename $$prog .c).o; \
	done
	@go generate ./internal/ebpf/...

.PHONY: ebpf-embed
ebpf-embed: ebpf-native
	@go embed ./internal/ebpf/programs

# Development helper
.PHONY: ebpf-dev
ebpf-dev:
	@echo "Starting eBPF development container..."
	@if [ "$(shell uname)" = "Darwin" ]; then \
		docker run -it --rm -v "$(PWD)":/build -w /build flywall-ebpf-builder bash; \
	else \
		bash; \
	fi
```

### Option 2: VM-based Build Environment

#### Vagrant Configuration
```ruby
# Vagrantfile
Vagrant.configure("2") do |config|
  config.vm.box = "ubuntu/jammy64"

  # Allocate resources for compilation
  config.vm.provider "virtualbox" do |vb|
    vb.memory = "2048"
    vb.cpus = "2"
  end

  # Provision eBPF build environment
  config.vm.provision "shell", inline: <<-SHELL
    apt-get update
    apt-get install -y clang llvm linux-headers-generic libelf-dev libbpf-dev make
    apt-get install -y golang-go

    # Mount shared directory
    mkdir -p /host
    mount -t vboxsf host /host 2>/dev/null || true
  SHELL

  # Share project directory
  config.vm.synced_folder ".", "/host"
end
```

#### Build Script
```bash
#!/bin/bash
# scripts/build-ebpf.sh

set -e

echo "Building eBPF programs..."

if [ "$(uname)" = "Darwin" ]; then
    echo "Detected macOS, using VM for compilation"

    # Start VM if not running
    if ! vagrant status | grep -q "running"; then
        vagrant up
    fi

    # Build in VM
    vagrant ssh -c "cd /host && make ebpf-native"

    # Copy artifacts back
    vagrant ssh -c "cp -r /host/build/ebpf/*.o /host/build/ebpf/" 2>/dev/null || true

else
    echo "Building natively on Linux"
    make ebpf-native
fi

echo "eBPF build complete!"
```

### Option 3: GitHub Actions CI/CD

#### GitHub Actions Workflow
```yaml
# .github/workflows/ebpf-build.yml
name: eBPF Build

on:
  push:
    paths:
      - 'internal/ebpf/**'
      - '**.go'
  pull_request:
    paths:
      - 'internal/ebpf/**'

jobs:
  build-ebpf:
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v4

    - name: Install eBPF dependencies
      run: |
        sudo apt-get update
        sudo apt-get install -y clang llvm linux-headers-generic libelf-dev libbpf-dev

    - name: Setup Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'

    - name: Build eBPF programs
      run: make ebpf

    - name: Run eBPF tests
      run: go test ./internal/ebpf/...

    - name: Upload eBPF artifacts
      uses: actions/upload-artifact@v3
      with:
        name: ebpf-programs
        path: build/ebpf/*.o
```

## Implementation Details

### 1. Go Generate Integration

```go
// internal/ebpf/programs/embed.go
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@latest \
//    --type=xdp_program \
//    --cc=clang \
//    -cflags "-O2 -target bpf" \
//    xdp_program \
//    programs/xdp_blocklist.c

package programs

import (
    "embed"
)

//go:embed *.o
var Programs embed.FS

func LoadProgram(name string) ([]byte, error) {
    data, err := Programs.ReadFile(name + ".o")
    if err != nil {
        return nil, err
    }
    return data, nil
}
```

### 2. Development Workflow

#### On macOS:
```bash
# 1. Make changes to eBPF programs
vim internal/ebpf/programs/xdp_blocklist.c

# 2. Build (automatically uses Docker)
make ebpf

# 3. Test locally (if you have a Linux VM or remote)
make test-integration TARGET=linux-vm

# 4. Commit changes
git add .
git commit -m "Update XDP blocklist program"
```

#### Quick Iteration:
```bash
# Start development container
make ebpf-dev

# Inside container
# Edit and compile quickly
clang -O2 -target bpf -c program.c -o program.o
# Test with bpftool
bpftool prog load program.o /sys/fs/bpf/test
```

### 3. IDE Integration

#### VS Code Configuration
```json
// .vscode/settings.json
{
    "ebpf.clangPath": "/usr/bin/clang",
    "ebpf.includePaths": [
        "/usr/include",
        "${workspaceFolder}/internal/ebpf/include"
    ],
    "files.associations": {
        "*.c": "c",
        "*.h": "c"
    }
}
```

#### Remote Development
```bash
# Use VS Code Remote SSH to connect to build VM
code --remote ssh-remote+ubuntu@ebpf-builder

# Or use Docker extension for container development
```

### 4. Testing Strategy

#### Unit Tests (Run on macOS)
```go
// internal/ebpf/loader_test.go
func TestLoadEmbeddedProgram(t *testing.T) {
    // Test loading embedded programs
    data, err := programs.LoadProgram("xdp_blocklist")
    require.NoError(t, err)
    assert.NotEmpty(t, data)
}
```

#### Integration Tests (Linux Only)
```bash
# scripts/run-integration-tests.sh
#!/bin/bash

if [ "$(uname)" != "Linux" ]; then
    echo "Integration tests require Linux, using remote VM..."
    ssh root@ebpf-test-vm "cd /flywall && make test-integration"
else
    make test-integration
fi
```

### 5. Release Process

#### Automated Release Build
```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4

    - name: Build release binaries
      run: |
        # Build for multiple architectures
        GOOS=linux GOARCH=amd64 make build
        GOOS=linux GOARCH=arm64 make build

    - name: Create release
      uses: actions/create-release@v1
      with:
        tag_name: ${{ github.ref }}
        release_name: Release ${{ github.ref }}
```

## Best Practices

### 1. Keep eBPF Programs Self-Contained
```c
// Use relative includes
#include "common.h"  // Not <linux/if_ether.h>

// Define constants locally
#define ETH_P_IP 0x0800
```

### 2. Version Compatibility
```c
// Check kernel version at runtime
#if LINUX_VERSION_CODE >= KERNEL_VERSION(5, 8, 0)
    // Use new feature
#else
    // Fallback implementation
#endif
```

### 3. Build Caching
```makefile
# Use build cache
EBPF_CACHE_DIR = .cache/ebpf

%.o: %.c
	@mkdir -p $(EBPF_CACHE_DIR)
	$(CLANG) -O2 -target bpf -c $< -o $(EBPF_CACHE_DIR)/$@
	@cp $(EBPF_CACHE_DIR)/$@ $@
```

## Troubleshooting

### Common Issues

1. **Header Not Found**
   ```bash
   # Install headers for specific kernel
   sudo apt install linux-headers-$(uname -r)
   ```

2. **Clang Version Mismatch**
   ```bash
   # Use specific clang version
   clang-14 -O2 -target bpf -c program.c -o program.o
   ```

3. **Permission Errors**
   ```bash
   # Fix Docker permissions
   sudo usermod -aG docker $USER
   ```

### Debug Mode
```bash
# Verbose build
make ebpf V=1

# Debug compilation
docker run --rm -v "$(PWD)":/build -w /build flywall-ebpf-builder \
    bash -c "clang -v -O2 -target bpf -c programs/test.c -o test.o"
```

## Migration Path

1. **Phase 1**: Set up Docker build environment
2. **Phase 2**: Integrate with existing Makefile
3. **Phase 3**: Add GitHub Actions for CI/CD
4. **Phase 4**: Optimize for developer experience
5. **Phase 5**: Add cross-compilation for other architectures

This approach ensures seamless development on macOS while building eBPF programs in a proper Linux environment.
