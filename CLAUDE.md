# Code Generation Guidelines

> **Also read**: `CLAUDE-GO.md` and `CLAUDE-REVIEW.md` and `CLAUDE-LOCAL.md` (if present) for complete guidance.

These guidelines apply to all code generation, regardless of language. For language-specific rules, see `CLAUDE-GO.md` or `CLAUDE-TS.md`.

---

## Code Style

### Follow idiomatic patterns

Write code that looks like it belongs in the language you're using. Follow the conventions, idioms, and best practices of that language and its ecosystem. When in doubt, look at how the standard library or well-regarded projects do it.

### Functions should tend to do one thing

Functions should have a reasonable scope with a clear purpose. If you find yourself using "and" to describe what a function does, consider splitting it. Keep functions short enough to understand at a glance—if it doesn't fit on one screen, it's probably doing too much.

```
// Don't: validateAndSaveUser()
// Do: validate(), then save() separately
```

### Prefer explicit over clever, but stay concise

Code is read more often than it's written. Prioritize clarity over brevity, but also avoid unnecessary verbosity—too much code is just as hard to read as too little. Find the balance where the code is immediately understandable without being bloated.

```
// Don't: obscure one-liners or overly verbose boilerplate
// Do: clear, straightforward code that says what it means
```

### Name things for what they represent

Names should describe the thing itself, not how it's used or where it came from.

```
// Don't: tempList, dataFromAPI, processedResult
// Do: users, orders, validatedItems
```

### Avoid code duplication

Don't copy-paste code. Extract shared logic into reusable functions, utilities, or base classes. Prefer extension and composition over duplication. When you see repeated patterns, consolidate them.

### Be mindful of file organization

Trend away from multi-thousand line files. If a file is growing large, look for natural boundaries to split it. Related code should be grouped together, but massive files become hard to navigate and maintain.

### Be wary of accidental mutation

Understand whether your language passes by value or reference. When working with objects or collections, be explicit about whether you're modifying the original or creating a copy. Accidental mutation causes subtle bugs.

---

## Architecture

### Clear separation of concerns

Each module/package should have a clear, singular purpose. Business logic shouldn't know about HTTP. Database access shouldn't know about presentation.

### Dependencies flow inward

External layers (HTTP handlers, CLI) depend on internal layers (business logic), which depend on core domain. Never the reverse.

```
handlers → services → domain
    ↓         ↓
  repos    (no outward deps)
```

### Interfaces at boundaries

Define interfaces where modules meet. This allows swapping implementations (real DB vs mock) and makes testing easier.

### Avoid circular dependencies

If A imports B and B imports A, extract shared code to a new package C that both can import.

### Consider structural impact

When making changes, consider how they affect the overall codebase structure. If a change is making the code messier or harder to navigate, surface this to the user. Suggest refactoring instead of polluting the existing structure with workarounds.

---

## Testing

### Every feature needs a test plan

Before writing code, know how you'll verify it works. This could be unit tests, integration tests, or manual verification steps—but decide upfront.

### Test behavior, not implementation

Tests should verify what the code does, not how it does it. If you refactor internals without changing behavior, tests shouldn't break.

```
// Don't: assert that a specific internal method was called 3 times
// Do: assert that the output is correct for given inputs
```

### Use descriptive test names

A test name should explain the scenario being tested. When a test fails, the name should tell you what broke.

```
// Don't: TestUser, TestValidation
// Do: TestCreateUser_WithDuplicateEmail_ReturnsConflictError
```

### Prefer integration tests for I/O-heavy code

For code that primarily shuffles data between systems (APIs, databases), integration tests provide more confidence than unit tests with mocks.

---

## Error Handling

### Errors should provide context

When an error occurs, include what operation failed and relevant identifiers. Raw errors like "connection refused" don't tell you which service or what operation.

```
// Don't: return err
// Do: return error wrapping with "failed to fetch user {id}: {err}"
```

### Don't swallow errors silently

Every error should be either:
1. Handled and recovered from
2. Logged with context
3. Returned/propagated to the caller

Never ignore an error without explicit justification.

### Handle errors at the appropriate level

Don't handle errors too early (before you have context to handle them properly) or too late (after the context is lost). Handle where you can actually do something meaningful.

### Distinguish recoverable vs unrecoverable errors

Some errors are expected (user not found, validation failed) and should be handled gracefully. Others indicate bugs or system failures and should fail loudly.

### Be mindful of log levels and spam

Use appropriate log levels: errors for actual problems, info for significant events, debug for detailed diagnostics. Avoid logging the same information repeatedly in hot paths. Include useful details, but don't spam logs with noise that makes it hard to find what matters.

---

## Security

### Validate all external input

Anything from outside the system boundary (user input, API responses, file contents) must be validated before use. Trust nothing external.

### Never log sensitive data

Secrets, tokens, passwords, and PII should never appear in logs. When in doubt, don't log it. Mask or redact if you must reference it.

```
// Don't: log.Info("authenticating user", "token", token)
// Do: log.Info("authenticating user", "user_id", userID)
```

### Use parameterized queries

Never concatenate user input into queries. Always use parameterized queries or prepared statements to prevent injection attacks.

```
// Don't: query := "SELECT * FROM users WHERE id = " + userID
// Do: query with parameterized placeholder and userID as parameter
```

### Principle of least privilege

Request only the permissions you need. Don't use admin credentials when read-only access suffices. Scope access tokens narrowly.

---

## Working on Large Changes

### Split large changes into reviewable chunks

If a change is approaching ~1000 lines and there's more to do, ask about splitting the work. A good pattern:
1. First PR: structural changes (interfaces, method stubs, abstractions)
2. Second PR: fill in the implementation

This keeps each PR human-reviewable and logically organized. Large monolithic changes are hard to review and risky to merge.
