---
id: testing-standards
kind: instruction
version: 1
description: Testing requirements, patterns, and verification workflow
preservation: required
scope:
  fileTypes:
    - ".go"
appliesTo:
  targets: ["*"]
  profiles: ["*"]
---

# Testing Standards

## Required Test Types

Every package must have:
- **Unit tests** — Table-driven, in `<package>_test` package for exported API
- **Benchmark tests** — Both simple and complex simulations
- **Integration tests** — End-to-end pipeline flows in `tests/integration/`

## Table-Driven Tests

Always use table-driven tests for pipeline stages and renderers:

```go
tests := []struct {
    name    string
    input   any
    want    any
    wantErr bool
}{
    {"valid input", ..., ..., false},
    {"missing field", ..., nil, true},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { ... })
}
```

## Golden Tests

Use golden file testing for renderer output. Golden files live alongside test files in `golden/` directories.

## Verification Workflow

Before stating work is complete:

1. Run `make test` — unit tests with race detection (`-race -short`)
2. Run `make lint` — golangci-lint
3. Run `make vet` — go vet

## Test Failure Policy

- **Do not simplify tests** just because they fail — check both code and test.
- If a test is wrong, fix the test. If code is wrong, fix the code.
- Use `t.Helper()` in test helper functions.
- Use `t.Parallel()` where safe.

## Fixture & Testdata

- Test fixtures go in `fixtures/` subdirectories alongside tests
- Integration test fixtures in `tests/integration/fixtures/`
- Golden files in `golden/` subdirectories
