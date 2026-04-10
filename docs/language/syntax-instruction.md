# Syntax Reference: Instruction

An **Instruction** is always-on guidance injected unconditionally into the AI model's context within its scope. Instructions are appropriate for architecture principles, coding standards, testing expectations, review policies, domain vocabulary, and workflow guidance that should always be present.

Instructions are distinct from [Rules](syntax-rule.md): rules are conditional or scoped to specific file types or conditions; instructions are unconditional within their scope.

---

## Quick Example

```yaml
id: go-standards
kind: instruction
description: Go coding standards and conventions
preservation: preferred
scope:
  fileTypes: [".go"]
content: |
  ## Go Coding Standards

  - Use Go 1.24+ features including structured concurrency
  - Follow Effective Go guidelines
  - Context-first APIs: always pass `context.Context` as the first argument
  - Wrap errors with `fmt.Errorf("operation: %w", err)` to preserve stack context
  - Table-driven tests with subtests (`t.Run`) for all exported functions
  - No `init()` functions except for test fixtures
```

---

## Field Reference

### Inherited from ObjectMeta

See [ObjectMeta reference](README.md#common-envelope--objectmeta) for full field documentation.

| Field | Typical Usage for Instructions |
|---|---|
| `id` | Unique identifier (e.g., `go-standards`, `arch-principles`) |
| `kind` | Always `instruction` |
| `description` | Short human-readable summary shown in build reports |
| `preservation` | Usually `preferred`; use `required` for mandatory compliance standards |
| `scope` | Restrict by path, file type, or label; omit for repo-root (global) instructions |
| `appliesTo` | Limit to specific targets or profiles if needed |
| `extends` | Inherit from a base instruction and add/override fields |
| `labels` | Tags for grouping (e.g., `security`, `testing`, `go`) |
| `targetOverrides` | Adjust file placement or disable for specific targets |

### `content`

```yaml
content: |
  ## Architecture Principles
  Always use hexagonal architecture with clean dependency rules.
  Dependencies must point inward: adapters → application → domain.
```

| Field | Type | Required | Description |
|---|---|---|---|
| `content` | string | yes | Markdown text injected into the AI context. Supports full Markdown including headers, lists, code blocks, and tables. |

---

## Scope Behavior

An instruction with no `scope` applies globally (repository root):

```yaml
# Applies everywhere — all files, all directories
id: always-on-policy
kind: instruction
content: |
  All code must pass CI before merging.
```

An instruction with `scope.paths` applies only to matching subtrees:

```yaml
# Applies only within the services/ subtree
id: services-standards
kind: instruction
scope:
  paths:
    - "services/**"
content: |
  Each service must expose a /healthz endpoint.
```

An instruction with `scope.fileTypes` applies only when the AI is working with matching files:

```yaml
# Applies only to Go files
id: go-error-handling
kind: instruction
scope:
  fileTypes: [".go"]
content: |
  Always check returned errors. Never assign errors to `_`.
```

---

## Preservation Notes

| Preservation | Effect |
|---|---|
| `required` | Build fails if the instruction cannot be emitted for a target. Use for mandatory compliance policy. |
| `preferred` | Emitted when the target supports it; skipped with a warning otherwise. Suitable for most instructions. |
| `optional` | May be silently skipped; always reported. Use for enrichment instructions. |

---

## Target Mapping

Instructions map to the primary model-context file for each target:

| Target | Output file | Notes |
|---|---|---|
| `claude` | `CLAUDE.md` or scoped markdown | Natively supports hierarchical placement |
| `cursor` | `.cursor/rules/*.mdc` | MDC format with frontmatter glob filters |
| `copilot` | `.github/copilot-instructions.md` | Single flat file; scope merging required |
| `codex` | `AGENTS.md` | Natively supports hierarchical placement |

Cursor requires the instruction content to be lowered into MDC format. If the instruction has `scope.fileTypes`, the compiler emits an appropriate `globs` frontmatter in the `.mdc` file.

---

## Inheritance Example

```yaml
# Base instruction
id: base-standards
kind: instruction
content: |
  Always write tests.

# Derived instruction — adds to base content
id: go-standards
kind: instruction
extends:
  - base-standards
content: |
  Use table-driven tests with t.Run for all Go functions.
```

---

## Minimal Form

The simplest possible instruction:

```yaml
id: always-test
kind: instruction
content: "Always write unit tests for every public function."
```

---

## See Also

- [syntax-rule.md](syntax-rule.md) — Conditional policy (scoped by condition)
- [syntax-skill.md](syntax-skill.md) — Reusable workflow bundles
- [examples/01-first-instruction.md](examples/01-first-instruction.md) — Beginner example
