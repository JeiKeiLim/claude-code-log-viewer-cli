# Story 7.1: OAuth Credential Access

Status: done

## Story

As a **cclv user with a Claude Code subscription**,
I want **cclv to read my existing Claude Code credentials**,
So that **I don't need to log in separately**.

## Acceptance Criteria

1. **AC-1: macOS Keychain Access**
   - Given I am on macOS and logged into Claude Code
   - When cclv needs credentials
   - Then it retrieves the OAuth token from Keychain (`Claude Code-credentials`)

2. **AC-2: Linux File-Based Access**
   - Given I am on Linux and logged into Claude Code
   - When cclv needs credentials
   - Then it reads the OAuth token from `~/.claude/.credentials.json`

3. **AC-3: Environment Variable Override (Linux/Windows)**
   - Given `CLAUDE_CODE_OAUTH_TOKEN` env var is set on Linux or Windows
   - When cclv needs credentials
   - Then it uses the env var value (takes precedence over file-based access)
   - Note: macOS always uses Keychain (no env var override)

4. **AC-4: Graceful Error Handling**
   - Given no credentials are found
   - When cclv tries to fetch credentials
   - Then it returns a clear error without crashing

5. **AC-5: Windows File-Based Access**
   - Given I am on Windows and logged into Claude Code
   - When cclv needs credentials
   - Then it reads the OAuth token from `%USERPROFILE%\.claude\.credentials.json`
   - Note: Use `os.UserHomeDir()` which returns `%USERPROFILE%` on Windows

6. **AC-6: Token Validation**
   - Given credentials are retrieved
   - When the access token is empty or expired
   - Then it returns `ErrInvalidCredentials` or `ErrTokenExpired`

## Tasks / Subtasks

- [x] Task 1: Create `internal/usage/` package structure (AC: #1-6)
  - [x] Subtask 1.1: Create `internal/usage/credentials.go` with package declaration
  - [x] Subtask 1.2: Create `internal/usage/types.go` with `Credentials` struct and sentinel errors
  - [x] Subtask 1.3: Create `internal/usage/credentials_test.go` scaffold

- [x] Task 2: Implement Credentials struct and JSON parsing (AC: #2, #5, #6)
  - [x] Subtask 2.1: Define `Credentials` struct matching Claude Code's JSON format
  - [x] Subtask 2.2: Implement `getTokenFromFile()` with validation (non-empty token check)
  - [x] Subtask 2.3: Implement `isTokenExpired()` helper using `expiresAt` field
  - [x] Subtask 2.4: Write unit tests for JSON parsing (valid, malformed, missing fields, empty token, expired)

- [x] Task 3: Implement macOS Keychain access (AC: #1)
  - [x] Subtask 3.1: Implement `getTokenFromKeychain()` with 5-second context timeout
  - [x] Subtask 3.2: Handle Keychain error codes (44=not found, permission denied, timeout)
  - [x] Subtask 3.3: Write integration test with `//go:build darwin` build tag

- [x] Task 4: Implement environment variable override (AC: #3)
  - [x] Subtask 4.1: Check `CLAUDE_CODE_OAUTH_TOKEN` env var before file-based access (Linux/Windows only)
  - [x] Subtask 4.2: Write unit tests for env var precedence

- [x] Task 5: Implement main `GetOAuthToken()` function (AC: #1-6)
  - [x] Subtask 5.1: Add platform detection using `runtime.GOOS`
  - [x] Subtask 5.2: Route to appropriate credential source per platform
  - [x] Subtask 5.3: Validate token before returning (non-empty, not expired)
  - [x] Subtask 5.4: Ensure 90%+ coverage via table-driven tests for all paths

## Dev Notes

### Critical Implementation Details

**Credential JSON Format:**
```json
{
  "claudeAiOauth": {
    "accessToken": "sk-ant-oaut...",
    "refreshToken": "...",
    "expiresAt": "2026-01-25T12:00:00Z"
  }
}
```

**macOS Keychain Command:**
```bash
security find-generic-password -s "Claude Code-credentials" -w
```
- Returns raw JSON on stdout
- Exit code 0: success
- Exit code 44 (errSecItemNotFound): credential not found
- Exit code 45 (errSecDuplicateItem): should not occur for read
- Exit code non-zero + "The user name or passphrase you entered is not correct": access denied
- **CRITICAL**: Use `exec.CommandContext` with 5-second timeout to prevent hang if Keychain prompts

**Credential Priority Order (Platform-Specific):**
```
macOS:
  1. Keychain only (no env var override)

Linux/Windows:
  1. CLAUDE_CODE_OAUTH_TOKEN env var (if set)
  2. ~/.claude/.credentials.json (or %USERPROFILE%\.claude\.credentials.json)
```

### Sentinel Errors (Required)

```go
var (
    ErrNoCredentials      = errors.New("no Claude Code credentials found")
    ErrKeychainNotFound   = errors.New("credential not found in Keychain")
    ErrKeychainTimeout    = errors.New("Keychain access timed out")
    ErrInvalidCredentials = errors.New("invalid credentials format")
    ErrTokenExpired       = errors.New("OAuth token has expired - run 'claude' to re-login")
    ErrEmptyToken         = errors.New("credentials file exists but accessToken is empty")
)
```

### Token Validation Logic

```go
func isTokenExpired(expiresAt string) bool {
    if expiresAt == "" {
        return false // No expiry = assume valid
    }
    t, err := time.Parse(time.RFC3339, expiresAt)
    if err != nil {
        return false // Parse error = assume valid, let API reject
    }
    return time.Now().After(t)
}
```

### File Path Construction

```go
// Cross-platform path - works on all OSes
home, err := os.UserHomeDir()
// On Windows: returns %USERPROFILE% (e.g., C:\Users\username)
// On Linux/macOS: returns $HOME (e.g., /home/username)
credPath := filepath.Join(home, ".claude", ".credentials.json")
```

### Package Structure

```
internal/usage/
├── credentials.go      # GetOAuthToken() and platform-specific helpers
├── credentials_test.go # Table-driven unit tests
└── types.go           # Credentials struct + sentinel errors
```

### Required Imports (stdlib only)

```go
import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "time"
)
```

### Testing Strategy

**Unit Tests (all platforms):**
- JSON parsing: valid, malformed, missing `claudeAiOauth`, empty `accessToken`
- Token expiration: valid, expired, missing `expiresAt`, malformed date
- Env var precedence (mock with `t.Setenv`)
- File not found handling

**Integration Tests (platform-specific):**
```go
//go:build darwin

func TestKeychainAccess(t *testing.T) {
    // Only runs on macOS CI/local
}
```

```go
//go:build linux

func TestFileBasedAccess(t *testing.T) {
    // Only runs on Linux CI/local
}
```

### Critical Rules (from project-context.md)

- NO EMOJI in any output or code comments
- Use `make test` not raw `go test`
- Table-driven tests required
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Follow `internal/token/service.go` soft-fail pattern if needed

### Expected Commit Format

```
feat: add OAuth credential access for usage monitoring (Story 7.1)

Implements cross-platform credential retrieval:
- macOS: Keychain via security command with timeout
- Linux/Windows: ~/.claude/.credentials.json
- Environment variable override for Linux/Windows

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

### References

- [Source: _bmad-output/planning-artifacts/epics-phase4.md#Story-7.1]
- [Source: _bmad-output/planning-artifacts/research/technical-claude-code-usage-limits-research-2026-01-20.md#4-Credential-Access-Methods]
- [Source: _bmad-output/project-context.md] - Critical rules and patterns
- [Source: internal/token/service.go] - Reference implementation for new package pattern

### Anti-Patterns to Avoid

1. **DO NOT** add env var override for macOS - Keychain is the only source
2. **DO NOT** use `time.Sleep` for Keychain timeout - use `context.WithTimeout`
3. **DO NOT** return empty string on error - always return sentinel error
4. **DO NOT** skip token validation after successful retrieval
5. **DO NOT** use `%USERPROFILE%` string literal - use `os.UserHomeDir()`

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - Implementation completed without issues.

### Completion Notes List

1. Created `internal/usage/` package with cross-platform OAuth credential retrieval
2. Implemented all sentinel errors as specified: `ErrNoCredentials`, `ErrKeychainNotFound`, `ErrKeychainTimeout`, `ErrInvalidCredentials`, `ErrTokenExpired`, `ErrEmptyToken`
3. macOS Keychain access uses `security find-generic-password` with 5-second context timeout
4. Linux/Windows support env var `CLAUDE_CODE_OAUTH_TOKEN` with file fallback
5. Token validation checks for empty tokens and expired tokens (RFC3339 parsing)
6. Added testable command executor pattern for Keychain testing
7. All tests pass with 95.6% coverage (exceeds 90% requirement)
8. Table-driven tests used throughout following project conventions
9. macOS-specific integration tests with `//go:build darwin` build tag

### File List

- `internal/usage/types.go` - Credentials struct, OAuthToken struct, sentinel errors
- `internal/usage/credentials.go` - GetOAuthToken(), platform-specific credential retrieval
- `internal/usage/credentials_test.go` - Unit tests for all paths
- `internal/usage/credentials_darwin_test.go` - macOS Keychain integration tests
