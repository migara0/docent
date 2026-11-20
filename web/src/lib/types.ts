// The JSON the Go server returns (internal/library and internal/server). Keep in step with docs/architecture.md.

export type SourceKind = 'library' | 'folder' | 'file' | 'dropped' | 'help';

export interface SourceInfo {
  id: string;
  name: string;
  kind: SourceKind;
  profile: string;
  root: string;
  file?: string;
  idPrefix?: string; // note IDs of this library start with this (developer notes)
  pages: number;
  folders: number;
  updated: string;
  indexing: boolean;
  error?: string;
}

export interface Recent {
  path: string;
  name: string;
  kind: 'folder' | 'file';
  openedAt: string;
}

export interface LibrariesResponse {
  libraries: SourceInfo[];
  recents: Recent[];
  readOnly: boolean;
}

export interface TreeNode {
  name: string;
  path: string;
  title?: string;
  id?: string; // the note ID of a page (developer notes)
  kind: 'folder' | 'page';
  count?: number;
  url?: string;
  children?: TreeNode[];
}

export interface TreeResponse {
  info: SourceInfo;
  tree: TreeNode;
}

export interface Heading {
  level: number;
  id: string;
  text: string;
}

export interface PageLink {
  text: string;
  href: string;
  kind: 'page' | 'broken' | 'file' | 'external' | 'anchor';
  target?: string;
}

export interface Ref {
  path: string;
  title: string;
  url: string;
}

export interface Crumb {
  name: string;
  url?: string;
}

export interface PageData {
  source: string;
  path: string;
  url: string;
  title: string;
  meta?: Record<string, unknown>;
  html: string;
  toc: Heading[];
  words: number;
  modified: string;
  createdAt?: string; // created and last updated with the time of day (RFC 3339), when known
  updatedAt?: string;
  crumbs: Crumb[];
  links: PageLink[];
  backlinks: Ref[];
  prev?: Ref;
  next?: Ref;
  warnings?: string[];
  notices?: Notice[];
  file?: string;
  place?: Place;
  plan?: PlanBoard; // the status board of a plan note
}

export interface Notice {
  kind: 'warning' | 'info';
  title: string;
  text?: string;
  items?: string[];
}

/** Where a developer note lives: its service (or feature, or technology) and type. */
export interface Place {
  group?: string;
  service: string;
  type: string;
}

export interface NoteRow {
  path: string;
  title: string;
  url: string;
  id?: string;
  created?: string;
  updated: string;
  createdAt?: string; // the same with the time of day (RFC 3339), when known
  updatedAt?: string;
  status?: string;
  tags?: string[];
  folder: string;
  type?: string;
  service?: string;
}

export interface TagCount {
  name: string;
  count: number;
  pages: NoteRow[];
}

export interface SearchHit {
  path: string;
  url: string;
  title: string;
  folder: string;
  score: number;
  snippet?: string;
}

export interface FsEntry {
  name: string;
  path: string;
  kind: 'dir' | 'file';
  md?: number;
  cap?: boolean;
}

export interface FsListing {
  dir: string;
  parent: string;
  home: string;
  crumbs: { name: string; path: string }[];
  entries: FsEntry[];
  sep?: string; // the machine's path separator: "/" or "\"
  roots?: string[]; // Windows: the drives ("C:\", "D:\")
}

export interface ServerEvent {
  type: 'page' | 'tree' | 'source';
  source: string;
  path?: string;
}

export interface ShareInfo {
  available: boolean;
  active: boolean;
  urls: string[];
  until?: string;
  noNetwork: boolean;
}

export interface DropResult {
  source: string;
  pages: { name: string; path: string; url: string }[];
}

export interface IdMatch {
  id: string;
  title: string;
  url: string;
  source: string;
  library: string;
  exact: boolean;
}

/** A plan note's status board (internal/devnotes/plan.go). */
export type StepState = 'todo' | 'now' | 'done' | 'blocked' | 'skipped';
export type StageState = 'done' | 'active' | 'todo' | 'blocked';

export interface PlanStep {
  step: string;
  state: StepState;
}

export interface PlanStage {
  name: string;
  state: StageState;
  current?: boolean; // the first stage that is not done
  done: number;
  total: number;
  steps: PlanStep[];
  waiting?: number; // external tasks holding it up
}

export interface PlanWait {
  what: string;
  who?: string;
  since?: string;
  blocks?: string;
  days: number; // -1 when there is no date
}

export interface PlanBoard {
  status: 'planning' | 'in-progress' | 'done' | 'dropped' | string;
  done: number;
  total: number;
  percent: number;
  now: string;
  next: string;
  stages: PlanStage[];
  waiting: PlanWait[];
  updated?: string;
  days: number;
}

export interface PlanRow {
  source: string;
  library: string;
  path: string;
  url: string;
  id?: string;
  title: string;
  service: string;
  board: PlanBoard;
}

export type PlanEdit =
  | { source: string; path: string; op: 'state'; ref: string; state: StepState }
  | { source: string; path: string; op: 'resolve'; index: number };

/** The newest notes of a library with note IDs (GET /_api/latest). Note IDs only go up, so a higher ID is a newer note. */
export interface LatestNote {
  id: string;
  num: number;
  title: string;
  url: string;
  path: string;
  type?: string;
  service?: string;
  created?: string;
}

export interface LatestSource {
  source: string;
  name: string;
  prefix: string;
  maxId: number;
  notes: LatestNote[];
}
