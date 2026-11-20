export interface ToastAction {
  label: string;
  run: () => void;
}

class Toast {
  message = $state('');
  on = $state(false);
  action = $state<ToastAction | undefined>(undefined);
  private timer: ReturnType<typeof setTimeout> | undefined;

  /** A short message; with an action it stays longer and shows a button. */
  show(message: string, ms = 2200, action?: ToastAction) {
    this.message = message;
    this.action = action;
    this.on = true;
    clearTimeout(this.timer);
    this.timer = setTimeout(() => (this.on = false), ms);
  }

  hide() {
    clearTimeout(this.timer);
    this.on = false;
  }
}

export const toast = new Toast();
