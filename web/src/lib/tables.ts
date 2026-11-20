// Helpers for tables in a page: copy one as Markdown, and let long names (routes, field paths) wrap where they can.

/** A table as Markdown. The first row is the header; a pipe in a cell is escaped and a line break becomes a space. */
export function toMarkdown(rows: string[][]): string {
  const clean = (c: string) => c.replace(/\s*\n\s*/g, ' ').replace(/\|/g, '\\|').trim();
  const width = Math.max(0, ...rows.map((r) => r.length));
  if (!rows.length || !width) return '';
  const line = (r: string[]) => `| ${Array.from({ length: width }, (_, i) => clean(r[i] ?? '')).join(' | ')} |`;
  const [head = [], ...body] = rows;
  return [line(head), `| ${Array.from({ length: width }, () => '---').join(' | ')} |`, ...body.map(line)].join('\n');
}

/** One piece of a cell as Markdown: code, bold, italics and links keep their marks, so the copy reads like what was written. */
function inline(node: Node): string {
  if (node.nodeType === Node.TEXT_NODE) return node.textContent ?? '';
  if (!(node instanceof Element)) return '';
  const inner = () => [...node.childNodes].map(inline).join('');
  switch (node.tagName) {
    case 'CODE':
      return '`' + (node.textContent ?? '') + '`';
    case 'STRONG':
    case 'B':
      return `**${inner()}**`;
    case 'EM':
    case 'I':
      return `*${inner()}*`;
    case 'DEL':
      return `~~${inner()}~~`;
    case 'A': {
      const href = node.getAttribute('href');
      return href ? `[${inner()}](${href})` : inner();
    }
    case 'BR':
      return ' ';
    case 'WBR':
      return '';
    default:
      return inner();
  }
}

/** The cells of a table in the page, as Markdown text. */
export function cellsOf(table: HTMLTableElement): string[][] {
  return [...table.rows].map((r) => [...r.cells].map((c) => [...c.childNodes].map(inline).join('').replace(/\s+/g, ' ').trim()));
}

/** Where a long piece of code may break: after a slash, dot, underscore, colon, comma or hyphen. Returns the pieces, or null when it is short enough to leave alone. */
export function breakPoints(text: string, min = 16): string[] | null {
  if (text.length < min) return null;
  const parts = text.split(/(?<=[/._:,-])/);
  return parts.length > 1 ? parts : null;
}

/** Puts a <wbr> (an invisible place the browser may break the line) into long code in a table. The text itself is unchanged, so copying gives the same text. */
export function softBreakCode(root: ParentNode): void {
  for (const code of root.querySelectorAll('td code, th code')) {
    for (const node of [...code.childNodes]) {
      if (node.nodeType !== Node.TEXT_NODE) continue;
      const parts = breakPoints(node.textContent ?? '');
      if (!parts) continue;
      const frag = document.createDocumentFragment();
      parts.forEach((p, i) => {
        if (i > 0) frag.append(document.createElement('wbr'));
        frag.append(p);
      });
      node.replaceWith(frag);
    }
  }
}
