# Performance regression policy

v1 has no absolute speed SLA. The release gate measures representative compilation and saved-HTML extraction in both primary runtimes and compares each workload with a same-process JSON-decoding calibration. Ratios reduce runner-to-runner CPU variance while retaining sensitivity to algorithmic or allocation regressions.

The same command reports non-blocking scaling observations for a 10,000-element selector workload. It compares full traversal with a one-result bounded query in both runtimes. These observations are evidence for algorithmic changes rather than an environment-independent latency promise; the correctness tests separately require bounded queries to preserve document order and cardinality behavior.

The reviewed observations and thresholds are stored in `performance/baselines.json`. Each maximum is four times its measured median baseline. A pull request that exceeds a maximum fails; changing a baseline requires recording the new measured result and explaining the implementation change in the pull request. Faster observations do not update baselines automatically.

Measure without enforcing the checked-in threshold:

```bash
npm run build:typescript
node scripts/check-performance.mjs --measure
```

For allocation evidence on the Go DOM boundary, run:

```bash
GOTOOLCHAIN=local go test -run '^$' -bench 'BenchmarkQuery(All|Limit)' -benchmem ./internal/dom
```

Run the blocking regression gate:

```bash
make performance
```
