#!/bin/bash

# Create VM image with specific kernel version for eBPF testing

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Default values
BASE_IMAGE=""
KERNEL_VERSION=""
OUTPUT_IMAGE=""
VM_NAME="flywall-test"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --base-image)
            BASE_IMAGE="$2"
            shift 2
            ;;
        --kernel-version)
            KERNEL_VERSION="$2"
            shift 2
            ;;
        --output)
            OUTPUT_IMAGE="$2"
            shift 2
            ;;
        --name)
            VM_NAME="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 --base-image IMAGE --kernel-version VERSION --output OUTPUT"
            echo "  --base-image     Base cloud image (e.g., ubuntu-22.04.img)"
            echo "  --kernel-version Kernel version to install (e.g., 5.15, 6.1, 6.5)"
            echo "  --output         Output image file"
            echo "  --name           VM name (default: flywall-test)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Check required arguments
if [[ -z "$BASE_IMAGE" || -z "$KERNEL_VERSION" || -z "$OUTPUT_IMAGE" ]]; then
    echo -e "${RED}Error: Missing required arguments${NC}"
    echo "Use --help for usage information"
    exit 1
fi

echo -e "${GREEN}Creating VM image with kernel $KERNEL_VERSION${NC}"
echo "Base image: $BASE_IMAGE"
echo "Output: $OUTPUT_IMAGE"

# Create working directory
WORK_DIR=$(mktemp -d)
trap "rm -rf $WORK_DIR" EXIT

# Copy base image
cp "$BASE_IMAGE" "$WORK_DIR/base.img"

# Create cloud-init config
cat > "$WORK_DIR/user-data" << 'EOF'
#cloud-config
package_update: true
package_upgrade: true
packages:
  - linux-image-generic
  - build-essential
  - clang
  - llvm
  - libelf-dev
  - linux-headers-generic
  - golang-go
  - git
  - qemu-guest-agent
runcmd:
  - systemctl enable qemu-guest-agent
  - systemctl start qemu-guest-agent
  - echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/environment
  - usermod -a -G libvirt ubuntu
EOF

cat > "$WORK_DIR/meta-data" << EOF
instance-id: $VM_NAME
local-hostname: $VM_NAME
EOF

# Create cloud-init ISO
cloud-localds "$WORK_DIR/cloud-init.iso" "$WORK_DIR/user-data" "$WORK_DIR/meta-data"

# Resize image
qemu-img resize "$WORK_DIR/base.img" 10G

# Start VM to install packages
echo -e "${YELLOW}Starting VM to install kernel and dependencies...${NC}"

kvm -m 2048 \
    -cpu host \
    -smp 2 \
    -hda "$WORK_DIR/base.img" \
    -cdrom "$WORK_DIR/cloud-init.iso" \
    -netdev user,id=net0,hostfwd=tcp::2222-:22 \
    -device e1000,netdev=net0 \
    -daemonize \
    -pidfile "$WORK_DIR/vm.pid"

VM_PID=$(cat "$WORK_DIR/vm.pid")

# Wait for VM to finish cloud-init
echo "Waiting for cloud-init to complete..."
for i in {1..60}; do
    if ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 -p 2222 ubuntu@localhost \
        "systemctl is-active cloud-final.service" 2>/dev/null; then
        echo "Cloud-init completed"
        break
    fi
    echo "Waiting... ($i/60)"
    sleep 10
done

# Install specific kernel version
echo -e "${YELLOW}Installing kernel $KERNEL_VERSION...${NC}"
ssh -o StrictHostKeyChecking=no -p 2222 ubuntu@localhost << EOF
    set -e

    # Add apt repository for specific kernel if needed
    case "$KERNEL_VERSION" in
        5.15)
            echo "Installing HWE kernel 5.15..."
            sudo apt-get install -y linux-image-5.15.0-91-generic linux-headers-5.15.0-91-generic
            ;;
        6.1)
            echo "Installing kernel 6.1 from mainline..."
            wget -q https://kernel.ubuntu.com/mainline/v6.1/amd64/linux-image-unsigned-6.1.0-060100-generic_6.1.0-060100.202212041355_amd64.deb
            wget -q https://kernel.ubuntu.com/mainline/v6.1/amd64/linux-modules-6.1.0-060100-generic_6.1.0-060100.202212041355_amd64.deb
            wget -q https://kernel.ubuntu.com/mainline/v6.1/amd64/linux-headers-6.1.0-060100-generic_6.1.0-060100.202212041355_all.deb
            sudo dpkg -i linux-*.deb
            rm linux-*.deb
            ;;
        6.5)
            echo "Installing kernel 6.5 from mainline..."
            wget -q https://kernel.ubuntu.com/mainline/v6.5/amd64/linux-image-unsigned-6.5.0-060500-generic_6.5.0-060500.202308130935_amd64.deb
            wget -q https://kernel.ubuntu.com/mainline/v6.5/amd64/linux-modules-6.5.0-060500-generic_6.5.0-060500.202308130935_amd64.deb
            wget -q https://kernel.ubuntu.com/mainline/v6.5/amd64/linux-headers-6.5.0-060500-generic_6.5.0-060500.202308130935_all.deb
            sudo dpkg -i linux-*.deb
            rm linux-*.deb
            ;;
        *)
            echo "Using default kernel"
            ;;
    esac

    # Update grub
    sudo update-grub

    # Clean up
    sudo apt-get autoremove -y
    sudo apt-get clean

    # Shutdown
    sudo shutdown -h now
EOF

# Wait for VM to shutdown
echo "Waiting for VM to shutdown..."
while kill -0 $VM_PID 2>/dev/null; do
    sleep 1
done

# Create final image
echo -e "${GREEN}Creating final image...${NC}"
qemu-img convert -f qcow2 -O qcow2 "$WORK_DIR/base.img" "$OUTPUT_IMAGE"

# Compress image
echo -e "${GREEN}Compressing image...${NC}"
qemu-img convert -c -O qcow2 "$OUTPUT_IMAGE" "${OUTPUT_IMAGE}.compressed"
mv "${OUTPUT_IMAGE}.compressed" "$OUTPUT_IMAGE"

echo -e "${GREEN}VM image created successfully: $OUTPUT_IMAGE${NC}"
