---
status: living document
updated: 2026-07-19
---

# Design backlog

Deferred structural improvements, phrased in the Grokking Simplicity
vocabulary the project uses: actions (I/O, effects), calculations (pure
functions), data (inert facts). Each entry names the trigger that makes it
worth doing; none is urgent on its own.

## Stratify the CLI `run` functions

`runExtract` in `cmd/scrape-kdl/main.go` and `run` in
`adapters/rod/cmd/scrape-kdl-rod/main.go` interleave flag parsing
(calculation), validation, option wiring, execution (action), and result
rendering in one function, and exit codes are decided at several depths.
The improvement is to parse arguments into an invocation value first — a
plain data struct describing what to run — so `args -> invocation` becomes
a calculation testable without I/O, and executing the invocation becomes a
single action. Do this the next time either CLI grows a mode or flag;
retrofit both CLIs in the same change so their structure stays parallel.

## Copy session state at the public API boundary

`convertOptions` in `scrapekdl.go` passes the caller's `http.Header` map
and cookie slice into the executor by reference. A caller mutating the
session while an extraction runs would race the request-building action.
Grokking Simplicity's copy-in discipline says defensive-copy at the trust
boundary: `Headers.Clone()` plus a slice copy in `convertOptions`. One-line
fix; bundle it with the next change that touches `scrapekdl.go`, and mirror
whatever guarantee is chosen in the TypeScript runtime's option handling.

## Extract result rendering into a calculation

Both CLIs duplicate the warning-line formatting loop and the
result-serialization choice (`--json` envelope vs indented document,
stdout vs `--out`). Rendering is a calculation (`result -> bytes` and
`warnings -> lines`); only the final writes are actions. Move the
formatting into `internal/clisupport` beside the input/session helpers the
CLIs already share when the output format next changes, so the parity
between the binaries stays compiler-enforced instead of copy-maintained.
