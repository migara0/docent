import { describe, expect, it } from 'vitest';
import { editorUrl } from './editor';

describe('editorUrl', () => {
  it('links a Linux or macOS path as it is', () => {
    expect(editorUrl('/home/a/notes/x.md', 'vscode')).toBe('vscode://file/home/a/notes/x.md');
    expect(editorUrl('/home/a/notes/x.md', 'zed')).toBe('zed://file/home/a/notes/x.md');
  });

  it('turns a Windows path into the form editors expect', () => {
    expect(editorUrl('C:\\Users\\a\\notes\\x.md', 'vscode')).toBe('vscode://file/C:/Users/a/notes/x.md');
    expect(editorUrl('d:\\work\\y.md', 'zed')).toBe('zed://file/d:/work/y.md');
  });

  it('gives nothing without a file or an editor', () => {
    expect(editorUrl(undefined, 'vscode')).toBeUndefined();
    expect(editorUrl('/x.md', '')).toBeUndefined();
  });
});
