---
title: {{title_q}}
id: {{id}}
description: ""
date: {{date}}
updated: {{date}}
author: {{author_q}}
status: planning
tags:
  - {{service}}
  - {{type}}
stages:
  - name: Design
    steps:
      - {step: Scope, state: todo}
      - {step: Review, state: todo}
  - name: Plan
    steps:
      - {step: Task split, state: todo}
      - {step: Approved, state: todo}
  - name: Develop
    steps:
      - {step: Build, state: todo}
  - name: Testing
    steps:
      - {step: Unit, state: todo}
      - {step: Integration, state: todo}
      - {step: QA pass, state: todo}
  - name: Release
    steps:
      - {step: Docs, state: todo}
      - {step: Cherry-pick, state: todo}
waiting: []
---

# {{title}}

The board above is drawn from the front matter. Keep it short: a few steps per stage, one idea each. Change it with `docent plan` (or edit the front matter).
Everything below is ordinary Markdown for the details.

## Goal

One or two sentences: what this task delivers and why.

## Plan

<!-- The approved plan: what changes in which service, in the order we do it. Link API notes, ADRs and the commits note by ID. -->

## Decisions

<!-- Choices made along the way, each with the reason. -->

## Risks and open questions

<!-- What could go wrong, and what is still undecided (with who decides). -->

## Related notes

- ...
