# Go-Specific Guidelines

These rules supplement the generic guidelines in `CLAUDE.md`. Follow both.

---

## Code Style

### Formatting and linting

Run `gofmt` and `golangci-lint` before committing. The linter config is authoritative—if it passes, the style is acceptable.

### Variable naming

Use short names in small scopes, descriptive names in larger scopes.

```go
// In a 3-line function, this is fine:
for i, v := range items { ... }

// In a 50-line function, be explicit:
for itemIndex, currentItem := range inventoryItems { ... }
```

### Receiver names

Use short, consistent receiver names. Never use `this` or `self`.

```go
// Do:
func (s *Service) Create(ctx context.Context) error { ... }

// Don't:
func (this *Service) Create(ctx context.Context) error { ... }
func (service *Service) Create(ctx context.Context) error { ... }
```

---

## Architecture

### Package organization

- Package by feature/domain, not by layer
- Use `internal/` for packages that shouldn't be imported externally
- Keep `cmd/` minimal—wire things up and call `Run()`

```
internal/
  user/           # User domain (service, repo, handlers)
  billing/        # Billing domain
  notification/   # Notification domain
pkg/              # Reusable utilities
cmd/              # Entry points only
```

### Interface placement

Interfaces belong with the consumer, not the implementer. Define interfaces where you need them.

```go
// In the consumer package:
type UserRepository interface {
    Get(ctx context.Context, id string) (*User, error)
}

// Not in the repository package alongside the implementation
```

### Dependency injection

Pass dependencies explicitly. Prefer constructor injection over global state.

```go
// Do:
func NewService(repo Repository, logger *zap.Logger) *Service {
    return &Service{repo: repo, logger: logger}
}

// Don't:
var globalRepo Repository // package-level state
```

---

## Error Handling

### Always wrap errors with context

Use `fmt.Errorf` with `%w` or your error package's wrap function. Include what operation failed.

```go
// Do:
if err != nil {
    return fmt.Errorf("failed to create user %s: %w", userID, err)
}

// Don't:
if err != nil {
    return err
}
```

### Check errors immediately

Handle errors right after the call that might produce them. Don't defer error checking.

```go
// Do:
result, err := doSomething()
if err != nil {
    return err
}
// use result

// Don't:
result, err := doSomething()
// ... other code ...
if err != nil { // easy to forget or misplace
    return err
}
```

### Use sentinel errors for expected cases

When callers need to handle specific error types, use sentinel errors or typed errors.

```go
var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")

// Callers can check:
if errors.Is(err, ErrNotFound) {
    // handle not found
}
```

### Don't panic for normal errors

Reserve `panic` for truly unrecoverable situations (programmer errors, invariant violations). Normal errors should be returned.

```go
// Do:
func ParseConfig(path string) (*Config, error) {
    // return error if file doesn't exist
}

// Don't:
func MustParseConfig(path string) *Config {
    // panic if file doesn't exist
}
```

---

## Testing

### Table-driven tests

Use table-driven tests when testing multiple cases of the same behavior.

```go
func TestValidateEmail(t *testing.T) {
    tests := []struct {
        name    string
        email   string
        wantErr bool
    }{
        {"valid email", "user@example.com", false},
        {"missing @", "userexample.com", true},
        {"empty", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateEmail(tt.email)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
            }
        })
    }
}
```

### Test naming convention

Use descriptive names that explain the scenario:

```go
func Test_CreateUser_WithDuplicateEmail_ReturnsConflictError(t *testing.T)
func Test_GetUser_WhenNotFound_ReturnsNil(t *testing.T)
```

### Use t.Helper() in test helpers

Mark helper functions so test failures report the correct line.

```go
func assertNoError(t *testing.T, err error) {
    t.Helper()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}
```

### Be consistent with assertion libraries

Pick one (testify/assert, stdlib, etc.) and stick with it across the codebase.

---

## Concurrency

### Context cancellation

Always respect context cancellation. Check `ctx.Done()` in long-running operations.

```go
func ProcessItems(ctx context.Context, items []Item) error {
    for _, item := range items {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if err := process(item); err != nil {
                return err
            }
        }
    }
    return nil
}
```

### Use errgroup for parallel operations

When running multiple operations in parallel that can fail, use `errgroup`.

```go
g, ctx := errgroup.WithContext(ctx)

for _, item := range items {
    item := item // capture loop variable
    g.Go(func() error {
        return processItem(ctx, item)
    })
}

if err := g.Wait(); err != nil {
    return err
}
```

### Document goroutine ownership

When spawning goroutines, document who owns them and how they're cleaned up.

```go
// Start starts the background processor.
// The goroutine runs until ctx is cancelled.
// Caller must call Stop() or cancel the context to clean up.
func (p *Processor) Start(ctx context.Context) {
    go p.run(ctx)
}
```

### Prefer channels for communication, mutexes for state

Use channels to coordinate between goroutines. Use mutexes to protect shared state.

```go
// Channel for work coordination:
jobs := make(chan Job)

// Mutex for shared state:
var mu sync.Mutex
var count int
```

---

## Observability

### Prefer spans over log lines

Use tracing spans instead of log lines for observability, especially at info level. Spans provide context, timing, and can be correlated across services. Reserve logging for errors (which should still include useful details).

```go
// Do: Start a span for the operation
ctx, span := tracer.Start(ctx, "CreateUser")
defer span.End()

span.SetAttributes(attribute.String("user.email", email))

// Don't: Log every operation
log.Info("creating user", "email", email)
log.Info("user created", "id", userID)
```

### Ensure methods have spans for tracing

Important methods—especially service boundaries, database operations, and external API calls—should have spans so traces are useful for debugging.

```go
func (s *Service) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
    ctx, span := tracer.Start(ctx, "UserService.CreateUser")
    defer span.End()

    // ... implementation
}
```

### Errors can be logged in addition to spans

When an error occurs, it's fine to log it with context in addition to recording it on the span. Include useful details like identifiers and operation context.

```go
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, "failed to create user")
    log.Error("failed to create user", "email", email, "error", err)
    return nil, err
}
```
