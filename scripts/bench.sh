#!/usr/bin/env bash
# Measures Docent against the budgets in docs/adr/0008-performance-budgets.md.
#
#   scripts/bench.sh [SOURCE_DIR_OF_MARKDOWN] [PAGES]      (defaults: testdata/fixtures, 10000)
#
# It builds a synthetic library of PAGES pages by repeating the Markdown found under SOURCE_DIR (real notes give realistic numbers;
# nothing is copied into this repository), starts a server with an isolated config, and prints: start-up time, idle and settled memory,
# search time, and how long an edit takes to reach the browser. Needs: go, python3, curl.
set -euo pipefail
cd "$(dirname "$0")/.."
SRC=${1:-testdata/fixtures}; PAGES=${2:-10000}
WORK=$(mktemp -d); trap 'kill $SERVER 2>/dev/null || true; rm -rf "$WORK"' EXIT
export DOCENT_HOME="$WORK/home"; mkdir -p "$DOCENT_HOME"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$WORK/docent" ./cmd/docent
printf 'binary: %.1f MB\n' "$(echo "$(stat -c %s "$WORK/docent") / 1048576" | bc -l)"

python3 - "$SRC" "$WORK/lib" "$PAGES" <<'PY'
import itertools, pathlib, sys
src, out, n = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2]), int(sys.argv[3])
texts = [p.read_text(errors="ignore") for p in src.rglob("*.md") if 300 < p.stat().st_size < 20000]
assert texts, "no Markdown found under " + str(src)
cyc = itertools.cycle(texts)
for i in range(n):
    d = out / f"area-{i // 100:03d}" / f"topic-{(i // 10) % 10}"
    d.mkdir(parents=True, exist_ok=True)
    (d / f"page-{i % 10:02d}.md").write_text(next(cyc) + f"\n\nGenerated page {i}.\n")
print(f"library: {n} pages, {sum(f.stat().st_size for f in out.rglob('*.md')) // 1048576} MB of Markdown")
PY

rss() { awk '/VmRSS/{printf "%.1f MB", $2/1024}' /proc/$1/status; }
PORT=18799; t0=$(date +%s%N)
"$WORK/docent" serve --no-open --addr 127.0.0.1:$PORT >/dev/null 2>&1 & SERVER=$!
until curl -s localhost:$PORT/_api/health >/dev/null; do sleep 0.005; done
echo "cold start to first request: $(( ($(date +%s%N) - t0) / 1000000 )) ms (budget 150)"
echo "idle memory, no library: $(rss $SERVER) (budget 40)"
t0=$(date +%s%N); "$WORK/docent" open "$WORK/lib" --no-open >/dev/null
ID=$(curl -s localhost:$PORT/_api/libraries | python3 -c "import sys,json; print(json.load(sys.stdin)['libraries'][0]['id'])")
until curl -s "localhost:$PORT/_api/tree?src=$ID" -o /dev/null -w '%{http_code}' | grep -q 200; do sleep 0.05; done
echo "index $PAGES pages: $(( ($(date +%s%N) - t0) / 1000000 )) ms (budget 3000)"
sleep 3; echo "memory with $PAGES pages: $(rss $SERVER) (budget 40)"
curl -s -o /dev/null -w 'open one page: %{time_total}s (budget 0.1)\n' "localhost:$PORT/_api/page?src=$ID&path=area-000/topic-0/page-00.md"
curl -s -o /dev/null -w 'search: %{time_total}s\n' "localhost:$PORT/_api/search?src=$ID&q=the"
f="$WORK/lib/area-000/topic-0/page-00.md"; : > "$WORK/ev"
( curl -sN localhost:$PORT/_api/events | while read -r l; do case "$l" in data:*) date +%s%N > "$WORK/ev"; break;; esac; done ) & CURL=$!
sleep 0.4; ts=$(date +%s%N); echo "edited" >> "$f"
for _ in $(seq 100); do [ -s "$WORK/ev" ] && break; sleep 0.01; done
echo "edit reaches the browser after: $(( ($(cat "$WORK/ev") - ts) / 1000000 )) ms (budget 100)"
kill $CURL 2>/dev/null || true
echo "memory after tree + search + edit: $(rss $SERVER)"
