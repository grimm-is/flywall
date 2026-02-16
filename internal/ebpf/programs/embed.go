// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package programs

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@latest --no-strip --target=bpfel --cc=/opt/homebrew/opt/llvm/bin/clang TcOffload c/tc_offload.c -- -O2 -target bpf -I.
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@latest --no-strip --target=bpfel --cc=/opt/homebrew/opt/llvm/bin/clang DnsSocket c/dns_socket.c -- -O2 -target bpf -I.
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@latest --no-strip --target=bpfel --cc=/opt/homebrew/opt/llvm/bin/clang DhcpSocket c/dhcp_socket.c -- -O2 -target bpf -I.
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@latest --no-strip --target=bpfel --cc=/opt/homebrew/opt/llvm/bin/clang XdpBlocklist c/xdp_blocklist.c -- -O2 -target bpf -I.
