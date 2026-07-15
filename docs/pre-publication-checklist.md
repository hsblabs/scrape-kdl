# v1 pre-publication checklist

Status: complete for the repository state represented by `release/rc-state.json`. Publication has not been approved or performed.

- [x] Freeze the Go, TypeScript, CLI, diagnostic, dated language, Validated IR, HTTP/browser runtime, and official adapter contracts.
- [x] Freeze Linux/macOS, amd64/arm64, Go 1.26, Node.js 26, Chromium-supported, and Firefox/WebKit-best-effort tiers in the executable support matrix.
- [x] Run canonical IR, diagnostic, cross-language, HTTP, browser, WPT/Chromium, security, cancellation, resource-bound, race, fuzz, and clean-consumer gates.
- [x] Dry-build all four CLI archives, checksums, Go module-proxy input, both npm tarballs, and a clean native install without publishing them.
- [x] Publish migration notes, draft v1 release notes, the maintenance policy, and the RC operations procedure in the repository.
- [x] Make publication workflows manual, exact-confirmation guarded, protected by the `v1-publication` GitHub environment, and fail-closed unless its `PUBLICATION_APPROVAL` secret exactly authorizes the requested tag.
- [x] Provide a reviewed blocker ledger and an executable 14-consecutive-day eligibility check.
- [ ] Make the repository public. Project-owner approval required; intentionally not performed.
- [ ] Publish a pre-v1 release candidate. Project-owner approval required; intentionally not performed.
- [ ] Operate the candidate for at least 14 consecutive blocker-free days. Ready to start after approval; no period is claimed yet.
- [ ] Publish stable v1 tags, archives, checksums, Go modules, npm packages, and adapters. Project-owner approval required; intentionally not performed.

Run the pre-publication checks with:

```bash
make rc-check
```

This command never creates a tag, GitHub release, or registry publication. The machine-readable checklist and candidate ledger are in `release/rc-state.json`; `make rc-state` rejects premature eligibility or publication claims.
