// Which folders of a library's tree are open. Remembered per library in this browser.

export class TreeState {
  open = $state<Record<string, boolean>>({});
  private key: string;

  constructor(src: string) {
    this.key = `docent:tree:${src}`;
    try {
      this.open = JSON.parse(localStorage.getItem(this.key) ?? '{}') as Record<string, boolean>;
    } catch {
      this.open = {};
    }
  }

  private save() {
    try {
      localStorage.setItem(this.key, JSON.stringify(this.open));
    } catch {
      /* ignore */
    }
  }

  toggle(path: string) {
    this.open[path] = !this.open[path];
    this.save();
  }

  /** Opens every folder above a page so it is visible in the tree. */
  reveal(pagePath: string) {
    const segs = pagePath.split('/').slice(0, -1);
    let changed = false;
    for (let i = 1; i <= segs.length; i++) {
      const p = segs.slice(0, i).join('/');
      if (!this.open[p]) {
        this.open[p] = true;
        changed = true;
      }
    }
    if (changed) this.save();
  }
}
