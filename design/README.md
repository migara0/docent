# Design

| File | What |
| ---- | ---- |
| `tokens.css` | The design tokens: colour (OKLCH, light and dark via `light-dark()`), type, space, shape, motion. Change `--accent*` to re-skin a library |
| `docent.css` | Every component: shell, rail, tree, topbar, TOC, prose, callouts, tables, code, diagrams, cards, palette, dialogs, popovers. The Svelte app imports it unchanged |
| `prototype.html` | A clickable prototype of the real design: home, reading a note, Ctrl/Cmd+K search, the folder picker (open once or add to libraries), reading controls, focus mode, themes, phone layout |
| `fonts/` | Inter and JetBrains Mono (variable, Latin subset), self-hosted |

View it:

```bash
cd design && python3 -m http.server 5180 --bind 127.0.0.1   # then open http://127.0.0.1:5180/prototype.html
```

## Principles

- **Content first.** A calm reading column (about 72 characters), generous line height, one accent colour, no decoration that is not information.
- **Keyboard first, touch friendly.** Ctrl/Cmd+K reaches everything; every control is at least 36 px and works on a phone.
- **Real text size.** The reading controls change the page text and reflow it (`--reading-scale`), they do not zoom the interface.
- **Light, dark and system**, from one set of tokens. Reduced motion is respected.
- **Plain words for everyone.** "Library", "Open a folder or file", "Add to libraries": no developer jargon in the interface.
