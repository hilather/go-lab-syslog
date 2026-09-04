# Parallelization plan

See `00-program-board.md` for the normative table.

## Lanes after M0

| Lane | Waves | Notes |
|---|---|---|
| Wire | WIRE-001 | Blocks both data-plane lanes |
| UDP | UDP-001 → FIL-001 | FIL also needs CFG |
| TCP | TCP-001 | Independent of UDP after WIRE |
| Store | STORE-001 | Independent of UDP/TCP after WIRE |
| Control | STA-001 → API-001 → SEC-001 → MCP-001 | Strict order |
| Observability | OBS-001 | After UDP + API hooks |
| Ship | DEP-001 → SWAP-001 | After ingest + API |
| UI | UI-001 | After API + SEC; required for GA |

## Do not parallelize

- CFG-001 with anyone writing YAML field names.
- API-001 with MCP-001 (MCP consumes the frozen capability table).
- SEC-001 with MCP-001 (bearer middleware shared).
