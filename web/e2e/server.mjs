// Starts Docent for the end-to-end tests: a fresh config folder, a temporary copy of the e2e fixture library registered as "E2E notes".
import { spawn, spawnSync } from 'node:child_process';
import { cpSync, existsSync, mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const bin = process.env.DOCENT_BIN ?? join(root, 'bin', 'docent');
if (!existsSync(bin)) {
  const b = spawnSync('go', ['build', '-o', bin, './cmd/docent'], { cwd: root, stdio: 'inherit' });
  if (b.status !== 0) process.exit(1);
}
const work = mkdtempSync(join(tmpdir(), 'docent-e2e-'));
const lib = join(work, 'lib');
cpSync(join(root, 'testdata/fixtures/e2e'), lib, { recursive: true });
writeFileSync(join(work, 'where.txt'), lib); // tests that edit files read the library location from here
const env = { ...process.env, DOCENT_HOME: join(work, 'home'), DOCENT_E2E_LIB: lib };
const add = spawnSync(bin, ['add', lib, '--name', 'E2E notes'], { env, stdio: 'inherit' });
if (add.status !== 0) process.exit(1);

// A developer-notes library with a real git workspace behind it, so the stale-module notice can be tested against actual history.
const notes = join(work, 'doc-notes');
const workspace = join(work, 'workspace');
cpSync(join(root, 'testdata/fixtures/devnotes'), notes, { recursive: true });
const repo = join(workspace, 'billing-api');
mkdirSync(join(repo, 'src', 'profiles'), { recursive: true });
writeFileSync(join(repo, 'src', 'profiles', 'index.ts'), 'export {};\n');
writeFileSync(join(repo, 'src', 'config.ts'), 'export {};\n');
const git = (...a) => spawnSync('git', ['-C', repo, '-c', 'user.name=T', '-c', 'user.email=t@t', ...a], { env: { ...env, GIT_AUTHOR_DATE: '2026-11-04T12:00:00', GIT_COMMITTER_DATE: '2026-11-04T12:00:00' }, stdio: 'ignore' });
spawnSync('git', ['init', '-q', repo], { stdio: 'ignore' });
git('add', '-A');
git('commit', '-qm', 'code changed after the module note was updated');
// A plan note with dates relative to today, so what the board says ("3 days", "updated today") never changes from one run to the next.
const day = (ago) => {
  const d = new Date(Date.now() - ago * 86_400_000); // the local date, as Docent shows it
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
};
mkdirSync(join(notes, 'features', 'badge', 'plan'), { recursive: true });
writeFileSync(
  join(notes, 'features', 'badge', 'plan', `${day(6)}-badge-rollout.md`),
  `---
title: "Badge rollout"
id: DOC-0009
description: ""
date: ${day(6)}
updated: ${day(0)}
author: "Test"
status: in-progress
tags:
  - badge
  - plan
stages:
  - name: Design
    steps:
      - {step: Scope, state: done}
      - {step: Review, state: done}
  - name: Develop
    steps:
      - {step: Schema, state: done}
      - {step: Print endpoint, state: now}
      - {step: Validation, state: todo}
      - {step: Logging, state: todo}
  - name: Testing
    steps:
      - {step: Unit, state: todo}
      - {step: QA pass, state: todo}
  - name: Release
    steps:
      - {step: Docs, state: todo}
waiting:
  - {what: Printer firmware update, who: Facilities, since: ${day(3)}, blocks: Release}
  - {what: QA environment access, who: QA lead, since: ${day(1)}}
---

# Badge rollout

The goal, the plan and the decisions go here, as ordinary Markdown.
`,
);
const addDev = spawnSync(bin, ['add', notes, '--name', 'doc-notes', '--profile', 'devnotes', '--workspace', workspace], { env, stdio: 'inherit' });
if (addDev.status !== 0) process.exit(1);

const server = spawn(bin, ['serve', '--no-open', '--addr', '127.0.0.1:18900'], { env, stdio: 'inherit' });
for (const sig of ['SIGINT', 'SIGTERM']) process.on(sig, () => { server.kill(sig); });
server.on('exit', (code) => process.exit(code ?? 0));
