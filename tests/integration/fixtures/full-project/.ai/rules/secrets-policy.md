---
id: secrets-policy
kind: rule
preservation: required
scope:
  paths: ["**/*.go"]
conditions:
  - type: language
    value: go
---

Never store secrets in source code.
