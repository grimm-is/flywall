// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package network

// ReadSysctl reads a sysctl value from the specified path.
func ReadSysctl(path string) (string, error) {
	return DefaultSystemController.ReadSysctl(path)
}

// WriteSysctl writes a sysctl value to the specified path.
func WriteSysctl(path, value string) error {
	return DefaultSystemController.WriteSysctl(path, value)
}

// IsNotExist checks if an error indicates that a file or directory does not exist.
func IsNotExist(err error) bool {
	return DefaultSystemController.IsNotExist(err)
}
