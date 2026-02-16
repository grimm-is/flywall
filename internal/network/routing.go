// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package network

// RunCommand runs a command and returns its stdout.
func RunCommand(name string, arg ...string) (string, error) {
	return DefaultCommandExecutor.RunCommand(name, arg...)
}
