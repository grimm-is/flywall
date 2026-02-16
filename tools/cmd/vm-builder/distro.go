// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Distro defines the interface for Linux distribution support
type Distro interface {
	// Name returns the distribution name
	Name() string

	// GetDownloads returns a list of files to download for the build
	GetDownloads(arch *ArchConfig, buildDir string) ([]FileDownload, error)

	// GetKernelArgs returns the kernel command line arguments
	GetKernelArgs(arch *ArchConfig, webPort int) string

	// GenerateProvisionScript generates the shell script to run inside the VM
	GenerateProvisionScript(config *VMConfig, arch *ArchConfig, buildDir string, projectRoot string, webPort int, vmMountPath string, binaryName string) (string, error)

	// GetKernelPath returns the path to the kernel image
	GetKernelPath(buildDir string) string

	// GetInitrdPath returns the path to the initramfs
	GetInitrdPath(buildDir string) string
}

// FileDownload represents a file to be downloaded
type FileDownload struct {
	URL  string
	Name string // Local filename
}

// GetDistro returns the appropriate Distro implementation based on config
func GetDistro(config *VMConfig) (Distro, error) {
	distro := "alpine"
	if config != nil && config.OS.Distro != "" {
		distro = strings.ToLower(config.OS.Distro)
	}

	switch distro {
	case "alpine":
		return NewAlpineDistro(config), nil
	default:
		return nil, fmt.Errorf("unsupported distribution: %s", distro)
	}
}

// AlpineDistro implements Distro for Alpine Linux
type AlpineDistro struct {
	Version string
	Release string
	Mirror  string
}

func NewAlpineDistro(config *VMConfig) *AlpineDistro {
	version := "3.22.2" // Default fallback
	if config != nil && config.OS.Version != "" {
		version = config.OS.Version
	}

	release := ""
	if config != nil {
		release = config.OS.Release
	}
	if release == "" {
		// Derive release from version (e.g. 3.22.2 -> v3.22)
		parts := strings.Split(version, ".")
		if len(parts) >= 2 {
			release = "v" + parts[0] + "." + parts[1]
		} else {
			release = "v3.22" // Fallback
		}
	}

	mirror := ""
	if config != nil {
		mirror = config.OS.Mirror
	}
	if mirror == "" {
		mirror = "http://dl-cdn.alpinelinux.org/alpine"
	}

	return &AlpineDistro{
		Version: version,
		Release: release,
		Mirror:  mirror,
	}
}

func (d *AlpineDistro) Name() string {
	return "Alpine Linux " + d.Version
}

func (d *AlpineDistro) GetDownloads(arch *ArchConfig, buildDir string) ([]FileDownload, error) {
	baseURL := fmt.Sprintf("%s/%s/releases/%s/netboot", d.Mirror, d.Release, arch.Arch)

	return []FileDownload{
		{baseURL + "/vmlinuz-virt", "vmlinuz-virt-" + d.Version},
		{baseURL + "/initramfs-virt", "initramfs-virt-" + d.Version},
		{baseURL + "/modloop-virt", "modloop-virt-" + d.Version},
	}, nil
}

func (d *AlpineDistro) GetKernelArgs(arch *ArchConfig, webPort int) string {
	// console=ttyS0 ip=dhcp modloop=http://... alpine_repo=...
	return fmt.Sprintf("console=%s ip=dhcp modloop=http://10.0.2.2:%d/modloop-virt-%s alpine_repo=%s/%s/main",
		arch.ConsoleTTY, webPort, d.Version, d.Mirror, d.Release)
}

// GenerateProvisionScript generates the Alpine provisioning script
// Note: Logic moved/adapted from main.go
func (d *AlpineDistro) GenerateProvisionScript(config *VMConfig, arch *ArchConfig, buildDir string, projectRoot string, webPort int, vmMountPath string, binaryName string) (string, error) {
	packages := strings.Join(config.Packages, " ")

	script := `#!/bin/sh
set -e
echo "🔧 [Guest] Starting Alpine Install (` + d.Version + `)..."

# 1. Prepare Disks
echo "🔧 [Guest] Formatting disk..."
apk add --quiet --no-cache e2fsprogs
modprobe ext4
mkfs.ext4 -F -q /dev/vda
mkdir -p /mnt/target
mount -t ext4 /dev/vda /mnt/target

# 2. Install Alpine
echo "🔧 [Guest] Downloading RootFS..."
cd /mnt/target
wget -q -O rootfs.tar.gz ` + d.Mirror + `/` + d.Release + `/releases/` + arch.Arch + `/alpine-minirootfs-` + d.Version + `-` + arch.Arch + `.tar.gz
tar -xzf rootfs.tar.gz
rm rootfs.tar.gz

# 3. Configure System
echo "🔧 [Guest] Configuring System..."
echo "nameserver 8.8.8.8" > etc/resolv.conf
echo "` + d.Mirror + `/` + d.Release + `/main" > etc/apk/repositories
echo "` + d.Mirror + `/` + d.Release + `/community" >> etc/apk/repositories

# Remove default Alpine MOTD
rm -f etc/motd

# Set hostname based on binary name (usually flywall)
lower_name=$(echo "` + binaryName + `" | tr '[:upper:]' '[:lower:]')
echo "$lower_name" > etc/hostname
echo "127.0.0.1 $lower_name localhost" > etc/hosts

# Create directories that minirootfs doesn't have but packages expect
mkdir -p etc/security usr/lib/pam.d usr/lib/security/pam_filter

# Install Kernel & Tools via chroot
chroot /mnt/target apk add --no-cache ` + packages + `

# Enable Core Services
chroot /mnt/target rc-update add loopback boot
chroot /mnt/target rc-update add devfs sysinit
chroot /mnt/target rc-update add dmesg sysinit

# --- FLYWALL CONFIGURATION ---
linux_arch="` + arch.LinuxArch() + `"
ln -sf ` + vmMountPath + `/build/` + binaryName + `-linux-$linux_arch usr/sbin/` + binaryName + `
`
	// Apply Post-Install Steps
	for _, step := range config.PostInstall {
		script += fmt.Sprintf("\n# Step: %s\n", step.Name)

		if step.Copy != nil {
			// Logic for copy (generate wget command)
			// We need to write the file to buildDir so it can be served
			// The main.go HTTP server serves buildDir

			srcPath := filepath.Join(projectRoot, step.Copy.Source)
			destName := filepath.Base(step.Copy.Dest)

			// Read source
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return "", fmt.Errorf("failed to read source file %s: %w", srcPath, err)
			}

			// Write to build dir
			buildPath := filepath.Join(buildDir, destName)
			if err := os.WriteFile(buildPath, data, 0644); err != nil {
				return "", fmt.Errorf("failed to stage file %s: %w", destName, err)
			}

			// Add download command to script
			script += fmt.Sprintf("mkdir -p $(dirname %s)\n", step.Copy.Dest)
			script += fmt.Sprintf("wget -q -O %s http://10.0.2.2:%d/%s\n", step.Copy.Dest, webPort, destName)

			if step.Copy.Mode != "" {
				script += fmt.Sprintf("chmod %s %s\n", step.Copy.Mode, step.Copy.Dest)
			}
		}

		if step.Run != "" {
			script += fmt.Sprintf("%s\n", step.Run)
		}
	}

	// ... Initab and rest ...
	// Using hardcoded strings from main.go again directly here:

	inittab := fmt.Sprintf(`::sysinit:/sbin/openrc sysinit
::sysinit:/sbin/openrc boot
::wait:/sbin/openrc default

# Run dispatcher script on console
%s:2345:once:/root/entrypoint_dispatcher.sh

::shutdown:/sbin/openrc shutdown
::ctrlaltdel:/sbin/reboot
`, arch.ConsoleTTY)

	script += fmt.Sprintf("\ncat > etc/inittab <<'INITTAB'\n%sINITTAB\n", inittab)
	script += fmt.Sprintf("echo \"%s\" >> etc/securetty\n", arch.ConsoleTTY)

	// Network Config
	script += `cat > etc/network/interfaces <<'END'
auto lo
iface lo inet loopback
END
`
	// Runlevels
	script += `ln -s /etc/init.d/urandom etc/runlevels/boot/
ln -s /etc/init.d/localmount etc/runlevels/boot/
ln -s /etc/init.d/hostname etc/runlevels/boot/

echo "root:root" | chroot . chpasswd

# Shared Folder Setup
mkdir -p mnt/` + strings.ToLower(binaryName) + `
mkdir -p mnt/build
mkdir -p mnt/assets
mkdir -p mnt/worker

# host_share
echo "host_share ` + vmMountPath + ` 9p trans=virtio,version=9p2000.L,ro,msize=262144 0 0" >> etc/fstab
# build_share
echo "build_share /mnt/build 9p trans=virtio,version=9p2000.L,ro,msize=262144 0 0" >> etc/fstab
# assets_share
echo "assets_share /mnt/assets 9p trans=virtio,version=9p2000.L,msize=262144 0 0" >> etc/fstab
# worker_share
echo "worker_share /mnt/worker 9p trans=virtio,version=9p2000.L,msize=262144 0 0" >> etc/fstab
echo "9p" >> etc/modules
echo "9pnet" >> etc/modules
echo "9pnet_virtio" >> etc/modules
`

	// --- ENTRYPOINT DISPATCHER ---
	dispatcher := `#!/bin/sh
# VM Entrypoint Dispatcher
# Determines which test/script to run based on kernel command line

# Helper function to exit VM with proper exit code propagation
qemu_exit() {
    local exit_code="${1:-0}"
    local arch=$(uname -m)

    if [ "$arch" = "aarch64" ] && [ -x ` + vmMountPath + `/build/qemu-exit-arm64 ]; then
        ` + vmMountPath + `/build/qemu-exit-arm64 "$exit_code"
        return
    elif [ "$arch" = "x86_64" ] && [ -x ` + vmMountPath + `/build/qemu-exit-amd64 ]; then
        ` + vmMountPath + `/build/qemu-exit-amd64 "$exit_code"
        return
    fi

    case "$arch" in
        x86_64|i686|i386)
            if [ "$exit_code" = "0" ]; then value=0; else value=1; fi
            if [ -c /dev/port ]; then
                printf "\\x$(printf '%02x' $value)" | dd of=/dev/port bs=1 seek=244 count=1 2>/dev/null
            else
                poweroff -f 2>/dev/null
            fi
            ;;
        *)
            poweroff -f 2>/dev/null
            ;;
    esac
}

# Mount shared folders if not already mounted (redundant safety check)
# fstab should handle this, but we ensure they're available
if ! mountpoint -q ` + vmMountPath + ` 2>/dev/null; then
    mount -t 9p -o trans=virtio,version=9p2000.L,ro,msize=262144 host_share ` + vmMountPath + ` 2>/dev/null || true
fi
if ! mountpoint -q /mnt/build 2>/dev/null; then
    mount -t 9p -o trans=virtio,version=9p2000.L,ro,msize=262144 build_share /mnt/build 2>/dev/null || true
fi
if ! mountpoint -q /mnt/assets 2>/dev/null; then
    mount -t 9p -o trans=virtio,version=9p2000.L,msize=262144 assets_share /mnt/assets 2>/dev/null || true
fi
if ! mountpoint -q /mnt/worker 2>/dev/null; then
    mount -t 9p -o trans=virtio,version=9p2000.L,msize=262144 worker_share /mnt/worker 2>/dev/null || true
fi

# Ensure loopback is up and has 127.0.0.1 for agent communication
# We do this in a loop because some init scripts might flush it
(
    while true; do
        if ! ip addr show lo | grep -q "127.0.0.1"; then
            ip link set up dev lo
            ip addr add 127.0.0.1/8 dev lo
        fi
        sleep 5
    done
) &

# Check boot mode from kernel cmdline
if cat /proc/cmdline | grep -q "test_mode=true"; then
    if [ -f ` + vmMountPath + `/scripts/vm/entrypoint-test.sh ]; then
        sh ` + vmMountPath + `/scripts/vm/entrypoint-test.sh
        test_exit=$?
        qemu_exit $test_exit
    fi
elif cat /proc/cmdline | grep -q "agent_mode=true"; then
    echo "⚡ Starting in AGENT-ONLY mode..."
    # Run the agent binary directly (no ctl/api) - use /mnt/build for read-only binaries
    if [ -x /mnt/build/orca-agent ]; then
        exec /mnt/build/orca-agent agent
    elif [ -x /mnt/build/toolbox-linux-$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/') ]; then
        exec /mnt/build/toolbox-linux-$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/') agent
    elif [ -x /mnt/build/toolbox-linux ]; then
        exec /mnt/build/toolbox-linux agent
    else
        echo "❌ Agent binary not found in /mnt/build"
        qemu_exit 1
    fi
elif cat /proc/cmdline | grep -q "dev_mode=true"; then
    if [ -f ` + vmMountPath + `/scripts/vm/entrypoint-dev.sh ]; then
        echo "⚡ Starting in development mode..."
        sh ` + vmMountPath + `/scripts/vm/entrypoint-dev.sh
    fi
elif cat /proc/cmdline | grep -q "client_vm="; then
    if [ -f ` + vmMountPath + `/scripts/vm/entrypoint-client.sh ]; then
        echo "⚡ Found entrypoint-client.sh. Executing..."
        sh ` + vmMountPath + `/scripts/vm/entrypoint-client.sh
    fi
elif cat /proc/cmdline | grep -q "single_vm_test=true"; then
    if [ -f ` + vmMountPath + `/tests/single_vm_zone_test.sh ]; then
        echo "⚡ Running single VM zone test..."
        sh ` + vmMountPath + `/tests/single_vm_zone_test.sh
        test_exit=$?
        qemu_exit $test_exit
    fi
elif cat /proc/cmdline | grep -q "config="; then
    if [ -f ` + vmMountPath + `/scripts/vm/entrypoint-firewall.sh ]; then
        echo "⚡ Found entrypoint-firewall.sh. Executing..."
        sh ` + vmMountPath + `/scripts/vm/entrypoint-firewall.sh
    fi
else
    if [ -f ` + vmMountPath + `/scripts/vm/entrypoint.sh ]; then
        echo "⚡ Found entrypoint.sh. Executing..."
        sh ` + vmMountPath + `/scripts/vm/entrypoint.sh
    else
        echo "⚡ No mode selected. Starting interactive shell..."
        exec /bin/sh -l
    fi
fi
`
	script += fmt.Sprintf("\ncat > root/entrypoint_dispatcher.sh <<'DISPATCHER'\n%sDISPATCHER\n", dispatcher)
	script += "chmod +x root/entrypoint_dispatcher.sh\n"

	// Upload Artifacts
	script += `
# 4. Upload Artifacts to Host
echo "📤 [Guest] Uploading new kernel to host..."
apk add --quiet --no-cache curl
curl -s -T /mnt/target/boot/vmlinuz-virt http://10.0.2.2:` + fmt.Sprintf("%d", webPort) + `/vmlinuz
curl -s -T /mnt/target/boot/initramfs-virt http://10.0.2.2:` + fmt.Sprintf("%d", webPort) + `/initramfs

cd /
sync
umount /mnt/target
sync
echo "✅ BUILD_COMPLETE"
poweroff
`
	return script, nil
}

func (d *AlpineDistro) GetKernelPath(buildDir string) string {
	return filepath.Join(buildDir, "vmlinuz-virt-"+d.Version)
}

func (d *AlpineDistro) GetInitrdPath(buildDir string) string {
	return filepath.Join(buildDir, "initramfs-virt-"+d.Version)
}
