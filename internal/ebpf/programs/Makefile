# eBPF Programs Makefile

# Directories
PROG_DIR := .
BUILD_DIR := build
INCLUDE_DIR := include
VMLINUX_DIR := ../../..

# Tools
CLANG := clang
LLVM_STRIP := llvm-strip
BPFTOOL := bpftool

# Flags
CFLAGS := -O2 -g -D__TARGET_ARCH_x86 -target bpf
INCLUDES := -I$(INCLUDE_DIR) -I$(VMLINUX_DIR) -I/usr/include

# Programs
PROGRAMS := tc_offload dns_socket dhcp_socket xdp_blocklist

# Object files
OBJS := $(patsubst %,$(BUILD_DIR)/%.o,$(PROGRAMS))

.PHONY: all clean

all: $(OBJS)

$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

$(BUILD_DIR)/%.o: $(PROG_DIR)/%.c $(VMLINUX_DIR)/vmlinux.h | $(BUILD_DIR)
	$(CLANG) $(CFLAGS) $(INCLUDES) -c $< -o $@
	$(LLVM_STRIP) -g $@

# Generate vmlinux.h if it doesn't exist
$(VMLINUX_DIR)/vmlinux.h:
	@if [ ! -f $@ ]; then \
		if [ -f /sys/kernel/btf/vmlinux ]; then \
			bpftool btf dump file /sys/kernel/btf/vmlinux format c > $@; \
		else \
			echo "Warning: Using minimal vmlinux.h - some features may not work"; \
		fi \
	fi

clean:
	@rm -rf $(BUILD_DIR)

# Install dependencies (Ubuntu/Debian)
install-deps:
	@echo "Installing eBPF development dependencies..."
	@sudo apt-get update
	@sudo apt-get install -y clang llvm linux-libc-dev libelf-dev libbpf-dev

# Show map information
show-maps: $(OBJS)
	@for prog in $(PROGRAMS); do \
		echo "=== Maps in $$prog ==="; \
		$(BPFTOOL) map list pinned /sys/fs/bpf/ 2>/dev/null || echo "No maps found"; \
	done

# Debug: Show LLVM IR
debug-ir: $(BUILD_DIR)/%.ll
$(BUILD_DIR)/%.ll: $(PROG_DIR)/%.c | $(BUILD_DIR)
	$(CLANG) $(CFLAGS) $(INCLUDES) -S -emit-llvm $< -o $@
