// Which overlays and panels are open. One place, so keyboard shortcuts, buttons and menus all agree.

class UI {
  navOpen = $state(false); // the sidebar drawer on a phone
  paletteOpen = $state(false);
  pickerOpen = $state(false);
  printOpen = $state(false);
  readingOpen = $state(false);
  shareOpen = $state(false);
  navigatorOpen = $state(false); // the Browse popup
  libMenuOpen = $state(false);
  diagram = $state<{ svg: string; title: string } | null>(null); // the diagram open in the enlarged view
  table = $state<{ html: string; title: string } | null>(null); // the table open in the enlarged view
  titleOut = $state(false); // the page's title has scrolled under the top bar: the top bar shows it (with the note ID) instead of the breadcrumb

  closeMenus() {
    this.readingOpen = false;
    this.libMenuOpen = false;
  }
}

export const ui = new UI();
