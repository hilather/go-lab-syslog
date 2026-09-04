# Tasks

Work packages for LabSyslog 1.0. Program order starts at
[`00-program-board.md`](00-program-board.md).

## Rules

- Read `../AGENTS.md` before taking a task.
- Do not start a task whose required dependencies are incomplete.
- Claim package and schema ownership before editing shared surfaces.
- Keep changes inside the stated ownership boundary unless coordination
  is recorded.
- Add regression tests for every behavior changed.
- Update all affected documentation in the same pull request.
- All required CI must pass. If CI fails, fix and harden it; do not
  bypass it.
- Add an unreleased changelog entry for externally observable behavior.
- Do not mark a task complete while TODO tests, skipped checks, stale
  docs, or unreviewed generated changes remain.
- Placeholder Make targets exit 1, not 0.
- If an invariant must change, write an ADR first.

## Template

Copy [`agent-task-template.md`](agent-task-template.md).
