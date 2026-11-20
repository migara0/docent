---
title: {{title_q}}
id: {{id}}
description: ""
date: {{date}}
author: {{author_q}}
tags:
  - {{service}}
  - {{type}}
---

# {{title}}

!!! info "Summary"
    What was upgraded (framework, SDK, dependencies) and from which versions to which.

## Version changes

| Package | From | To | Reason |
| ------- | ---- | -- | ------ |
| `@nestjs/core` | x | y | ... |

## Breaking changes handled

| Area | Change needed | File |
| ---- | ------------- | ---- |
| ... | ... | `src/...` |

## Vulnerabilities addressed

| Advisory | Severity | Package | Resolved by |
| -------- | -------- | ------- | ----------- |

## Build and test results

- Build: ...
- Tests: ...

## QA focus areas

- [ ] ...

## Rollback

Revert the lockfile and `package.json` together.

## Commit message

```text
chore(deps): brief description
```
