---
id: binary-output
kind: rule
preservation: required
conditions:
  - type: language
    value: go
---

Manually compiled Go binaries must have .out postfix so .gitignore ignores them.
