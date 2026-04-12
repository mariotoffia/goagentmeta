# Syntax Reference: Rule

A **Rule** is a scoped or conditional policy. Rules are semantically distinct from [Instructions](syntax-instruction.md): an instruction is unconditionally injected within its scope; a rule is injected only when one or more activation conditions are met. Rules are the right choice for language-specific standards, file-pattern restrictions, generated-code policies, and security-sensitive path controls.

Some targets do not have a native concept of "conditional rules" and require the compiler to lower rules into instructions. This lowering may lose the conditional semantics; use `preservation: required` to fail the build rather than accept silent loss.

---

## Quick Example

The primary authoring format is a **Markdown file with YAML frontmatter**:

```markdown
---
id: go-security-rules
kind: rule
description: Security policy for Go source files
preservation: preferred
scope:
  paths:
    - "services/**"
  fileTypes: [".go"]
conditions:
  - type: language
    value: go
---

## Go Security Rules

- Never log secrets, tokens, or credentials
- Always validate and sanitize external input before use
- Use `crypto/rand` for cryptographic randomness; never `math/rand`
- Prefer `net/http` timeouts: always set `ReadTimeout`, `WriteTimeout`, and `IdleTimeout`
```

Save this as `.ai/rules/go-security-rules.md`. The frontmatter holds metadata and conditions; the body is the rule content.

---

## Field Reference

### Inherited from ObjectMeta

See [ObjectMeta reference](README.md#common-envelope--objectmeta) for full field documentation.

| Field | Typical Usage for Rules |
|---|---|
| `id` | Unique identifier (e.g., `go-security-rules`, `no-generated-edits`) |
| `kind` | Always `rule` |
| `description` | Short summary for build reports |
| `preservation` | `required` for mandatory compliance; `preferred` for most rules |
| `scope` | Almost always set for rules — restricts by path or file type |
| `extends` | Inherit from a base rule and override specific conditions or content |
| `targetOverrides` | Adjust lowering hints or disable for specific targets |

### Content (Markdown Body)

The rule content is the Markdown body that follows the closing `---` of the frontmatter:

```markdown
---
id: example-rule
kind: rule
conditions:
  - type: language
    value: go
---

## Rule Content

This is the policy text injected when the rule's conditions are met.
```

| Part | Required | Description |
|---|---|---|
| Markdown body | yes | Policy text after the frontmatter `---`. Injected into the AI context when all conditions are satisfied. |

### `conditions`

```yaml
conditions:
  - type: language
    value: go
  - type: path-pattern
    value: "services/**"
  - type: generated
    value: "false"
```

`conditions` is a list of `RuleCondition` objects. All conditions in the list must be satisfied for the rule to activate (AND semantics). An empty `conditions` list means the rule is unconditional within its scope (equivalent to an instruction).

#### RuleCondition Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `type` | string | yes | Condition kind. See condition types below. |
| `value` | string | yes | Condition value to match against. |

#### Condition Types

| Type | Value Examples | Description |
|---|---|---|
| `language` | `go`, `typescript`, `python`, `rust` | Matches files of the specified programming language |
| `path-pattern` | `services/**`, `cmd/**`, `internal/adapter/**` | Matches files under the specified glob pattern |
| `generated` | `true`, `false` | Matches generated files (detected by `// Code generated` header or similar) |
| `file-extension` | `.go`, `.ts`, `.yaml` | Matches files with the given extension |
| `label` | `backend`, `security-sensitive` | Matches objects or files tagged with the given label |

---

## Scope vs. Conditions

`scope` and `conditions` are complementary. **Scope** defines the addressable domain — which files and directories the rule is registered for. **Conditions** define the activation predicate evaluated at runtime.

```markdown
---
id: api-input-validation
kind: rule
scope:
  paths:
    - "internal/adapter/**"    # Only applies within the adapter subtree
  fileTypes: [".go"]           # Only applies to Go files
conditions:
  - type: path-pattern
    value: "**/handler_*.go"   # Only activates for handler files
---

Always validate and sanitize all HTTP request parameters before use.
```

---

## Lowering Notes

| Target | Native Rule Support | Lowering |
|---|---|---|
| `claude` | Native — `scope.paths` maps to directory hierarchy | No lowering needed |
| `cursor` | Adapted — MDC `globs` frontmatter carries file-type scope | Condition semantics may be lost |
| `copilot` | Lowered — single instruction file; scope merged by prefix comment | Conditions lowered to scope comments |
| `codex` | Native — directory hierarchy supported | No lowering needed |

When a target cannot preserve condition semantics and `preservation: required`, the build fails with a diagnostic explaining which condition types are unsupported.

---

## Inheritance Example

Base rule — applies to all Go files:

```markdown
---
id: base-go-rule
kind: rule
scope:
  fileTypes: [".go"]
conditions:
  - type: language
    value: go
---

Always run `go vet` before committing.
```

Derived rule — narrows to security-sensitive services paths:

```markdown
---
id: go-secrets-rule
kind: rule
extends:
  - base-go-rule
scope:
  paths:
    - "services/auth/**"
    - "services/payments/**"
---

Never log `*http.Request` bodies or form values — they may contain credentials.
```

---

## Comparison: Instruction vs. Rule

| Aspect | Instruction | Rule |
|---|---|---|
| Activation | Always-on within scope | Conditional within scope |
| `conditions` field | Not supported | Supported |
| Target lowering | Straightforward | May lose condition semantics |
| Use case | Standards, principles, policies | Language-specific, generated-code, security-sensitive |

---

## See Also

- [syntax-instruction.md](syntax-instruction.md) — Always-on guidance
- [examples/02-scoped-rule.md](examples/02-scoped-rule.md) — Beginner example
