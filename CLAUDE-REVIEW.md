# Code Review Guidelines

Use this checklist when reviewing pull requests. These guidelines apply regardless of whether the code was written by a human or AI.

For language-specific review points, see `CLAUDE-GO.md` or `CLAUDE-TS.md`.

---

## Completeness

- [ ] **Does the PR actually solve what it claims?** Read the PR description and linked issue, then verify the code addresses it
- [ ] **Are edge cases handled?** Empty inputs, null values, boundary conditions, concurrent access
- [ ] **Are error paths covered?** What happens when things fail? Network errors, invalid data, missing resources
- [ ] **Is the acceptance criteria met?** If there's a spec or requirements, check each item

### Red flags
- PR title says "Add feature X" but feature X is only partially implemented
- Happy path works but error cases return generic errors or crash
- No consideration of what happens with empty/null/zero inputs

---

## Regression Risk

- [ ] **Could this break existing functionality?** Look for changes to shared code, interfaces, or data structures
- [ ] **Are there integration points that might be affected?** APIs, database schemas, message formats
- [ ] **Have dependent systems been considered?** Other services that call this code, downstream consumers
- [ ] **Are there breaking changes?** API contract changes, removed fields, changed behavior

### Red flags
- Changes to widely-used utility functions without updating all callers
- Database migrations that could break existing queries
- Renamed or removed API fields without versioning

---

## Test Coverage

- [ ] **Are there tests for the new/changed behavior?** Not just "tests exist" but "tests cover this change"
- [ ] **Do tests actually validate the change?** Tests should fail if the feature breaks, not just execute the code
- [ ] **Are failure scenarios tested?** Error handling, edge cases, invalid inputs
- [ ] **Is the test plan clear?** Either documented or obvious from test names

### Red flags
- Tests that only check the happy path
- Tests that mock so much they don't test real behavior
- "Tested manually" without explanation of what was tested
- Tests that would pass even if the feature was completely broken

---

## Scope

- [ ] **Is the PR focused on a single concern?** One feature, one bug fix, or one refactor—not all three
- [ ] **Should this be split?** Large PRs are harder to review and riskier to merge
- [ ] **Are there unrelated changes?** Drive-by refactors, formatting changes, unrelated fixes
- [ ] **Is the PR a reasonable size?** Rule of thumb: if it takes more than 30 minutes to review, it's probably too big

### Red flags
- PR description lists multiple unrelated items
- Changes span many unrelated files
- "While I was in there, I also..." changes
- 1000+ line PRs (unless it's mostly generated code or tests)

---

## Consistency

- [ ] **Does this match existing patterns?** Look at how similar things are done elsewhere in the codebase
- [ ] **Are naming conventions followed?** Variables, functions, files, packages
- [ ] **Is the code style consistent?** With surrounding code and project standards
- [ ] **Are established libraries used?** Don't reinvent what already exists in the codebase

### Red flags
- New patterns introduced when existing ones would work
- Different naming style than the rest of the codebase
- Custom implementation of something that exists in a shared utility
- Third-party library added when a standard library solution exists

---

## Reviewability

- [ ] **Is the PR description clear?** Explains what changed and why, not just "fixes bug"
- [ ] **Are commits logically organized?** Each commit should be a coherent unit (though not required)
- [ ] **Is complex logic explained?** Either in code comments or PR description
- [ ] **Can you understand it without asking?** If you need a walkthrough, the code may be too complex

### Red flags
- Empty or minimal PR description
- "WIP" or "misc fixes" without details
- Complex algorithms with no explanation
- Magic numbers or obscure logic without comments

---

## How to Give Feedback

### Be specific
```
// Don't: "This could be improved"
// Do: "This query runs N+1 times in a loop. Consider batching the lookups."
```

### Explain why
```
// Don't: "Use X instead of Y"
// Do: "Use X instead of Y because X handles the edge case where Z"
```

### Distinguish severity
- **Blocking**: Must fix before merge (bugs, security issues, breaking changes)
- **Should fix**: Important but could be a follow-up (test coverage gaps, minor issues)
- **Nit**: Style preferences, suggestions, take-it-or-leave-it
