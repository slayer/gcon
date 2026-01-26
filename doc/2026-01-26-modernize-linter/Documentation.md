# Linter Modernization

## Summary

Successfully modernized the golangci-lint configuration to use a comprehensive suite of modern linters for improved code quality, security, and maintainability.

## Changes Made

### 1. Updated `.golangci.yml` Configuration

Replaced the minimal configuration with a comprehensive setup including:

**Core Linters (6):**
- `errcheck` - Check for unchecked errors
- `govet` - Vet examines Go source code
- `ineffassign` - Detect ineffectual assignments
- `staticcheck` - Go static analysis (includes gosimple, stylecheck, unused)
- `unused` - Check for unused code

**Error Handling & Bug Detection (5):**
- `errorlint` - Error wrapping (Go 1.13+)
- `err113` - Error definition rules
- `errname` - Error naming conventions (ErrXxx, XxxError)
- `nilnil` - Checks for nil error with invalid value
- `nilerr` - Find code returning nil when error is not nil

**Performance (2):**
- `prealloc` - Slice preallocation opportunities
- `noctx` - HTTP requests without context.Context

**Code Quality (6):**
- `revive` - Fast, configurable linter (replaces golint)
- `gocognit` - Cognitive complexity (threshold: 20)
- `gocyclo` - Cyclomatic complexity (threshold: 15)
- `maintidx` - Maintainability index (threshold: 20)
- `nestif` - Deeply nested if statements (threshold: 5)
- `gocritic` - Extensive Go linter

**Style & Consistency (3):**
- `unconvert` - Unnecessary type conversions
- `unparam` - Unused function parameters
- `mirror` - Wrong mirror patterns of bytes/strings

**Security (1):**
- `gosec` - Security problems in source code

**Code Smells (4):**
- `goconst` - Repeated strings (3+ occurrences)
- `dupl` - Code duplication (100 token threshold)
- `misspell` - Misspelled English words
- `nakedret` - Naked returns in long functions (30+ lines)

**Modern Go Features (2):**
- `copyloopvar` - Loop variable copying (Go 1.22+)
- `intrange` - Integer range loop opportunities

### 2. Updated Makefile

- Changed `GOLANGCI_LINT_VERSION` from `v2.1.6` to `latest`
- Added `lint-fix` target for auto-fixing issues
- Updated help documentation

### 3. Exclusion Rules

Added targeted exclusions for:
- Test files (complexity, duplication, security checks)
- Bubble Tea Update methods (higher complexity allowed)
- Internal packages (package comments optional)
- Generated code (all linters disabled)
- Common false positives (Close() error checks, etc.)

## Current Status

### Linter Execution Results

Ran linter on entire codebase:
- **Total Issues Found:** 192
- **Tests:** All existing tests still pass

### Issue Breakdown

| Linter | Count | Description |
|--------|-------|-------------|
| `err113` | 50 | Error wrapping/definition improvements needed |
| `revive` | 50 | Code style and best practices |
| `gocognit` | 18 | High cognitive complexity |
| `gosec` | 16 | Security concerns |
| `goconst` | 15 | Repeated strings |
| `nestif` | 14 | Deeply nested conditionals |
| `dupl` | 9 | Code duplication |
| `prealloc` | 9 | Slice preallocation opportunities |
| `errorlint` | 3 | Error wrapping issues |
| `intrange` | 3 | Integer range loop opportunities |
| `nilnil` | 2 | Nil error with invalid value |
| `maintidx` | 1 | Low maintainability index |
| `gocritic` | 1 | Code improvement suggestion |
| `unparam` | 1 | Unused parameter |

## Common Issues and Fixes

### 1. Error Wrapping (`err113`, `errorlint`)

**Problem:** Direct error creation without wrapping
```go
// Bad
return errors.New("failed to connect")

// Good
return fmt.Errorf("failed to connect: %w", err)
```

### 2. Repeated Strings (`goconst`)

**Problem:** Magic strings repeated multiple times
```go
// Bad
if status == "RUNNING" { ... }
if status == "RUNNING" { ... }

// Good
const StatusRunning = "RUNNING"
if status == StatusRunning { ... }
```

### 3. Cognitive Complexity (`gocognit`)

**Problem:** Functions with high cognitive complexity
```go
// Bad - deeply nested logic
func Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
        case KeyMsg:
            switch msg.Key {
                case "a":
                    if condition {
                        if nested {
                            // ...
                        }
                    }
            }
    }
}

// Good - extract sub-functions
func Update(msg tea.Msg) tea.Cmd {
    return a.handleKeyMsg(msg)
}
```

### 4. Security Issues (`gosec`)

Common security warnings:
- G404: Weak random number generator (use `crypto/rand`)
- G402: TLS MinVersion too low
- G601: Implicit memory aliasing in for loop

### 5. Code Duplication (`dupl`)

**Problem:** Similar test structures or functions
```go
// Bad - duplicated test logic
func TestA() { ... similar code ... }
func TestB() { ... similar code ... }

// Good - extract helper function
func testHelper(t *testing.T, input string) { ... }
```

## Usage

### Running the Linter

```bash
# Run linter on all code
make lint

# Run linter with auto-fix (for fixable issues)
make lint-fix

# Run linter on specific directory
golangci-lint run ./internal/ui/...
```

### CI/CD Integration

The linter is configured for CI/CD:
- Timeout: 5 minutes
- Concurrency: 4
- Exit code 1 on issues
- Parallel execution allowed

## Benefits

1. **Better Error Handling**: Catches unchecked errors and improper error wrapping
2. **Security**: Identifies potential security vulnerabilities
3. **Maintainability**: Flags complex code that needs refactoring
4. **Performance**: Suggests optimization opportunities
5. **Consistency**: Enforces consistent code style across the project
6. **Modern Go**: Leverages Go 1.22+ features

## Next Steps

To address the 192 issues found:

1. **Quick Wins** (auto-fixable):
   - Run `make lint-fix` to auto-fix ~30-40% of issues
   - Issues like `unconvert`, `mirror`, some `revive` rules

2. **High Priority** (security & bugs):
   - Address `gosec` security warnings (16 issues)
   - Fix `nilnil` issues (2 issues)
   - Fix `errorlint` wrapping issues (3 issues)

3. **Medium Priority** (maintainability):
   - Refactor high complexity functions (`gocognit`: 18 issues)
   - Simplify deeply nested conditions (`nestif`: 14 issues)
   - Extract constants for repeated strings (`goconst`: 15 issues)

4. **Low Priority** (optimization):
   - Add slice preallocation (`prealloc`: 9 issues)
   - Use integer range loops (`intrange`: 3 issues)
   - Address code duplication (`dupl`: 9 issues)

5. **Error Handling Refactor**:
   - Create sentinel errors with `Err` prefix (`err113`: 50 issues)
   - Improve error messages and context

## Configuration Notes

### Bubble Tea Specific

The configuration includes special handling for Bubble Tea patterns:
- Higher complexity allowed in `Update()` methods
- Recognizes the idiomatic switch-heavy patterns
- Allows longer functions in model files

### Test Files

Tests are excluded from:
- Complexity checks (tests can be verbose)
- Duplication detection (similar test structure is OK)
- Security checks (not production code)
- Maintainability index (test clarity over brevity)

### Generated Code

All linters are disabled for:
- `.pb.go` files (protobuf)
- `.gen.go` files (code generation)

## Linter Configuration Philosophy

The configuration balances strictness with pragmatism:
- **Strict** on errors, security, and correctness
- **Moderate** on complexity (allows for real-world patterns)
- **Lenient** on style (focuses on important issues)
- **Contextual** (different rules for tests vs production)

## References

- [golangci-lint Documentation](https://golangci-lint.run/)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
