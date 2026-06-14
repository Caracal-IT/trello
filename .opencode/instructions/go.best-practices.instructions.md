---
name: "go-best-practices"
description: "Go coding, testing, and performance standards for this repository."
applyTo: "**/*.go"
---

# Go Best Practices

## Code Quality
- Keep packages focused and small; avoid circular dependencies.
- Prefer explicit interfaces where they improve testability.
- Return wrapped errors with enough context for debugging.

## Testing Requirements
- Add comprehensive automated tests for all new logic.
- Ensure all tests pass before merging changes.
- Add benchmarks for performance-sensitive paths.

## Documentation Requirements
- Add GoDoc comments for exported packages, types, functions, and methods.
- Keep examples aligned with real usage and test coverage.
