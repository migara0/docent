import { api } from './api';
import { folderUrl, normalizePath } from './router.svelte';
import type { Recent, SourceInfo, TreeNode, TreeResponse } from './types';

/** Lookup tables for one library's tree: which file a page URL means, and which folder a folder URL means. */
export interface TreeIndex {
  pages: Map<string, string>;
  folders: Map<string, TreeNode>;
  firstIndex?: string; // the root index or README page, if any
}

function buildIndex(src: string, root: TreeNode): TreeIndex {
  const idx: TreeIndex = { pages: new Map(), folders: new Map([[normalizePath(folderUrl(src, '')), root]]) };
  const walk = (n: TreeNode) => {
    for (const c of n.children ?? []) {
      if (c.kind === 'page') {
        if (c.url) idx.pages.set(normalizePath(c.url), c.path);
        if (n === root && !idx.firstIndex && /^(index|readme)\.(md|markdown)$/i.test(c.name)) idx.firstIndex = c.path;
      } else {
        idx.folders.set(normalizePath(folderUrl(src, c.path)), c);
        walk(c);
      }
    }
  };
  walk(root);
  return idx;
}

class AppState {
  libs = $state<SourceInfo[]>([]);
  recents = $state<Recent[]>([]);
  readOnly = $state(false);
  loaded = $state(false);
  version = $state(0); // bumped when the server reports a change, so open pages reload
  currentPath = $state(''); // the page being read, for highlighting in the tree
  error = $state('');
  trees = $state<Record<string, TreeResponse | undefined>>({});
  private indexes = new Map<string, TreeIndex>();

  async refreshLibraries() {
    try {
      const r = await api.libraries();
      this.libs = r.libraries;
      this.recents = r.recents;
      this.readOnly = r.readOnly;
      this.error = '';
    } catch (e) {
      this.error = e instanceof Error ? e.message : String(e);
    } finally {
      this.loaded = true;
    }
  }

  async loadTree(src: string) {
    try {
      const t = await api.tree(src);
      this.indexes.set(src, buildIndex(src, t.tree));
      this.trees[src] = t;
    } catch {
      this.trees[src] = undefined;
      this.indexes.delete(src);
    }
  }

  index(src: string): TreeIndex | undefined {
    void this.trees[src]; // reactive dependency: callers re-run when the tree reloads
    return this.indexes.get(src);
  }

  /** The libraries to show in lists: everything except the built-in guide, which has its own button. */
  get listed(): SourceInfo[] {
    return this.libs.filter((l) => l.kind !== 'help');
  }

  lib(src: string): SourceInfo | undefined {
    return this.libs.find((l) => l.id === src);
  }
}

export const app = new AppState();
