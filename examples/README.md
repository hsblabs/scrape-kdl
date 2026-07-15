# Executable examples

Each subdirectory is self-contained user documentation and an acceptance case. `example.json` declares exact language and IR versions, inputs, reviewed artifacts, execution modes, and any browser adapter requirement. [`schema.json`](./schema.json) is the data contract shared by Go and future TypeScript runners.

Run the checked-in suite without changing files:

```bash
make examples
```

The runners compile every extractor independently in Go and TypeScript and execute every declared entry against saved HTML or an isolated local HTTP response. They compare canonical IR and output with the same reviewed artifacts. Browser examples use the shared rod/Playwright fixture and the official adapter gates documented in `docs/release-gates.md`. No default example uses the external network or credentials.

After intentionally reviewing a semantic change, refresh the goldens explicitly:

```bash
go run ./cmd/check-examples --update
git diff -- examples
make examples
```

Never update goldens merely to make a failing check pass. Review the source, specification, IR, output, and warning changes together.
