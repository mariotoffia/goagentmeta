# Example 10: Full Project

**Level**: 🔴 Advanced  
**Goal**: A complete `.ai/` project for a Go microservice that wires every entity type together: instruction, rule, skill, agent, hook, command, capability, plugin, reference, asset, and script.

---

## Project: `payment-service`

A Go microservice handling payment processing on AWS. The `.ai/` tree demonstrates how all entities compose into a coherent AI-assistance configuration.

---

## Complete Directory Layout

```
payment-service/
├── AGENT.md                             # Compiled from instructions
├── .ai/
│   ├── manifest.yaml
│   │
│   ├── instructions/
│   │   ├── project-overview.yaml        # Always-on project context
│   │   └── go-standards.yaml           # Go coding standards
│   │
│   ├── rules/
│   │   ├── secrets-policy.yaml         # No credential exposure (required)
│   │   └── generated-code.yaml         # No edits to generated files
│   │
│   ├── skills/
│   │   ├── go-aws-lambda.yaml          # Lambda dev skill
│   │   └── payment-processing.yaml     # Domain-specific skill
│   │
│   ├── agents/
│   │   ├── planner.yaml                # Task planner
│   │   ├── implementer.yaml            # Go implementer
│   │   └── security-reviewer.yaml      # Security review
│   │
│   ├── hooks/
│   │   ├── post-edit-validate.yaml     # gofmt + go vet after each edit
│   │   └── session-start.yaml          # Install tools at session start
│   │
│   ├── commands/
│   │   ├── review-pci.yaml             # /review-pci PCI-DSS compliance review
│   │   └── scaffold-handler.yaml       # /scaffold-handler new Lambda handler
│   │
│   ├── capabilities/
│   │   ├── mcp-github.yaml             # GitHub API
│   │   └── mcp-aws.yaml               # AWS API
│   │
│   ├── plugins/
│   │   ├── github-mcp.yaml             # GitHub MCP server
│   │   └── aws-mcp.yaml               # AWS MCP server
│   │
│   ├── references/
│   │   ├── pci-dss-requirements.md     # PCI-DSS compliance standard
│   │   └── payment-api-contracts.md    # Internal payment API specs
│   │
│   ├── assets/
│   │   └── templates/
│   │       └── lambda-handler.go.tmpl  # Handler scaffold template
│   │
│   └── scripts/
│       ├── hooks/
│       │   ├── post-edit-validate.sh
│       │   └── session-start.sh
│       └── commands/
│           └── scaffold-handler.sh
```

---

## manifest.yaml

```yaml
schemaVersion: 1

project:
  name: payment-service
  monorepo: false

build:
  defaultTargets: [claude, copilot, codex]
  defaultProfiles: [local-dev, ci]
  outputRoot: .ai-build
  syncMode: explicit

compilation:
  hierarchyMode: preserve-when-supported
  materialization: copy-or-symlink
  unsupportedMode: fail-on-required
  provenance: full

preservation:
  default: preferred

dependencies:
  "@community/golang-benchmark": "~1.1.3"

registries:
  - name: community
    url: https://registry.aicontrolplane.dev/v1
    priority: 1
```

---

## Instructions

```yaml
# .ai/instructions/project-overview.yaml
id: project-overview
kind: instruction
description: Payment service project overview and conventions
preservation: required

content: |
  # Payment Service — Agent Guidelines

  A Go Lambda microservice for payment transaction processing.
  Deployed on AWS. PCI-DSS compliant.

  ## Architecture
  - Hexagonal architecture: domain → application → port → adapter
  - All payment logic in `internal/domain/` — no AWS SDK imports there
  - AWS SDK v2 used only in `internal/adapter/`

  ## Build Commands
  ```bash
  make build    # Build all packages
  make test     # Run tests with race detection
  make lint     # golangci-lint
  make check    # build + lint + test
  ```

  ## Critical Constraints
  - **Never** log card numbers, CVVs, or full PANs — even partially
  - **Never** return raw AWS errors to API callers
  - All monetary amounts are `int64` cents — never `float64`
```

---

## Rules

```yaml
# .ai/rules/secrets-policy.yaml
id: secrets-policy
kind: rule
description: Prevent PCI-DSS credential and card data exposure
preservation: required

scope:
  paths: ["internal/**", "cmd/**"]
  fileTypes: [".go"]

conditions:
  - type: language
    value: go

content: |
  ## PCI-DSS Secret and Card Data Rules

  ### Forbidden in Logs or Errors
  - Card numbers (PAN), CVV, expiry dates
  - AWS credentials or tokens
  - Database connection strings

  ### Required Patterns
  - Mask PANs in logs: show only last 4 digits
  - Use `crypto/rand` for token generation
  - Store card data only via tokenization — never raw
```

---

## Skills

```yaml
# .ai/skills/payment-processing.yaml
id: payment-processing
kind: skill
description: Domain knowledge for payment transaction processing in Go
preservation: preferred

content: |
  ## Payment Processing Skill

  ### Money Representation
  Always use `int64` cents. Never use `float64` for monetary values.

  ```go
  type Money struct {
    AmountCents int64
    Currency    string // ISO 4217: "USD", "EUR"
  }
  ```

  ### Idempotency
  Every payment operation must be idempotent. Use an `IdempotencyKey`
  header and store processed keys in DynamoDB with TTL.

  ### Error Handling
  Never return raw payment processor errors to API callers.
  Map to internal error codes: `ErrInsufficientFunds`, `ErrCardDeclined`, etc.

  Consult `references/payment-api-contracts.md` for the full error code mapping.

requires:
  - filesystem.read
  - terminal.exec

resources:
  references:
    - references/pci-dss-requirements.md
    - references/payment-api-contracts.md
  assets:
    - assets/templates/lambda-handler.go.tmpl

activation:
  hints:
    - payment
    - transaction
    - charge
    - refund

allowedTools:
  - Read
  - Write
  - Edit
  - Grep
  - Glob
  - "Bash(go:*)"
```

---

## Commands

```yaml
# .ai/commands/review-pci.yaml
id: review-pci
kind: command
description: Review code changes for PCI-DSS compliance
preservation: preferred

action:
  type: prompt
  ref: |
    You are a PCI-DSS compliance reviewer.

    Read `references/pci-dss-requirements.md` for the full compliance requirements.

    Review all changed Go files in this session for:
    1. Card data (PAN, CVV, expiry) logged, stored, or transmitted unencrypted
    2. Missing idempotency keys on payment operations
    3. Raw payment processor errors returned to callers
    4. Monetary values stored as float64 instead of int64 cents

    Report each finding with: file, line range, severity, and remediation.
```

---

## Entity Relationship Map

```mermaid
erDiagram
    MANIFEST ||--o{ TARGET : builds-for
    MANIFEST ||--o{ PROFILE : builds-with

    INSTRUCTION["instruction\nproject-overview\ngo-standards"] }o--o{ SCOPE : scoped
    RULE["rule\nsecrets-policy\ngenerated-code"] }o--o{ SCOPE : scoped

    SKILL["skill\ngo-aws-lambda\npayment-processing"] }o--o{ CAPABILITY : requires
    SKILL }o--o{ REFERENCE : links
    SKILL }o--o{ ASSET : links

    AGENT["agent\nplanner\nimplementer\nsecurity-reviewer"] }o--o{ SKILL : links
    AGENT }o--o{ CAPABILITY : requires
    AGENT }o--o{ HOOK : scopes
    AGENT }o--o{ AGENT : "delegates to"

    HOOK["hook\npost-edit-validate\nsession-start"] ||--|| SCRIPT : executes

    COMMAND["command\nreview-pci\nscaffold-handler"] }o--o{ REFERENCE : loads

    PLUGIN["plugin\ngithub-mcp\naws-mcp"] }o--o{ CAPABILITY : provides
    CAPABILITY["capability\nmcp.github\nmcp.aws"] ||--|| PLUGIN : provided-by
```

---

## Compilation Output

For target `claude`, profile `local-dev`, the compiler emits:

```
.ai-build/claude/local-dev/
├── CLAUDE.md                           # project-overview + go-standards instructions
├── .claude/
│   └── rules/
│       ├── secrets-policy.mdc          # Lowered rule (scoped)
│       └── generated-code.mdc
├── claude_desktop_config.json          # MCP server configs (github, aws)
└── provenance.json                     # Full build provenance
```

For target `copilot`, profile `ci`:

```
.ai-build/copilot/ci/
├── .github/
│   ├── copilot-instructions.md         # Merged instructions
│   ├── copilot/
│   │   ├── review-pci.md              # Command as prompt file
│   │   └── scaffold-handler.md
│   └── copilot-mcp.json               # MCP server configs
└── provenance.json
```

---

## Key Takeaways

1. **Instructions and rules** provide always-on and conditional guidance — no configuration needed, they just work
2. **Skills** package reusable knowledge with capability requirements and resource links
3. **Agents** compose skills, enforce tool policies, and wire delegation + handoffs
4. **Hooks** provide deterministic automation backed by scripts
5. **Commands** give users on-demand workflows backed by prompts, skills, or scripts
6. **Capabilities + plugins** are the runtime wiring — define what's needed abstractly, let the compiler wire the concrete MCP configs per target
7. **References and assets** keep the AI context lean by providing on-demand depth

---

## See Also

- [../README.md](../README.md) — Entity taxonomy and common envelope
- All `syntax-*.md` files for full field references
- [ARCHITECTURE.md](../../../ARCHITECTURE.md) — Compiler pipeline architecture
