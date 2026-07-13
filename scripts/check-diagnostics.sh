#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
python3 - <<'PY'
from pathlib import Path
import re
import sys

implementation = set()
for path in Path("internal").rglob("*.go"):
    implementation.update(re.findall(r'"([EW]_[A-Z0-9_]+)"', path.read_text(encoding="utf-8")))
documented = set(re.findall(r'`([EW]_[A-Z0-9_]+)`', Path("docs/spec/diagnostics.md").read_text(encoding="utf-8")))
missing = sorted(implementation - documented)
if missing:
    print("implementation diagnostics missing from docs/spec/diagnostics.md:", file=sys.stderr)
    for code in missing:
        print(f"  {code}", file=sys.stderr)
    raise SystemExit(1)
print(f"diagnostics documented: {len(implementation)} implementation codes")
PY
