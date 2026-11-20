# 0007. Versions: latest stable, pinned, recorded

Status: accepted (2026-11-10)

## Decision
Use the latest stable release of every tool and library. Versions are pinned by `go.mod`/`go.sum` and `pnpm-lock.yaml` and listed in `docs/versions.md`.
A major version that the rest of the toolchain does not yet support is not used; the reason is recorded next to the version so it is revisited.
Updates are a deliberate, tested step (`make test` must pass), not automatic.
