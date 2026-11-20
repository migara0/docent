# Test data

`fixtures/` holds small, public, synthetic libraries that exercise every supported feature (see `docs/dialect.md`).
Real note collections are never copied here (they can be private). Tests that need them read the path from `DOCENT_CORPUS`
(for example `DOCENT_CORPUS=/path/to/workspace`) and skip when it is not set.
