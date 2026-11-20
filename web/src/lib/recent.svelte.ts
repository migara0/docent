// The last few pages you read, kept in this browser, for the search palette's empty state.
export interface RecentPage {
  title: string;
  url: string;
  library: string;
}

const KEY = 'docent:recent-pages';

function load(): RecentPage[] {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? '[]') as RecentPage[];
    return Array.isArray(v) ? v.slice(0, 8) : [];
  } catch {
    return [];
  }
}

class Recent {
  list = $state<RecentPage[]>(load());

  remember(p: RecentPage) {
    this.list = [p, ...this.list.filter((x) => x.url !== p.url)].slice(0, 8);
    try {
      localStorage.setItem(KEY, JSON.stringify(this.list));
    } catch {
      /* ignore */
    }
  }
}

export const recent = new Recent();
