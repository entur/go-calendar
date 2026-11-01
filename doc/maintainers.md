# Maintainer guide

Operations, CI and release material for the people who run this repository. Users of the library need only the [README](../README.md). The design and its reasons are in [ADR 0001](adr/0001-norwegian-calendar-design.adoc).

## CI and required checks

| Workflow | Jobs | When |
|---|---|---|
| `ci.yml` | `test (floor)`, `test (stable)`, `test-386`, `lint`, `vuln`, `fuzz (…)` ×4, `scripts`, `zizmor` | pull requests, pushes to `main`, weekly |
| `codeql.yml` | `CodeQL (go)`, `CodeQL (actions)` on pull requests; the shared code-scan workflow on `main` and weekly | as stated |
| `scorecard.yml` | OpenSSF Scorecard | pushes to `main`, weekly |
| `release.yml` | release-please, `verify-module`, tag audit | pushes to `main` |
| `tag-guard.yml` | tag audit and issue on failure | `v*` tag push, daily, on demand from `main`, from `release.yml` |

`test (floor)` runs the exact Go version in `go.mod`'s `go` directive, and `test (stable)` the latest Go release. Weekly runs fuzz each target for 10 minutes instead of 20 seconds and refresh the vulnerability data.

## Supply-chain policy

- **Pinning.** Every action and reusable workflow is pinned to a full commit SHA, resolved from its upstream repository, with a `# vX.Y.Z` comment. Tags, including floating major-version tags, can be moved, so a tag pin does not fix what runs. The shared code-scan workflow (`entur/gha-security`) is pinned by SHA as well; that also pins the local actions it references with `$/`.
- **Permissions.** Every workflow sets `permissions: {}` at the top and grants permissions per job. Checkouts use `persist-credentials: false`. There are no caches, no repository secrets, no `secrets: inherit` and no `pull_request_target`, and `GOTOOLCHAIN: local` is set so the matrix runs exactly the Go versions it names.
- **CodeQL.**
  - Pull requests: a local job (`go` with a manual build, plus `actions`) using the same queries and categories as the shared workflow.
  - `main` and the weekly run: the shared workflow, granted exactly the permission union its jobs declare at the pinned SHA.
  - Why not the shared workflow on pull requests: its scanner job reads an allowlist from the checked-out tree and can dismiss alerts, so it must not run on contributed code. Do not add that allowlist file to this repository.
  - When Dependabot bumps the shared workflow, re-read its job permissions and update the grant.
- **Workflow checks.** Run `actionlint` and `zizmor --offline .github/workflows` after every workflow change; CI runs zizmor too.
- **Tools.** `govulncheck` and `modhash` live in the nested `tools/` module, so they never become dependencies of the library. golangci-lint runs through its action at an exact version (`v2.15.0`).
- **Dependabot.**
  - Monthly, with a 7-day cooldown for version updates; security updates are not delayed.
  - Minor and patch updates are grouped; major updates arrive as individual pull requests.
  - Commit prefixes `ci(deps)` and `chore(deps)` never trigger a release.
- **Not adopted, with reasons.**
  - harden-runner: no job holds a secret. Revisit if one ever does.
  - GitHub workflow lockfiles: adopt them when they leave public preview.

## Releases

1. release-please opens or updates a release pull request on every push to `main`. The pull request is created with `GITHUB_TOKEN`, so its workflow runs start in an approval-required state: a maintainer approves the runs, reviews and merges.
2. On merge, release-please creates the tag and an immutable GitHub release. `verify-module` then:
   - downloads the version through proxy.golang.org, which records it in sum.golang.org;
   - checks that its `h1:` hash equals `modhash -rev <tag>` of the tagged tree;
   - runs `gh release verify`.
3. The release workflow then runs the tag audit explicitly, because a tag pushed with `GITHUB_TOKEN` does not trigger `tag-guard.yml`.
4. **A bad release** is fixed forward:
   - a `fix:` pull request adds `retract vX.Y.Z // reason` to `go.mod`;
   - a maintainer edits the GitHub release title to say "retracted";
   - tags are never deleted or moved.

## Tag audit

`.github/scripts/tag-audit.sh` checks every `v*` tag with read-only API calls. Each tag must point at a commit that:

- has the release-please release subject for its version;
- is on `main`;
- records that version in `.release-please-manifest.json`;
- came from a merged `release-please--branches--main` pull request by an allowed author.

The allowed authors are `ALLOWED_RELEASE_AUTHORS` at the top of the script; add the release App's bot login there when one is introduced. `.github/scripts/tag-audit_test.sh` tests the script against a fake `gh`. A failed audit opens an issue. The audit detects stray tags after they are created; it does not prevent their publication or consumption.

## Scorecard

Scorecard runs weekly with published results. Its Signed-Releases check ignores source-only releases and may report an inconclusive or not-applicable result, because this module publishes no release assets. Record the result from the pinned Scorecard version rather than expecting a particular score.

## Repository settings checklist (repository admin, before the first release)

- [ ] **Ruleset `main`:**
  - pull request required, 1 approval, code-owner review, dismiss stale approvals, approval of the latest push, conversation resolution;
  - squash merge only, linear history, signed commits;
  - required checks: the CI jobs above, `CodeQL (go)`, `CodeQL (actions)`, and the code-scanning results gate;
  - block force pushes and deletion; no bypass actors.
- [ ] **Ruleset `v*` tags:** restrict updates and deletions, block force pushes. Restricting tag creation needs a release GitHub App as the only bypass actor, because `GITHUB_TOKEN` cannot be one. Until then the tag audit detects stray tags.
- [ ] **Immutable releases** enabled before the first release (they apply only to later releases).
- [ ] **Actions settings:**
  - read-only default token;
  - "Allow GitHub Actions to create and approve pull requests" on (release-please);
  - fork pull request workflows require approval;
  - require full-length SHA pins once a test pull request shows that the shared code-scan workflow's `$/` references pass.
- [ ] **Security features:** private vulnerability reporting, Dependabot alerts and security updates, secret scanning and push protection.
- [ ] **Merging:** squash merge only; delete head branches on merge. The owning team (see `CODEOWNERS`) has admin access.
- [ ] **Scorecard:** record the first result.

## Organisation-admin follow-ups

- **Release App.** Install a repository-scoped release GitHub App. Then:
  - switch release-please to its token, which removes the manual "approve workflows" step;
  - add the App as the only bypass actor of a "restrict creations" rule on `v*` tags;
  - add its bot login to `ALLOWED_RELEASE_AUTHORS`;
  - turn off "Allow GitHub Actions to create and approve pull requests".
- **Organisation policies**, if repository-level settings are locked: the immutable-releases policy and the SHA-pinning policy.
