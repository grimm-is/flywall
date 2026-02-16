// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

// SecureString is a string that hides its value in JSON output.
// It is used for passwords, keys, and other sensitive data.
type SecureString string

func (s SecureString) String() string {
	if s == "" {
		return ""
	}
	return "(hidden)"
}

func (s SecureString) GoString() string {
	return "(hidden)"
}

// MarshalJSON masks the value in API responses.
func (s SecureString) MarshalJSON() ([]byte, error) {
	if s == "" {
		return []byte(`""`), nil
	}
	return []byte(`"(hidden)"`), nil
}

// UnmarshalText enables HCL decoding for this type
func (s *SecureString) UnmarshalText(text []byte) error {
	*s = SecureString(string(text))
	return nil
}
