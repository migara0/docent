/** The URL of a folder of a library, with a trailing slash. */
export function folderUrl(src: string, path: string): string {
  const tail = path ? path.split('/').map(encodeURIComponent).join('/') + '/' : '';
  return `/${encodeURIComponent(src)}/${tail}`;
}
