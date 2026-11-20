let ctx: CanvasRenderingContext2D | null | undefined;

/**
 * Turns any CSS colour expression (var(--x), color-mix(...), light-dark(...), oklch(...)) into plain "rgb(...)" or "rgba(...)".
 * Mermaid's own colour parser only understands classic formats, and the design tokens use modern ones.
 */
export function resolveColor(expr: string): string {
  const el = document.createElement('span');
  el.style.color = expr;
  el.style.display = 'none';
  document.documentElement.append(el);
  const computed = getComputedStyle(el).color;
  el.remove();
  ctx ??= document.createElement('canvas').getContext('2d', { willReadFrequently: true });
  if (!ctx) return computed;
  ctx.canvas.width = ctx.canvas.height = 1;
  ctx.clearRect(0, 0, 1, 1);
  ctx.fillStyle = '#000';
  ctx.fillStyle = computed; // an unparsable value keeps the previous (black), so this never throws
  ctx.fillRect(0, 0, 1, 1);
  const [r, g, b, a] = ctx.getImageData(0, 0, 1, 1).data as unknown as [number, number, number, number];
  return a === 255 ? `rgb(${r}, ${g}, ${b})` : `rgba(${r}, ${g}, ${b}, ${(a / 255).toFixed(3)})`;
}
