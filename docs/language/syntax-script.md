# Syntax Reference: Script

A **Script** is an executable artifact used by hooks, commands, skills, or inline plugins. Scripts are static files in `.ai/scripts/` that the compiler includes in the build output and the runtime executes.

Scripts are **not** instructions or knowledge documents — they are executable code. Common uses: validation scripts for hooks, formatting helpers for commands, test runners for skills, and server startup scripts for inline plugins.

---

## Quick Example

A bash validation script for a `post-edit` hook:

```bash
#!/usr/bin/env bash
# .ai/scripts/hooks/post-edit-validate.sh
# Validates changed Go files after every edit.
# Inputs: CHANGED_FILES (space-separated file paths)

set -euo pipefail

if [ -z "${CHANGED_FILES:-}" ]; then
  exit 0
fi

for file in $CHANGED_FILES; do
  if [[ "$file" == *.go ]]; then
    echo "Validating: $file"
    gofmt -l "$file" | grep . && { echo "ERROR: $file is not gofmt-formatted"; exit 1; } || true
    go vet ./$(dirname "$file")/... || exit 1
  fi
done

echo "Validation passed."
exit 0
```

---

## Script Declaration

Scripts are referenced by path from other entities. They are **not** standalone YAML objects — they are declared inline in skills, hooks, commands, and plugins.

### In a Hook

```yaml
# .ai/hooks/post-edit-validate.yaml
id: post-edit-validate
kind: hook
action:
  type: script
  ref: scripts/hooks/post-edit-validate.sh
```

### In a Command

```yaml
# .ai/commands/run-benchmarks.yaml
id: run-benchmarks
kind: command
action:
  type: script
  ref: scripts/commands/run-benchmarks.sh
```

### In a Skill's Resources

```yaml
# .ai/skills/go-aws-lambda.yaml
resources:
  scripts:
    - scripts/skills/go-aws-lambda/test.sh
    - scripts/skills/go-aws-lambda/deploy.sh
```

### In a Plugin's Artifacts

```yaml
# .ai/plugins/repo-graph.yaml
artifacts:
  scripts:
    - scripts/plugins/repo-graph/server.sh
    - scripts/plugins/repo-graph/init.sh
```

---

## Script Object Fields

When a script is surfaced as a first-class object (e.g., in build reports or the SDK), it carries the following fields:

| Field | Type | Required | Description |
|---|---|---|---|
| `path` | string | yes | Relative path from the `.ai/` root to the script file |
| `description` | string | no | Short summary of the script's purpose |
| `interpreter` | string | no | Runtime used to execute the script. Empty means auto-detect from shebang or extension. |

---

## Directory Conventions

Organize scripts by their consumer type:

```
.ai/scripts/
├── hooks/                 # Scripts invoked by hooks
│   ├── post-edit-validate.sh
│   └── session-start-setup.sh
├── commands/              # Scripts invoked by commands
│   ├── run-benchmarks.sh
│   └── format-code.sh
├── skills/                # Scripts bundled with skills
│   └── go-aws-lambda/
│       ├── test.sh
│       └── deploy.sh
└── plugins/               # Scripts for inline plugin implementations
    └── repo-graph/
        └── server.sh
```

---

## Interpreter Auto-Detection

If `interpreter` is not set, the runtime infers it from:
1. The script's **shebang line** (`#!/usr/bin/env bash`, `#!/usr/bin/env python3`)
2. The **file extension** (`.sh` → bash, `.py` → python3, `.js` → node)

Supported auto-detected interpreters:

| Extension | Interpreter |
|---|---|
| `.sh` | `bash` |
| `.py` | `python3` |
| `.js` | `node` |
| `.rb` | `ruby` |
| `.ts` | `ts-node` or `deno` |
| `.go` | `go run` |

Always include a shebang line as the first line of your script for maximum portability.

---

## Environment Variables

Hook scripts receive context via environment variables set by the runtime. The exact variables depend on the hook's `inputs.include` configuration:

| Variable | Set when input includes | Description |
|---|---|---|
| `CHANGED_FILES` | `changedFiles` | Space-separated list of changed file paths |
| `WORKING_DIRECTORY` | `workingDirectory` | Absolute path to the working directory |
| `TOOL_NAME` | `toolName` | Name of the tool being invoked (pre/post-tool-use hooks) |
| `SESSION_ID` | `sessionId` | Unique session identifier |

Command scripts receive any arguments defined in the command action, passed as `$@`.

---

## Exit Code Conventions

| Exit Code | Meaning |
|---|---|
| `0` | Success — continue workflow |
| `1` | Failure — trigger `failurePolicy` (fail-build / warn / ignore) |
| `2` | User abort — always treated as failure |

---

## Minimal Script Template

```bash
#!/usr/bin/env bash
# Brief description of what this script does.
# Usage: ./script-name.sh [args]

set -euo pipefail

main() {
  echo "Starting..."
  # ... implementation ...
  echo "Done."
}

main "$@"
```

---

## See Also

- [syntax-hook.md](syntax-hook.md) — Hooks that execute scripts
- [syntax-command.md](syntax-command.md) — Commands that execute scripts
- [syntax-plugin.md](syntax-plugin.md) — Inline plugins that bundle scripts
- [examples/05-hooks-and-scripts.md](examples/05-hooks-and-scripts.md) — Hooks and scripts example
