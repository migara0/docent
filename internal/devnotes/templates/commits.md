---
title: {{title_q}}
id: {{id}}
description: ""
date: {{date}}
author: {{author_q}}
status: in-progress
release: ""
tags:
  - {{service}}
  - {{type}}
---

# {{title}}

!!! info "Purpose"
    The commits that make up this change, oldest first, so they can be cherry-picked into a
    release branch. The commit tables below are generated from git; do not edit them by hand.

**Related notes:** link the implementation / bugfix / QA notes for this change.

## Cherry-pick notes

Hand-written, safe from refreshes.

- Dependencies on other changes (must be picked first):
- Commits to skip or pick differently (reverts, environment-specific config):
- Expected conflicts:
- [ ] Picked into release branch:

!!! tip "Keeping this up to date"
    Set `status: released` and `release: <version>` in the front matter once shipped; that
    freezes the note (`refresh` will skip it).

    ```bash
    python3 scripts/commits.py add <this-note> --repo <repo> --range develop..<feature-branch>
    python3 scripts/commits.py refresh <this-note>   # after new commits, rebases or amends
    ```

## Commits
