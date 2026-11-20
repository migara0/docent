import type { ServerEvent } from './types';

/** Listens for live changes from the server. EventSource reconnects by itself; events are batched so a burst of saves causes one reload. */
export function connectEvents(onBatch: (events: ServerEvent[]) => void): () => void {
  // `?print` is used by `docent bundle`: headless Chrome waits for open connections, and a printed page does not change.
  if (new URLSearchParams(location.search).has('print')) return () => {};
  const es = new EventSource('/_api/events');
  let queue: ServerEvent[] = [];
  let timer: ReturnType<typeof setTimeout> | undefined;
  const push = (e: MessageEvent) => {
    try {
      queue.push(JSON.parse(e.data) as ServerEvent);
    } catch {
      return;
    }
    clearTimeout(timer);
    timer = setTimeout(() => {
      const batch = queue;
      queue = [];
      onBatch(batch);
    }, 80);
  };
  for (const type of ['page', 'tree', 'source']) es.addEventListener(type, push as EventListener);
  return () => {
    clearTimeout(timer);
    es.close();
  };
}
