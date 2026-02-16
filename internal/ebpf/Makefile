# Makefile for building eBPF programs

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# eBPF parameters
CLANG=clang
LLVM_STRIP=llvm-strip
BPFTOOL=bpftool
CFLAGS=-Wall -Wextra -O2 -g -target bpf -D__TARGET_ARCH_x86
INCLUDES=-I./programs/include

# Directories
BPF_DIR=programs
BUILD_DIR=../../build/ebpf
OUTPUT_DIR=../../internal/ebpf

# Source files
BPF_SOURCES=$(wildcard $(BPF_DIR)/*.c)
BPF_OBJECTS=$(patsubst $(BPF_DIR)/%.c,$(BUILD_DIR)/%.o,$(BPF_SOURCES))
GO_GENERATED=$(patsubst $(BPF_DIR)/%.c,$(OUTPUT_DIR)/%_types.go,$(BPF_SOURCES))

.PHONY: all build clean test deps generate

all: build generate

# Create build directory
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Build eBPF object files
$(BUILD_DIR)/%.o: $(BPF_DIR)/%.c | $(BUILD_DIR)
	$(CLANG) $(CFLAGS) $(INCLUDES) -c $< -o $@
	$(LLVM_STRIP) -g $@

# Generate Go bindings
$(OUTPUT_DIR)/%_types.go: $(BUILD_DIR)/%.o
	cd $(OUTPUT_DIR) && $(GOCMD) generate -tags ebpf $*_bpf2go.go

build: $(BPF_OBJECTS)

generate: $(GO_GENERATED)

# Build specific program
xdp_blocklist: $(BUILD_DIR)/xdp_blocklist.o $(OUTPUT_DIR)/xdp_blocklist_types.go

tc_classifier: $(BUILD_DIR)/tc_classifier.o $(OUTPUT_DIR)/tc_classifier_types.go

socket_dns: $(BUILD_DIR)/socket_dns.o $(OUTPUT_DIR)/socket_dns_types.go

socket_tls: $(BUILD_DIR)/socket_tls.o $(OUTPUT_DIR)/socket_tls_types.go

socket_dhcp: $(BUILD_DIR)/socket_dhcp.o $(OUTPUT_DIR)/socket_dhcp_types.go

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f $(OUTPUT_DIR)/*_types.go
	rm -f $(OUTPUT_DIR)/*_bpf2go.go

# Run tests
test:
	$(GOTEST) -v ./...

# Download dependencies
deps:
	$(GOMOD) download
	$(GOGET) github.com/cilium/ebpf/cmd/bpf2go@latest

# Verify eBPF programs
verify: $(BPF_OBJECTS)
	@for obj in $(BPF_OBJECTS); do \
		echo "Verifying $$obj..."; \
		$(BPFTOOL) prog dump xlated $$obj > /dev/null; \
	done

# Show program information
info: $(BPF_OBJECTS)
	@for obj in $(BPF_OBJECTS); do \
		echo "=== $$obj ==="; \
		$(BPFTOOL) prog info $$obj; \
		echo; \
	done

# Install dependencies (Ubuntu/Debian)
install-deps:
	sudo apt-get update
	sudo apt-get install -y clang llvm linux-headers-generic libelf-dev libbpf-dev make pkg-config

# Help target
help:
	@echo "Available targets:"
	@echo "  all          - Build and generate all eBPF programs"
	@echo "  build        - Build eBPF object files"
	@echo "  generate     - Generate Go bindings"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run tests"
	@echo "  deps         - Download Go dependencies"
	@echo "  verify       - Verify eBPF programs"
	@echo "  info         - Show program information"
	@echo "  install-deps - Install system dependencies"
	@echo ""
	@echo "Individual programs:"
	@echo "  xdp_blocklist"
	@echo "  tc_classifier"
	@echo "  socket_dns"
	@echo "  socket_tls"
	@echo "  socket_dhcp"
