/**
 * The link that opens a file in the reader's editor, or undefined when no editor is chosen.
 * Editors take a path with forward slashes that starts with "/": a Windows path ("C:\notes\a.md") becomes "/C:/notes/a.md".
 */
export function editorUrl(file: string | undefined, editor: string): string | undefined {
  if (!file) return undefined;
  const path = file.replace(/\\/g, '/').replace(/^(?=[A-Za-z]:\/)/, '/');
  if (editor === 'vscode') return `vscode://file${path}`;
  if (editor === 'zed') return `zed://file${path}`;
  return undefined;
}
