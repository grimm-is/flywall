# Flywall Configuration Migrations
#
# This file defines schema migrations in a declarative format.
# Each migration specifies operations to transform config from one version to another.
# Reverse migrations are automatically inferred by inverting the operations.

migration "1.0" "1.1" {
  description = "Add eBPF configuration support"

  operation "add_block" "ebpf" {
    # Defaults are applied by migrate_ebpf.go's post-load migration
  }
}
