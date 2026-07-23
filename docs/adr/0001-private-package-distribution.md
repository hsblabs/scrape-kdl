---
status: accepted
date: 2026-07-24T00:31:11+09:00
decision-makers:
  - project owner
agent: OpenAI Codex (GPT-5)
---

# ADR 0001: Private package distribution

## Context

The repository is private and has no tags, GitHub Releases, or npm package
records. The project needs an operational private channel for the Go modules,
both npm packages, and the two CLIs without authorizing public distribution.

The current GitHub plan does not expose protected Environments or repository
rulesets for this private repository. npm trusted publishing cannot be configured
until each package record exists, and OIDC authenticates publishing but not
private package reads. A private repository also cannot produce npm provenance.

## Decision

Private versions use the SemVer prerelease suffix `-private.N`, starting with a
positive sequence number. npm publishes them with restricted access and the
`private` dist-tag. GitHub publishes CLI archives as prereleases in the private
repository. Public and private publication remain separate workflows with
different typed confirmations and opposite repository-visibility guards.

Private publication follows this dependency order:

1. verify an exact private core candidate;
2. create the annotated core tag and private core GitHub Release;
3. verify direct private Go module consumption;
4. publish both restricted npm packages;
5. update `adapters/rod/go.mod` to the tagged private core version and verify it
   without a local `replace`;
6. create the annotated adapter tag and private go-rod GitHub Release;
7. verify clean private Go and npm consumers.

The first npm version is bootstrapped manually from the inspected tarballs using
an interactive npm session with 2FA. No npm write token is stored in GitHub.
After both package records exist, each package trusts
`release-npm-private.yml` for `npm publish` through OIDC. Private package
installation and post-publication checks use user authentication or a read-only
token; they do not reuse publication credentials.

Because protected Environments and rulesets are unavailable on the current
private-repository plan, private workflows rely on manual dispatch, distinct
typed confirmation, private-visibility checks, existing annotated tags,
least-privilege job permissions, and serialized publication. Public workflows
retain their stronger Environment and tag-rule requirements.

## Consequences

- No public visibility change is required for private operation.
- Creating tags, Releases, or npm package versions remains an owner-authorized
  external action and is not performed by repository verification.
- A failed version is never moved or reused; recovery increments `private.N`.
- Making the repository public would also expose its existing Git tags and
  GitHub Releases. The owner must review that history before changing visibility.
- Restricted npm versions remain restricted until package access is explicitly
  changed.
- Private Go consumers must configure `GOPRIVATE` and non-interactive GitHub
  authentication. Private npm consumers need npm read access.
- npm provenance is unavailable while the source repository is private.
