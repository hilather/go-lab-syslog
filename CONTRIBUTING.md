# Contributing

1. Read [START-HERE.md](START-HERE.md) and [AGENTS.md](AGENTS.md).
2. Take one work package from [tasks/00-program-board.md](tasks/00-program-board.md)
   whose dependencies are complete.
3. Implement from the numbered `docs/` file and the matching
   `tasks/wave-*.md`. Do not invent paths or capability IDs.
4. Invariant changes need an ADR first.
5. Tests and docs ship in the same change.
6. Run the Make targets listed in AGENTS.md. Missing targets must
   fail closed. Operator SPA work needs Node **22.14.0**
   (`make web-test`, `make web-build`).

License: Apache-2.0.
