// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package vmm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type VM struct {
	Config            Config
	CID               uint32 // vsock Context ID (Linux only)
	SocketPath        string // Unix socket path (macOS)
	OverlayPath       string // Overlay qcow2 path for cleanup
	WorkerScratchPath string // Per-worker scratch directory for cleanup
	cmd               *exec.Cmd
}

func NewVM(cfg Config, id int) (*VM, error) {
	// Verify artifacts
	if _, err := os.Stat(cfg.KernelPath); err != nil {
		return nil, fmt.Errorf("kernel not found at %s", cfg.KernelPath)
	}

	vm := &VM{Config: cfg}

	// Determine base directory for ephemeral files
	baseDir := os.TempDir()
	if cfg.BuildDir != "" {
		baseDir = cfg.BuildDir
	}

	// Create per-worker scratch directory for isolated runtime state
	var scratchPath string
	if cfg.ArtifactDir != "" {
		// Use persistent artifact directory for test runs
		scratchPath = filepath.Join(cfg.ArtifactDir, fmt.Sprintf("worker-%d", id))
	} else {
		// Use temporary directory for non-test runs
		scratchPath = filepath.Join(baseDir, fmt.Sprintf("flywall-worker-%d-%d", os.Getpid(), id))
	}

	// Use 0777 to ensure 'nobody' user inside VM can write to it (e.g. via 9p mounts)
	if err := os.MkdirAll(scratchPath, 0777); err != nil {
		return nil, fmt.Errorf("failed to create worker scratch dir: %w", err)
	}
	// Explicitly chmod to 0777 in case umask restricted it
	os.Chmod(scratchPath, 0777)
	vm.WorkerScratchPath = scratchPath

	if runtime.GOOS == "linux" {
		// On Linux with KVM, use vsock with CID >= 3
		vm.CID = uint32(id + 2)
	} else {
		// On macOS, use Unix sockets
		vm.SocketPath = filepath.Join(baseDir, fmt.Sprintf("flywall-vm%d.sock", id))
	}

	return vm, nil
}

func (v *VM) Start(ctx context.Context) error {
	// Architecture detection
	var qemuBin, machine, cpu, consoleTTY string
	switch runtime.GOARCH {
	case "arm64":
		qemuBin = "qemu-system-aarch64"
		machine = "virt,accel=hvf" // macOS ARM64
		cpu = "cortex-a72"
		consoleTTY = "ttyAMA0"
	default: // amd64
		qemuBin = "qemu-system-x86_64"
		if runtime.GOOS == "darwin" {
			machine = "q35,accel=hvf"
		} else {
			machine = "q35,accel=kvm"
		}
		cpu = "host"
		consoleTTY = "ttyS0"
	}

	// Unique overlay per VM
	var overlayID string
	if v.CID != 0 {
		overlayID = fmt.Sprintf("cid%d", v.CID)
	} else {
		overlayID = filepath.Base(v.SocketPath)
	}
	baseDir := os.TempDir()
	if v.Config.BuildDir != "" {
		baseDir = v.Config.BuildDir
	}
	overlayPath := filepath.Join(baseDir, fmt.Sprintf("flywall-overlay-%d-%s.qcow2", os.Getpid(), overlayID))

	imgBin := findBinary("qemu-img")
	createCmd := exec.Command(imgBin, "create", "-f", "qcow2", "-b", v.Config.RootfsPath, "-F", "qcow2", overlayPath)
	if out, err := createCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create overlay (bin: %s): %v (%s)", imgBin, err, out)
	}
	v.OverlayPath = overlayPath // Store for cleanup in Stop()

	modeArg := "agent_mode=true"
	if v.Config.DevMode {
		modeArg = "dev_mode=true"
	}

	kernelArgs := fmt.Sprintf(
		"root=/dev/vda rw console=%s earlyprintk=serial rootwait modules=ext4 "+
			"printk.time=1 console_msg_format=syslog rc_nocolor=YES "+
			"%s quiet loglevel=0",
		consoleTTY, modeArg,
	)
	out := v.Config.Stdout
	if out == nil {
		out = os.Stdout
	}
	if v.Config.Debug {
		fmt.Fprintf(out, "DEBUG: DevMode=%v, Identity=%s\n", v.Config.DevMode, modeArg)
		fmt.Fprintf(out, "DEBUG: KernelArgs: %s\n", kernelArgs)
	}
	if v.Config.RunSkipped {
		kernelArgs += " flywall.run_skipped=1"
	}

	args := []string{
		"-machine", machine,
		"-cpu", cpu,
		"-smp", "1",
		"-m", fmt.Sprintf("%dM", func() int {
			if v.Config.MemoryMB > 0 {
				return v.Config.MemoryMB
			}
			return 256
		}()),
		"-nographic",
		"-no-reboot",

		"-kernel", v.Config.KernelPath,
		"-initrd", v.Config.InitrdPath,
		"-append", kernelArgs,

		"-drive", fmt.Sprintf("file=%s,format=qcow2,if=virtio,id=system_disk", overlayPath),

		// Host Share - Read-only for project root (source code, test scripts)
		"-virtfs", fmt.Sprintf("local,path=%s,mount_tag=host_share,security_model=none,readonly=on,id=host_share", v.Config.ProjectRoot),
	}

	// Assets Share - Shared writable with POSIX locking (ipsets, geoip, caches)
	assetsPath := v.Config.AssetsSharePath
	if assetsPath == "" {
		// Use a per-worker isolated assets directory for tests to avoid parallel conflicts
		assetsPath = filepath.Join(v.WorkerScratchPath, "assets")
	}
	os.MkdirAll(assetsPath, 0777)
	os.Chmod(assetsPath, 0777)
	args = append(args, "-virtfs", fmt.Sprintf("local,path=%s,mount_tag=assets_share,security_model=none,id=assets_share", assetsPath))

	// Worker Share - Per-worker isolated scratch (sockets, state, logs)
	args = append(args, "-virtfs", fmt.Sprintf("local,path=%s,mount_tag=worker_share,security_model=none,id=worker_share", v.WorkerScratchPath))

	// Build Share - Read-only binaries (defaults to ProjectRoot/build)
	buildSharePath := v.Config.BuildSharePath
	if buildSharePath == "" {
		buildSharePath = filepath.Join(v.Config.ProjectRoot, "build")
	}
	args = append(args, "-virtfs", fmt.Sprintf("local,path=%s,mount_tag=build_share,security_model=none,readonly=on,id=build_share", buildSharePath))

	// Add transport-specific devices
	if runtime.GOOS == "linux" && v.CID != 0 {
		// Linux: use vsock
		args = append(args,
			"-device", fmt.Sprintf("vhost-vsock-pci,guest-cid=%d", v.CID),
		)
	} else {
		// macOS: use virtio-serial with Unix socket
		_ = os.Remove(v.SocketPath)
		args = append(args,
			"-device", "virtio-serial-pci",
			"-chardev", fmt.Sprintf("socket,path=%s,server=on,wait=off,id=channel0", v.SocketPath),
			"-device", "virtserialport,chardev=channel0,name=flywall.agent",
		)
	}

	// Networking
	if v.Config.InterfaceCount <= 0 {
		v.Config.InterfaceCount = 4 // Default to 4 interfaces (WAN + 3 LAN)
	}

	// eth0 (WAN)
	wanNetdev := "user,id=wan"
	for host, guest := range v.Config.ForwardPorts {
		wanNetdev += fmt.Sprintf(",hostfwd=tcp::%d-:%d", host, guest)
	}

	args = append(args,
		"-netdev", wanNetdev,
		"-device", "virtio-net-pci,netdev=wan,mac=52:54:00:11:00:01",
	)

	// eth1..ethN (LANs)
	// Mac address incrementing strategy: 52:54:00:22:00:XX
	for i := 1; i < v.Config.InterfaceCount; i++ {
		netID := fmt.Sprintf("lan%d", i)
		macAddr := fmt.Sprintf("52:54:00:22:00:%02x", i)

		args = append(args,
			"-netdev", fmt.Sprintf("user,id=%s", netID),
			"-device", fmt.Sprintf("virtio-net-pci,netdev=%s,mac=%s", netID, macAddr),
		)
	}

	qemuFullPath := findBinary(qemuBin)
	v.cmd = exec.CommandContext(ctx, qemuFullPath, args...)

	if v.Config.Debug {
		v.cmd.Stdout = os.Stdout
		v.cmd.Stderr = os.Stderr
	}

	if v.Config.Stdin != nil {
		v.cmd.Stdin = v.Config.Stdin
		// Force output if attaching stdin, otherwise user is typing blindly
		// Respect Config overrides if provided
		if v.Config.Stdout != nil {
			v.cmd.Stdout = v.Config.Stdout
		} else {
			v.cmd.Stdout = os.Stdout
		}
		if v.Config.Stderr != nil {
			v.cmd.Stderr = v.Config.Stderr
		} else {
			v.cmd.Stderr = os.Stderr
		}
	} else {
		// Even if no stdin, respect stdout/stderr overrides if provided
		if v.Config.Stdout != nil {
			v.cmd.Stdout = v.Config.Stdout
		}
		if v.Config.Stderr != nil {
			v.cmd.Stderr = v.Config.Stderr
		}
	}

	return v.cmd.Run()
}

func (v *VM) Stop() error {
	if v.cmd != nil && v.cmd.Process != nil {
		v.cmd.Process.Kill()
	}

	// Clean up overlay file
	if v.OverlayPath != "" {
		os.Remove(v.OverlayPath)
	}

	// Clean up socket file (macOS)
	if v.SocketPath != "" {
		os.Remove(v.SocketPath)
	}

	// Clean up per-worker scratch directory (only if it's temporary)
	if v.WorkerScratchPath != "" && v.Config.ArtifactDir == "" {
		os.RemoveAll(v.WorkerScratchPath)
	}

	return nil
}

func findBinary(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}

	// Common locations on macOS/Linux if not in PATH
	extraPaths := []string{
		"/usr/local/bin/" + name,
		"/opt/homebrew/bin/" + name,
		"/usr/bin/" + name,
		"/bin/" + name,
	}

	for _, p := range extraPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return name // Fallback to original, which will eventually fail
}
