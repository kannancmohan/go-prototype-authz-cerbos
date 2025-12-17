# Go Style Guide

Best practices and standards for Go development.

## Quick Reference

| Rule | Setting |
|------|---------|
| Line length | 160 characters max |
| Function length | 120 lines, 60 statements max |
| Cyclomatic complexity | 15 max |
| Linting command | `golangci-lint run` |
| Auto-fix issues | `golangci-lint run --fix` |

## Table of Contents

1. [Code Formatting](#code-formatting)
2. [Project Structure](#project-structure)
3. [Naming Conventions](#naming-conventions)
4. [Error Handling](#error-handling)
5. [Testing](#testing)
6. [Documentation](#documentation)
7. [Security](#security)
8. [Performance](#performance)
9. [Dependencies](#dependencies)

## Code Formatting

### Linting

Use [`golangci-lint`](https://golangci-lint.run/) with [`.golangci.yml`](.golangci.yml) configuration.

Key features:
- **Formatters**: `gofumpt`, `gci`, `gofmt`, `golines`
- **80+ Linters**: Security, style, complexity, bugs, performance
- **Consistent Rules**: All team members use identical settings

### Import Organization

Imports are automatically organized into three sections:

```go
import (
    // Standard library
    "context"
    "fmt"
    
    // Third-party
    "github.com/some/package"
    
    // Internal/project
    "github.com/kannancmohan/go-prototype-authz-cerbos/internal/pkg"
)
```

### Code Structure

```go
// Good: Clear, concise function with early returns
func processRequest(ctx context.Context, req *Request) (*Response, error) {
    if req == nil {
        return nil, errors.New("request cannot be nil")
    }
    
    if err := req.Validate(); err != nil {
        return nil, fmt.Errorf("invalid request: %w", err)
    }
    
    result, err := doProcessing(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("processing failed: %w", err)
    }
    
    return result, nil
}

// Bad: Deeply nested
func processRequestBad(req *Request) (*Response, error) {
    if req != nil {
        if err := req.Validate(); err == nil {
            result, err := doProcessing(req)
            if err != nil {
                return nil, err
            }
            return result, nil
        } else {
            return nil, err
        }
    } else {
        return nil, errors.New("request cannot be nil")
    }
}
```

## Project Structure

### Standard Layout

```
.
├── cmd/              # Application entry points
│   └── app/
│       └── main.go
├── internal/         # Private application code
│   ├── domain/       # Business logic
│   ├── ports/        # Interface definitions
│   ├── adapters/     # Implementation of ports
│   └── pkg/          # Shared packages
├── api/              # API definitions
├── configs/          # Configuration files
├── go.mod            # Go module file
└── .golangci.yml     # Linting configuration
```

### File Naming

- Use `snake_case.go` for file names
- Use `*_test.go` for test files
- Use `*_integration_test.go` for integration tests

## Naming Conventions

### General Rules

- Use `camelCase` for variables, functions, and methods
- Use `PascalCase` for exported types, functions, and constants
- Use `UPPER_SNAKE_CASE` for constants
- Be descriptive but concise (avoid abbreviations unless they're widely known)

### Package Names

- Use short, lowercase, single-word names
- Avoid underscores and mixed caps
- Good: `auth`, `config`, `handler`
- Bad: `AuthPackage`, `config_manager`, `HTTP_handlers`

### Interface Names

- Use noun forms (e.g., `Reader`, `Writer`, `Repository`)
- Use `-er` suffix for single-method interfaces
- For multiple methods, use descriptive names

```go
// Good
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id string) (*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
}

// Good
type Stringer interface {
    String() string
}
```

### Function and Method Names

- Use descriptive verb-noun combinations
- Avoid stuttering with package names

```go
// Good
func (s *UserService) CreateUser(ctx context.Context, user *User) error

// Bad (stuttering)
func (s *UserService) CreateUserInUserService(ctx context.Context, user *User) error
```

### Variable Names

- Use short names for short-lived variables
- Use `ctx` for context.Context, `err` for errors
- Use `i`, `j`, `k` for loop indices

```go
// Good
for i, user := range users {
    if user.ID == id {
        return user, nil
    }
}

// Bad
for userIndex, userElement := range users {
    if userElement.ID == userID {
        return userElement, nil
    }
}
```

## Error Handling

### Error Creation and Wrapping

- Use `errors.New()` for simple errors
- Use `fmt.Errorf()` with `%w` for error wrapping
- Always add context to errors

```go
var ErrUserNotFound = errors.New("user not found")

func (r *UserRepository) GetByID(ctx context.Context, id string) (*User, error) {
    user, err := r.db.Query(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get user %s: %w", id, err)
    }
    if user == nil {
        return nil, ErrUserNotFound
    }
    return user, nil
}
```

### Sentinel Errors

- Define package-level sentinel errors for common cases
- Use `errors.Is()` for error comparison
- Use `errors.As()` for error type assertions

```go
var (
    ErrUserNotFound    = errors.New("user not found")
    ErrInvalidPassword = errors.New("invalid password")
)

func handleLogin(username, password string) error {
    user, err := getUser(username)
    if err != nil {
        if errors.Is(err, ErrUserNotFound) {
            return fmt.Errorf("authentication failed: %w", err)
        }
        return fmt.Errorf("login failed: %w", err)
    }
    // ...
}
```

## Testing

### Test Structure

- Place tests in the same package as the code being tested
- Use table-driven tests for multiple scenarios
- Use descriptive test names: `Test<FunctionName>_<Scenario>_<ExpectedResult>`

```go
func TestUserService_CreateUser(t *testing.T) {
    tests := []struct {
        name    string
        user    *User
        wantErr bool
    }{
        {
            name:    "valid user",
            user:    &User{Name: "John", Email: "john@example.com"},
            wantErr: false,
        },
        {
            name:    "empty name",
            user:    &User{Name: "", Email: "john@example.com"},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            svc := NewUserService(mockRepo)
            err := svc.CreateUser(context.Background(), tt.user)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("CreateUser() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Test Helpers

- Use test helpers to reduce duplication
- Accept `testing.TB` interface for flexibility
- Clean up resources in tests

```go
func setupTestDB(t *testing.T) *sql.DB {
    t.Helper()
    
    db, err := sql.Open("postgres", testDSN)
    if err != nil {
        t.Fatalf("failed to connect to test database: %v", err)
    }
    
    t.Cleanup(func() {
        db.Close()
    })
    
    return db
}
```

### Mocking

- Use interfaces to enable mocking
- Generate mocks using tools like `mockery` or `gomock`

```go
//go:generate mockery --name=UserRepository

type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id string) (*User, error)
}
```

## Documentation

### Package Documentation

Every package must have a package comment:

```go
// Package userservice provides user management functionality.
package userservice
```

### Function Documentation

Document all exported functions:

```go
// CreateUser creates a new user account.
func (s *UserService) CreateUser(ctx context.Context, user *User) error {
    // Implementation
}
```

### Code Comments

Explain "why", not "what":

```go
// Good: Explains why
// Using sync.Pool to reduce GC pressure
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 0, 1024)
    },
}

// Bad: Obvious comment
// Increment counter by 1
counter++
```

## Security

### Input Validation

- Validate all external inputs
- Use whitelisting over blacklisting

```go
func (r *CreateUserRequest) Validate() error {
    if r.Email == "" {
        return errors.New("email is required")
    }
    if !isValidEmail(r.Email) {
        return errors.New("invalid email format")
    }
    return nil
}
```

### Sensitive Data

- Never log sensitive information
- Use environment variables for secrets

```go
// Bad: Logging sensitive data
log.Printf("User login: email=%s, password=%s", email, password)

// Good: Logging only non-sensitive information
log.Printf("User login attempt: email=%s", email)
```

### Cryptography

- Use standard library crypto packages
- Use secure random number generators

```go
func generateToken() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("failed to generate token: %w", err)
    }
    return base64.URLEncoding.EncodeToString(b), nil
}
```

## Performance

### Memory Management

- Use object pools for frequently allocated objects
- Avoid unnecessary allocations in hot paths

```go
var responseBufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
    buf := responseBufferPool.Get().(*bytes.Buffer)
    defer responseBufferPool.Put(buf)
    buf.Reset()
    // Use buffer...
}
```

### Concurrency

- Use goroutines for I/O-bound operations
- Use worker pools for CPU-bound operations
- Always use context for cancellation

```go
func processItems(ctx context.Context, items []Item) error {
    const numWorkers = 4
    itemChan := make(chan Item, len(items))
    
    var wg sync.WaitGroup
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for item := range itemChan {
                if err := processItem(ctx, item); err != nil {
                    log.Printf("Failed to process item: %v", err)
                }
            }
        }()
    }
    
    for _, item := range items {
        select {
        case itemChan <- item:
        case <-ctx.Done():
            close(itemChan)
            return ctx.Err()
        }
    }
    
    close(itemChan)
    wg.Wait()
    return nil
}
```

### Database Optimization

- Use connection pooling
- Use prepared statements for repeated queries

```go
func (r *UserRepository) GetByID(ctx context.Context, id string) (*User, error) {
    const query = `SELECT id, name, email FROM users WHERE id = $1`
    
    stmt, err := r.db.PrepareContext(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("failed to prepare statement: %w", err)
    }
    defer stmt.Close()
    
    var user User
    err = stmt.QueryRowContext(ctx, id).Scan(&user.ID, &user.Name, &user.Email)
    if err != nil {
        return nil, fmt.Errorf("failed to query user: %w", err)
    }
    
    return &user, nil
}
```

## Dependencies

### Module Management

- Use Go modules
- Keep dependencies updated
- Use minimal dependency versions

```bash
# Initialize module
go mod init github.com/kannancmohan/go-prototype-authz-cerbos

# Tidy dependencies
go mod tidy

# Verify dependencies
go mod verify
```

### Version Pinning

- Use exact versions in `go.mod`
- Commit `go.sum` to version control

```go
require (
    github.com/some/package v1.2.3
    github.com/another/package v0.4.5
)
```

## Additional Resources

- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- [Google Go Style Guide](https://google.github.io/styleguide/go/)