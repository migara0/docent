import type { LatestSource, PlanBoard, PlanEdit, PlanRow, DropResult, FsListing, IdMatch, ShareInfo, LibrariesResponse, NoteRow, PageData, SearchHit, SourceInfo, TagCount, TreeResponse } from './types';

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init);
  if (!res.ok) {
    let msg = res.statusText;
    try {
      msg = ((await res.json()) as { error?: string }).error ?? msg;
    } catch {
      /* not JSON */
    }
    throw new ApiError(res.status, msg);
  }
  return (await res.json()) as T;
}

const q = (params: Record<string, string>) => new URLSearchParams(params).toString();
const post = (body: unknown): RequestInit => ({ method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });

export const api = {
  libraries: () => request<LibrariesResponse>('/_api/libraries'),
  tree: (src: string) => request<TreeResponse>(`/_api/tree?${q({ src })}`),
  page: (src: string, path: string, signal?: AbortSignal) => request<PageData>(`/_api/page?${q({ src, path })}`, { signal }),
  raw: async (src: string, path: string) => {
    const res = await fetch(`/_api/raw?${q({ src, path })}`);
    if (!res.ok) throw new ApiError(res.status, res.statusText);
    return res.text();
  },
  searchIndex: (src: string) => request<{ mode: 'client' | 'server'; docs?: SearchDoc[] }>(`/_api/search-index?${q({ src })}`),
  search: (src: string, query: string, signal?: AbortSignal) => request<{ hits: SearchHit[]; tookMs: number }>(`/_api/search?${q({ src, q: query, limit: '30' })}`, { signal }),
  overview: (src: string, folder: string) => request<{ folder: string; profile: string; notes: NoteRow[] }>(`/_api/overview?${q({ src, folder })}`),
  created: (src: string, signal?: AbortSignal) => request<{ pages: NoteRow[] }>(`/_api/created?${q({ src })}`, { signal }),
  recent: (src: string, limit = 10) => request<{ notes: NoteRow[] }>(`/_api/recent?${q({ src, limit: String(limit) })}`),
  ids: (text: string, src: string, signal?: AbortSignal) => request<{ matches: IdMatch[] }>(`/_api/ids?${q({ q: text, src })}`, { signal }),
  tags: (src: string) => request<{ tags: TagCount[] }>(`/_api/tags?${q({ src })}`),
  fs: (dir: string) => request<FsListing>(`/_api/fs?${q({ dir })}`),
  open: (path: string) => request<SourceInfo>('/_api/open', post({ path })),
  add: (path: string, name: string, profile: string) => request<SourceInfo>('/_api/libraries', post({ path, name, profile })),
  drop: (files: { name: string; text: string }[]) => request<DropResult>('/_api/drop', post({ files })),
  share: () => request<ShareInfo>('/_api/share'),
  setShare: (on: boolean, minutes = 60) => request<ShareInfo>('/_api/share', post({ on, minutes })),
  latest: () => request<{ sources: LatestSource[] }>('/_api/latest'),
  plans: (all = false) => request<{ plans: PlanRow[] }>(`/_api/plans${all ? '?all=1' : ''}`),
  editPlan: (edit: PlanEdit) => request<PlanBoard>('/_api/plan', post(edit)),
  remove: (id: string) => request<{ ok: boolean }>(`/_api/libraries/${encodeURIComponent(id)}`, { method: 'DELETE' }),
};

export interface SearchDoc {
  id: number;
  path: string;
  url: string;
  title: string;
  folder: string;
  headings: string;
  text: string;
}
