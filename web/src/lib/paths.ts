// Paths on the reader's machine, as typed or pasted into the folder picker: "/home/a/notes" on Linux and macOS, "C:\Users\a\notes" on Windows.

/** The last part of a path: "notes" for "/home/a/notes/" or "C:\Users\a\notes". */
export function baseName(path: string): string {
  return path.replace(/[\\/]+$/, '').split(/[\\/]/).pop() ?? '';
}

/** The folder that holds a path: "/home/a" for "/home/a/x.md", "C:\Users\a" for "C:\Users\a\x.md", and a root for a file at the top. */
export function parentOf(path: string): string {
  const at = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'));
  if (at < 0) return path;
  const dir = path.slice(0, at);
  if (dir === '') return path[0] ?? '/'; // "/x.md" is in "/"
  if (/^[A-Za-z]:$/.test(dir)) return dir + path[at]; // "C:\x.md" is in "C:\"
  return dir;
}
