// "Copy this section as Markdown": find a heading in the page's source and return it with everything under it.

const FENCE = /^\s*(`{3,}|~{3,})/;
const ATX = /^ {0,3}(#{1,6})[ \t]+(.*?)[ \t]*#*[ \t]*$/;

/** Heading text as shown on the page: no markup, no { attribute list }. */
function plain(text: string): string {
  return text
    .replace(/[ \t]+\{[^{}]*\}[ \t]*$/, '')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/[`*_~]/g, '')
    .replace(/\s+/g, ' ')
    .trim();
}

/** The section whose heading shows `text` (the `occurrence`-th time that exact text appears), up to the next heading of the same or higher level. */
export function sectionMarkdown(raw: string, text: string, occurrence: number): string | undefined {
  const body = raw.replace(/^---\n[\s\S]*?\n---\n/, (m) => '\n'.repeat(m.split('\n').length - 1)); // keep line numbers; drop front matter
  const lines = body.split('\n');
  const heads: { line: number; level: number; text: string }[] = [];
  let fence = '';
  lines.forEach((l, i) => {
    const f = FENCE.exec(l);
    if (f) {
      if (!fence) fence = f[1]!;
      else if (f[1]![0] === fence[0] && f[1]!.length >= fence.length) fence = '';
      return;
    }
    if (fence) return;
    const m = ATX.exec(l);
    if (m) heads.push({ line: i, level: m[1]!.length, text: plain(m[2]!) });
  });
  const want = plain(text);
  let seen = 0;
  const at = heads.findIndex((h) => h.text === want && seen++ === occurrence);
  if (at < 0) return undefined;
  const start = heads[at]!;
  const next = heads.slice(at + 1).find((h) => h.level <= start.level);
  return lines.slice(start.line, next ? next.line : lines.length).join('\n').replace(/[ \t]+\{[^{}]*\}[ \t]*$/m, '').replace(/\s+$/, '') + '\n';
}
