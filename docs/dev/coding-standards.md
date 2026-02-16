# Flywall Coding Standards

This document outlines the coding standards and conventions for the Flywall project. Adhering to these standards ensures code consistency, maintainability, and reliability.

## 1. General Philosophy

*   **Simplicity**: Prefer simple, readable code over clever, complex solutions.
*   **Safety**: prioritize type safety and explicit error handling. Avoid panics in production code.
*   **Idiomatic Go**: Follow standard Go conventions (Effective Go) unless there is a strong reason not to.
*   **Privilege Separation**: Respect the architecture's privilege boundaries (root `ctlplane` vs unprivileged `api`).

## 2. File Structure & Organization

*   **Imports**: Group imports into three blocks separated by newlines:
    1.  Standard Library (`"fmt"`, `"os"`)
    2.  Project Internal (`"grimm.is/flywall/internal/..."`)
    3.  Third-party (`"github.com/..."`)
*   **Package Layout**:
    *   `cmd/`: Main applications.
    *   `internal/`: Private application and library code.
    *   `pkg/`: Library code usable by external projects (avoid if possible).
*   **File Names**: Use `snake_case.go`. Test files must end in `_test.go`. OS-specific files should use suffixes like `_linux.go`.

## 3. Naming Conventions

*   **Exported Identifiers**: Use `PascalCase`.
*   **Unexported Identifiers**: Use `camelCase`.
*   **Variables**:
    *   Use short, descriptive names (e.g., `ctx`, `cfg`, `err`, `mu`).
    *   Use single-letter receivers for methods (e.g., `func (m *Manager) ...`).
*   **Interfaces**: Names should usually end in `er` (e.g., `Reader`, `Writer`), unless it's a specific service interface like `Service`.

## 4. Error Handling

*   **Package**: Use `grimm.is/flywall/internal/errors`. This is a drop-in replacement for the standard library `errors` package but adds structured capabilities.
*   **Structured Errors**: All application errors should be categorized using `errors.Kind` (e.g., `KindValidation`, `KindNotFound`, `KindInternal`, `KindPermission`).
*   **Check Errors**: Always check returned errors. `if err != nil { ... }`.
*   **Creation**:
    *   `errors.New(errors.KindValidation, "invalid input")`
    *   `errors.Errorf(errors.KindNotFound, "user %s not found", username)`
*   **Wrapping**: Use `errors.Wrap` or `errors.Wrapf` to add context and classify low-level errors.
    *   *Correct*: `return errors.Wrap(err, errors.KindInternal, "failed to save config")`
    *   *Incorrect*: `return fmt.Errorf("failed to save config: %w", err)`
*   **Is/As/Unwrap**: Use `errors.Is`, `errors.As`, and `errors.Unwrap` (identical to standard library logic).
*   **Attributes**: Attach key-value context to errors using `errors.Attr(err, "key", val)`. These are automatically logged by the structured logger.
*   **Sentinel Errors**: For errors that callers should check with `errors.Is()`, define package-level variables:
    ```go
    var ErrNotFound = errors.New("key not found")
    ```
    These do NOT require a `Kind` since they are used for identity comparison, not classification.
    *Example*: [state/store.go:198-203](file:///Users/ben/projects/flywall/internal/state/store.go#L198-L203)
*   **Must* Pattern**: Functions prefixed with `Must` may panic on error. Use ONLY for:
    - Compile-time constant initialization
    - `init()` functions where failure is unrecoverable
    - Test helpers via `t.Fatal()`
    *Never* use `Must*` functions in request handlers or runtime code paths.
*   **Panic**: Do not use `panic()` for control flow. Only panic on unrecoverable startup errors in `main` or `init`. Avoid panics in library code.

## 5. Logging

*   **Package**: Use `grimm.is/flywall/internal/logging`.
*   **Levels**:
    *   `Error`: System requires intervention or a request failed completely.
    *   `Warn`: Unexpected state that was handled, or potential issues.
    *   `Info`: Major lifecycle events (startup, shutdown, config change).
    *   `Debug`: Detailed internal state for troubleshooting.
*   **Structured Logging**: Use key-value pairs for context. **Never** use `fmt.Sprintf` inside a log call *at call sites*. This ensures structured log parsers can extract fields.
    *   *Correct*: `logger.Info("Starting service", "service", "dns", "port", 53)`
    *   *Incorrect*: `logger.Info(fmt.Sprintf("Starting dns service on port %d", 53))`
    (Library internals may use formatting where necessary, e.g., in a custom `Errorf` helper.)
*   **Error Logging**: Use `logger.WithError(err)` to automatically extract `Kind` and `Attributes` from structured errors.
*   **Scoped Loggers**: Use `logging.WithComponent("name")` or `logger.WithFields(...)` to create specialized loggers for specific components or requests.
    *Example*: [logger.go:112-119](file:///Users/ben/projects/flywall/internal/logging/logger.go#L112-L119)

## 6. Concurrency

*   **Context**: Pass `context.Context` as the first argument to functions that perform I/O or long-running operations.
*   **Synchronization**: Use `sync.Mutex` or `sync.RWMutex` to protect shared state. Group the mutex with the fields it protects in the struct.
*   **Goroutines**: Ensure all goroutines have a defined lifecycle and mechanism to stop (e.g., `<-ctx.Done()`).

## 7. Configuration

*   **Struct Tags**: Use `hcl` tags for HCL configuration and `json` tags for API serialization.
*   **Validation**: Validate configuration logic in a dedicated `Validate()` method or during loading.
*   **Immutability**: Treat configuration objects as immutable after loading where possible.

## 8. Testing

*   **Framework**: Use the standard `testing` package.
*   **Location**: Co-locate tests with the code they test (e.g., `service.go` and `service_test.go`).
*   **Table-Driven Tests**: Prefer table-driven tests for covering multiple scenarios.
    ```go
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{ ... }
    ```
*   **Helpers**: Use `grimm.is/flywall/internal/testutil` for common test helpers.
    - `RequireVM(t)`: skip tests requiring VM environment.
    - `RequireRoot(t)`: skip tests requiring root privileges.
    - `TempDir(t)`: create test-scoped temp directory.
*   **Time**: Use `grimm.is/flywall/internal/clock` and `clock.Now()` instead of `time.Now()` for any code that needs to be testable with simulated time.
    *Example*: [state/store.go:472](file:///Users/ben/projects/flywall/internal/state/store.go#L472)
*   **Sub-tests**: Use `t.Run()` for executing test cases.

## 9. Security & Privilege Separation

*   **Boundary Enforcement**: Respect the boundary between the unprivileged `api` and the privileged (root) `ctlplane`.
*   **Input Validation**: Validate all inputs at the boundary. Use `internal/validation` for identifiers (zone names, interface names) and paths.
*   **Sanitization**: Never pass unsanitized strings to shell commands or kernel configuration interfaces (e.g., `nftables`).

## 10. Hot Upgrades & State Handoff

*   **Socket Preservation**: Use `internal/upgrade.Manager` to preserve network sockets across process restarts.
*   **Inherit Pattern**: Prefer inheriting open connections via the `upgrade` manager over rebinding.
*   **Graceful Shutdown**: Always respect `context.Context` cancellation in server loops and long-running goroutines. Implement read/write deadlines to ensure timely exit.

## 11. Branding & Paths

*   **Branding Constants**: Use `internal/brand` for all product names, binary names, and default paths. Avoid hardcoding the project name.
*   **Directory Conventions**:
    *   State: `brand.GetStateDir()`
    *   Config: `brand.GetConfigDir()`
    *   Run/Sockets: `brand.GetRunDir()`
*   **Naming Consistency**: Use the branding identity for service names and socket files to ensure uniqueness.
*   **Identifier Normalization**: Prefer lowercase for system-level identifiers (zone names, interface names) to avoid case-sensitivity issues across different modules.

## 12. File I/O & Persistence

*   **Atomic Writes**: When persisting configuration or state files, always use the atomic write pattern (Write to `.tmp` file + `os.Rename`) to prevent data corruption during crashes or power loss.
    *Example*: [users.go:144-149](file:///Users/ben/projects/flywall/internal/auth/users.go#L139-L144) (Note: lines 139-144 specifically show the tmp file and rename logic).

## 13. Reference Patterns

| Pattern | Example | Description |
| :--- | :--- | :--- |
| `clock.Now()` | [state/store.go:472](file:///Users/ben/projects/flywall/internal/state/store.go#L472) | Decouples time for testing |
| Sentinel Errors | [state/store.go:198-203](file:///Users/ben/projects/flywall/internal/state/store.go#L198-203) | Standard error identity checks |
| Scoped Loggers | [logger.go:112-119](file:///Users/ben/projects/flywall/internal/logging/logger.go#L112-L119) | Adds `component` field to logs |
| Atomic Writes | [users.go:144-149](file:///Users/ben/projects/flywall/internal/auth/users.go#L139-L144) | Safe file persistence |

## 14. Commit Hygiene & Workflow

To maintain a clean and reliable repository, the following workflow rules are enforced:

*   **Conceptual Commits**: Commit code after every "conceptual milestone" (e.g., a single bug fix, a new component, a specific refactor). Avoid large "dirty" states with dozens of unrelated modified files.
*   **Test-Before-Commit**: All unit tests must pass before a commit is finalized.
*   **Acknowledgement of Failure**: If tests fail, the commit must be aborted, or the failure must be explicitly acknowledged and justified (rare). Use the pre-commit hook for enforcement.
*   **Atomic Commits**: Each commit should be a complete, buildable unit of work.
