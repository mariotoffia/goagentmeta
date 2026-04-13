---
id: architecture-principles
kind: instruction
version: 1
description: Core architecture principles — hexagonal, clean, DDD, compiler-as-control-plane
preservation: required
scope:
  paths:
    - "internal/**"
    - "pkg/**"
    - "cmd/**"
appliesTo:
  targets: ["*"]
  profiles: ["*"]
---

# Architecture Principles

GoAgentMeta follows a **hexagonal (ports & adapters)** architecture with **Clean Architecture** layering and **Domain-Driven Design** tactical patterns.

## Dependency Rule

Dependencies point **inward** — never outward. The innermost ring knows nothing about the outermost ring.

```
Adapters → Ports → Application → Domain
   ↑                                ↓
   └── NEVER depends on ────────────┘
```

## Layer Responsibilities

### Domain (`internal/domain/`)
Pure business types. **Zero infrastructure imports.** Contains:
- `model/` — Canonical objects (Instruction, Rule, Skill, Agent, Hook, Command, Reference)
- `capability/` — Capability contracts and resolution logic
- `plugin/` — Plugin distribution, selection, security models
- `build/` — Build coordinates (Target, Profile, BuildUnit)
- `pipeline/` — Pipeline IR types, phase definitions, compiler errors
- `tool/` — Tool plugin expressions, registry, validation

### Ports (`internal/port/`)
Small Go interfaces defining contracts. **No implementation.** Contains:
- `stage/` — Core compiler plugin SPI (`Stage` interface)
- `renderer/` — Target renderer interface (extends `Stage`)
- `registry/` — Package resolver, fetcher, searcher, verifier
- `filesystem/` — Reader, Writer, Materializer
- `reporter/` — Reporter, DiagnosticSink, ProvenanceRecorder

### Application (`internal/application/`)
Use-case orchestration through port interfaces. Contains:
- `compiler/` — Pipeline orchestrator, stage registry, compiler context
- `dependency/` — Dependency resolution, lock files, manifest parsing
- `packaging/` — Package building (VS Code, NPM, OCI)

### Adapters (`internal/adapter/`)
Infrastructure implementations — outermost ring. Contains:
- `stage/` — Built-in pipeline stage implementations (parser, validator, normalizer, etc.)
- `renderer/` — Target renderer backends (claude, cursor, copilot, codex)
- `cli/` — CLI adapter (Cobra commands, wiring)
- `filesystem/` — OS and in-memory filesystem
- `registry/` — Local, Git, HTTP registry clients
- `reporter/` — Diagnostic sink, provenance recorder, formatters
- `tool/` — Built-in tool plugins
- `plugin/` — External plugin loading

## Compiler Pipeline

The compiler executes 10 phases in order:

1. **parse** — Read `.ai/` files → `SourceTree`
2. **validate** — Structural + semantic validation
3. **resolve** — External dependency resolution
4. **normalize** — Inheritance, defaults, scope normalization → `SemanticGraph`
5. **plan** — Expand target×profile → `BuildPlan`
6. **capability** — Resolve capabilities → `CapabilityGraph`
7. **lower** — Lower unsupported concepts → `LoweredGraph`
8. **render** — Emit target-native files → `EmissionPlan`
9. **materialize** — Write files to disk
10. **report** — Generate provenance and build report

## Bounded Contexts

Each pipeline phase operates on its own IR types. IR flows forward through the pipeline — never backward.
