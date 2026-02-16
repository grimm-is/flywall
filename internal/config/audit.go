// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package config

// AuditConfig configures the audit logging subsystem.
type AuditConfig struct {
	// Enabled activates audit logging to SQLite.
	Enabled bool `hcl:"enabled,optional" json:"enabled"`

	// RetentionDays is the number of days to retain audit events.
	// Default: 90 days.
	RetentionDays int `hcl:"retention_days,optional" json:"retention_days,omitempty"`

	// KernelAudit enables writing to the Linux kernel audit log (auditd).
	// Useful for compliance with SOC2, HIPAA, etc.
	// Requires appropriate capabilities (CAP_AUDIT_WRITE).
	KernelAudit bool `hcl:"kernel_audit,optional" json:"kernel_audit,omitempty"`

	// DatabasePath overrides the default audit database location.
	DatabasePath string `hcl:"database_path,optional" json:"database_path,omitempty"`
}
