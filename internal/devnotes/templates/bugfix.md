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
    What was broken, and the one-line fix.

## Symptom

What users or logs showed. Include an error message or request sample.

## Root cause

What actually went wrong and why it was not caught earlier.

## Fix

| File | Change |
| ---- | ------ |
| `src/...` | ... |

## How to verify

- [ ] Reproduction steps that failed before and pass now
- [ ] Regression check on neighbouring behaviour

## Impact and rollback

Who or what was affected, and how to revert.

## Commit message

```text
fix(scope): brief description
```
