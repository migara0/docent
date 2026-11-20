import { api } from './api';
import { app } from './app.svelte';
import { router } from './router.svelte';
import { copyText, download } from './format';
import { toast } from './toast.svelte';
import type { PageData, TreeNode } from './types';

function firstPage(n: TreeNode | undefined): TreeNode | undefined {
  for (const c of n?.children ?? []) {
    if (c.kind === 'page') return c;
    const deeper = firstPage(c);
    if (deeper) return deeper;
  }
  return undefined;
}

/** Opens a folder or file (once, or added to the libraries) and shows it. */
export async function openPath(path: string, add?: { name: string; profile: string }) {
  try {
    const info = add ? await api.add(path, add.name, add.profile) : await api.open(path);
    await app.refreshLibraries();
    await app.loadTree(info.id);
    const first = firstPage(app.trees[info.id]?.tree);
    router.go(info.kind === 'file' && first?.url ? first.url : `/${encodeURIComponent(info.id)}/`);
    if (add) toast.show(`Added “${info.name}”. It will be here next time too.`);
  } catch (e) {
    toast.show(e instanceof Error ? e.message : 'Could not open that', 4000);
  }
}

export async function removeLibrary(id: string, name: string) {
  try {
    await api.remove(id);
    await app.refreshLibraries();
    toast.show(`Removed “${name}”. Your files were not touched.`);
    if (router.route.kind === 'library' && router.route.src === id) router.go('/');
  } catch (e) {
    toast.show(e instanceof Error ? e.message : 'Could not remove it', 4000);
  }
}

export async function copyMarkdown(page: PageData) {
  try {
    toast.show((await copyText(await api.raw(page.source, page.path))) ? 'Page copied as Markdown' : 'Could not copy');
  } catch {
    toast.show('Could not copy');
  }
}

export async function downloadMarkdown(page: PageData) {
  try {
    const name = page.path.split('/').pop() ?? 'page.md';
    download(name, await api.raw(page.source, page.path));
  } catch {
    toast.show('Could not download');
  }
}

export function editorUrl(file: string | undefined, editor: string): string | undefined {
  if (!file) return undefined;
  if (editor === 'vscode') return `vscode://file${file}`;
  if (editor === 'zed') return `zed://file${file}`;
  return undefined;
}

/** The built-in guide. */
export const HELP_URL = '/help/guide';
export function openHelp() {
  router.go(HELP_URL);
}

const MAX_DROP = 5 << 20;

/** Reads Markdown files the reader dropped (or chose) and opens them as temporary copies. Nothing is linked to the originals. */
export async function uploadFiles(files: File[], skippedFolders = false) {
  const md = files.filter((f) => /\.(md|markdown)$/i.test(f.name));
  const big = md.filter((f) => f.size > MAX_DROP);
  const take = md.filter((f) => f.size <= MAX_DROP);
  const notes: string[] = [];
  if (skippedFolders) notes.push('folders cannot be dropped: use Open folder or file');
  if (files.length - md.length > 0 && !skippedFolders) notes.push(`${files.length - md.length} not Markdown`);
  if (big.length) notes.push(`${big.length} over ${MAX_DROP >> 20} MB`);
  if (!take.length) {
    toast.show(`Nothing to open: ${notes.join(', ') || 'drop Markdown (.md) files'}`, 5000);
    return;
  }
  try {
    const res = await api.drop(await Promise.all(take.map(async (f) => ({ name: f.name, text: await f.text() }))));
    await app.refreshLibraries();
    await app.loadTree(res.source);
    router.go(res.pages[0]!.url);
    toast.show(`Opened ${take.length === 1 ? take[0]!.name : `${take.length} files`} as ${take.length === 1 ? 'a temporary copy' : 'temporary copies'}${notes.length ? ` (skipped: ${notes.join(', ')})` : ''}`, 5000);
  } catch (e) {
    toast.show(e instanceof Error ? e.message : 'Could not open the dropped files', 5000);
  }
}
