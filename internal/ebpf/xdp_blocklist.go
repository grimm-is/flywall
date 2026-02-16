// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

//go:build ignore

package main

import (
    "os"

    "github.com/cilium/ebpf/cmd/bpf2go"
)

func main() {
    if err := bpf2go.Gen("xdp_blocklist", "programs/xdp_blocklist.c",
        bpf2go.GoType("xdp_blocklist_types.go"),
        bpf2go.CFlags("-Wall", "-Wextra", "-O2", "-target", "bpf"),
        bpf2go.IncludePaths(".", "programs"),
    ); err != nil {
        os.Exit(1)
    }
}
