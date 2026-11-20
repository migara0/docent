# 0004. UI: Svelte 5, TypeScript, Vite, hand-written CSS

Status: accepted (2026-11-10)

## Decision
Svelte 5 (runes) + TypeScript 6 + Vite 8, tested with Vitest and Playwright (using the system Chrome, no browser download).
Styling is plain CSS driven by design tokens (`design/tokens.css`, OKLCH colours, `color-mix`, container queries, view transitions): no UI kit and no Tailwind,
so the look is entirely ours and the CSS stays small. Icons: Lucide (tree-shaken). Fonts: Inter and JetBrains Mono, self-hosted (works offline).
Search runs in the browser (MiniSearch). Mermaid 12 is bundled and loaded only on pages that contain a diagram.

## Consequences
Small initial bundle (target under 150 KB gzip, excluding Mermaid). Diagram zoom and search become native features instead of workarounds around a theme.
TypeScript 7 is the newest major but Svelte's checker supports 5 and 6 only (7 needs an experimental dual install), so TypeScript is pinned to 6.0.3 until Svelte supports 7.

## Measured (M2)
First load of the home screen: **139 KB gzipped including both fonts** (87 KB fonts, 43 KB JavaScript, 9 KB CSS), against a 150 KB budget. Mermaid (about 70 chunks, loaded only on pages with a diagram)
and MiniSearch (only when searching a small library) are not part of that. Native `<dialog>` elements are used for the palette, picker, print chooser and enlarged diagram, so focus handling and Escape come from the browser.
Two bugs the end-to-end suite caught are worth remembering: a dialog's `close` event arrives asynchronously, so a stale one can close a dialog that was just reopened (every dialog now checks `open` before reacting),
and a list fetched earlier must not overwrite a newer one (the folder picker ignores superseded responses).
