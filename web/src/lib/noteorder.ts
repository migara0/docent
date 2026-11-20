// The order note types are shown in, and friendlier names for them.
export const TYPE_ORDER = ['module', 'implementation', 'bugfix', 'upgrade', 'api', 'adr', 'qa', 'runbook', 'guide', 'commits'];

export function typeRank(t: string): number {
  const i = TYPE_ORDER.indexOf(t);
  return i < 0 ? TYPE_ORDER.length : i;
}

export const typeLabel = (t: string) => (t ? t.charAt(0).toUpperCase() + t.slice(1).replace(/-/g, ' ') : 'Notes');
