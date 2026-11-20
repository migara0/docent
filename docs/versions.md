# Versions (latest stable, pinned)

Resolved on 2026-11-10. Locked by `go.mod` / `web/pnpm-lock.yaml`.

| Component | Version | Note |
| --------- | ------- | ---- |
| Go toolchain | 1.27.3 | pinned in `go.mod`; auto-downloaded, system Go is not touched |
| Svelte | 5.57.3 | |
| Vite | 8.3.x | |
| @sveltejs/vite-plugin-svelte | 7.3.1 | |
| TypeScript | **6.0.3** | 7.0.2 is the newest major, but `svelte-check` supports 5 and 6 only (7 needs an experimental dual install). Revisit when Svelte tooling supports 7 |
| svelte-check | 4.7.8 | |
| Vitest | 5.1.0 | |
| Playwright Test | 1.65.0 | uses the system Chrome (`channel: "chrome"`), no browser download |
| @axe-core/playwright | 4.14.0 | accessibility checks in the end-to-end tests (MPL-2.0, used as a test tool only, not shipped) |
| MiniSearch | 7.2.0 | MIT |
| Mermaid | 12.3.0 | MIT; loaded only on pages with a diagram, themed from the design tokens |
| Inter / JetBrains Mono (variable) | 5.3.0 | OFL fonts via Fontsource, self-hosted |
| Lucide icons (Svelte) | 1.70.0 | ISC |

| goldmark | 1.8.7 | Markdown (MIT) |
| rsc.io/qr | 0.2.0 | QR codes for sharing with a phone (BSD-3) |
| chroma | 2.27.0 | code colouring engine (MIT); its lexer data for 69 languages is copied into `internal/highlight` (see NOTICE.md) |
| fsnotify | 1.10.1 | file watching (BSD-3) |
| bluemonday | 1.0.27 | HTML sanitising (BSD-3) |
| cobra | 1.10.2 | CLI (Apache-2.0) |
| go.yaml.in/yaml/v3 | 3.0.6 | front matter (MIT / Apache-2.0); the maintained successor of gopkg.in/yaml.v3 |
| golang.org/x/net, x/sys, x/text | 0.62.0, 0.50.0, 0.43.0 | raised from the old minimums their parents ask for |
