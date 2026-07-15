# Performance regression policy

v1 has no absolute speed SLA. The release gate measures representative compilation and saved-HTML extraction in both primary runtimes and compares each workload with a same-process JSON-decoding calibration. Ratios reduce runner-to-runner CPU variance while retaining sensitivity to algorithmic or allocation regressions.

The reviewed observations and thresholds are stored in `performance/baselines.json`. Each maximum is four times its measured median baseline. A pull request that exceeds a maximum fails; changing a baseline requires recording the new measured result and explaining the implementation change in the pull request. Faster observations do not update baselines automatically.

Measure without enforcing the checked-in threshold:

```bash
npm run build:typescript
node scripts/check-performance.mjs --measure
```

Run the blocking regression gate:

```bash
make performance
```
