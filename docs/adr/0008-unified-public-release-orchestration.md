---
status: accepted
date: 2026-08-01T14:49:32+09:00
updated: 2026-08-01T16:52:07+09:00
decision-makers:
  - project owner
agent: OpenAI Codex (GPT-5)
---

# ADR 0008: Publish public release surfaces through one orchestrator

## Context

The public core Go module and CLI, both npm packages, and the go-rod module and
CLI currently have three caller-visible release interfaces. A release owner
must push the core tag, wait for the Go proxy, dispatch npm publication, update
the go-rod dependency in a later commit, and push the adapter tag. The split
exposes implementation ordering to the caller and permits a release to stop in
a partially published state between independently approved workflows.

The ordering itself is real. The go-rod module must not be tested or tagged
against an unpublished core version, npm publication must use inspected
archives, and published Go tags and npm versions are immutable. The repository
also requires OIDC for npm and protected approval for every public write.

The go-rod dependency checksum is the only metadata normally unavailable before
the core tag exists. Go module zip rules omit nested modules, so changes below
`adapters/rod/` do not change the root module zip. The root module checksum for
the future version can therefore be calculated from the exact release commit
before publication and recorded in the nested module's `go.sum`.

## Decision

Public publication has one external seam: a manually dispatched unified
workflow accepting a SemVer version and the typed confirmation
`PUBLISH_RELEASE`. The workflow must run from the repository's default branch
and pins every operation to the dispatched commit.

The implementation remains sequential:

1. validate the release request and the anticipated go-rod dependency metadata;
2. run the complete release gate and build the inspected core/npm bundle without
   write or OIDC permission;
3. enter one protected `release-publish` Environment;
4. create or verify the annotated core tag and GitHub Release;
5. wait until the core module resolves through the public Go proxy;
6. publish or verify both npm packages through OIDC;
7. test go-rod against the published core, build its archives, and create or
   verify its annotated tag and GitHub Release;
8. wait for both Go modules and both npm packages to resolve, then verify the
   published artifacts.

The release-plan tool calculates the future root module zip and `go.mod`
checksums from an exact Git revision. Release preparation uses it to update
`adapters/rod/go.mod` and `adapters/rod/go.sum`; the unified workflow uses the
same interface in check mode. Release-facing source and documentation changes
must be committed before calculating these checksums.

Before a module tag exists, release preparation determines whether the version
is unused only from exact Git tags and GitHub Releases. It must not query the
public Go proxy for the future version: the proxy can cache a pre-tag negative
answer and delay visibility after publication. The orchestrator makes the first
public-proxy request only after it has created or verified the corresponding
annotated tag. The bounded post-tag retry remains a propagation check, not a
substitute for this ordering.

Publication is resumable but never overwriting. A retry skips a tag, Release,
or package only after proving that it matches the requested source or local
artifact. A mismatch fails closed. Tags are never moved, npm versions are never
reused, and differing GitHub Release assets are never replaced.

The previous public tag-triggered and npm-only workflows are removed instead of
being layered beneath the orchestrator. Private release workflows remain
separate because they have different visibility, authentication, and package
access contracts.

## Consequences

- A public release requires one workflow dispatch and one Environment approval.
- Partial failure is observable as a state within one run and can be resumed by
  rerunning the same version from the same commit.
- The protected publish job necessarily holds both `contents: write` and
  `id-token: write`; the unprivileged preparation job limits the duration and
  inputs exposed to those permissions.
- Repository administrators must configure the `release-publish` Environment,
  update both npm trusted publishers to the unified workflow and Environment,
  and leave release-tag creation unrestricted while protecting update,
  deletion, and non-fast-forward changes.
- Rollback cannot delete immutable public versions. Recovery completes the
  matching release or publishes a new prerelease or patch version.
