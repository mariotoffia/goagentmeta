# Example 01: First Instruction

**Level**: 🟢 Beginner  
**Goal**: Create a single always-on instruction for a Go project — the "hello world" of goagentmeta.

---

## What You'll Build

A single instruction file that tells every AI assistant working on your Go repository to follow your project's basic coding standards. The instruction applies to all Go files, repo-wide.

---

## File Structure

```
my-repo/
└── .ai/
    ├── manifest.yaml
    └── instructions/
        └── go-standards.yaml
```

---

## Step 1: Create the Manifest

Every `.ai/` project needs a `manifest.yaml`. Start with the minimal form:

```yaml
# .ai/manifest.yaml
schemaVersion: 1

project:
  name: my-service

build:
  defaultTargets:
    - claude
    - copilot
  defaultProfiles:
    - local-dev

preservation:
  default: preferred
```

This tells the compiler to emit output for Claude Code and GitHub Copilot using the `local-dev` profile.

---

## Step 2: Create the Instruction

```yaml
# .ai/instructions/go-standards.yaml
id: go-standards
kind: instruction
description: Core Go coding standards for this project
preservation: preferred

scope:
  fileTypes:
    - ".go"

content: |
  ## Go Coding Standards

  ### Code Style
  - Follow Effective Go and the Google Go Style Guide
  - Maximum file length: 500 lines — split files when approaching the limit
  - Use `gofmt` and `goimports` formatting (enforced by CI)

  ### Error Handling
  - Always check returned errors — never assign to `_` in production code
  - Wrap with context: `fmt.Errorf("operation name: %w", err)`
  - Use structured errors for domain-level failures

  ### Testing
  - Table-driven tests with `t.Run` subtests for all exported functions
  - Test file naming: `foo_test.go` in the same package
  - Run `go test -race ./...` before every PR

  ### Context
  - Pass `context.Context` as the first argument to all functions that do I/O
  - Never store contexts in structs — pass them through call chains

  ### Packages
  - Avoid `init()` functions except in test helpers
  - Prefer small, focused packages over large monolithic ones
```

---

## What This Produces

When you compile with `goagentmeta build`, the instruction is emitted to:

| Target | Output file |
|---|---|
| `claude` | `CLAUDE.md` (Go-scoped section) |
| `copilot` | `.github/copilot-instructions.md` |

The AI assistant will read these files on every session and apply the standards to all Go code it writes or reviews.

---

## Key Points

- **`scope.fileTypes: [".go"]`** — The instruction is only injected when the AI is working with `.go` files. For a global instruction (all files), omit the scope entirely.
- **`preservation: preferred`** — If a target doesn't fully support scoped instructions, the compiler lowers gracefully and warns rather than failing.
- **`content`** — Pure Markdown. Use headers, lists, and code blocks freely.

---

## Minimal Variant

The absolute minimum instruction — no scope, just always-on content:

```yaml
id: always-test
kind: instruction
content: "Always write unit tests for every public function."
```

---

## Next Steps

- [02-scoped-rule.md](02-scoped-rule.md) — Add a conditional rule on top of this instruction
- [03-basic-skill.md](03-basic-skill.md) — Create a reusable skill for Go Lambda development
- [../syntax-instruction.md](../syntax-instruction.md) — Full instruction reference
