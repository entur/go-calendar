# Security policy

## Reporting a vulnerability

Report suspected vulnerabilities privately through GitHub's private vulnerability reporting: open the repository's **Security** tab and choose **Report a vulnerability**. Please do not open a public issue.

Include what you found, how to reproduce it, and the affected versions. We acknowledge reports within 5 working days, and triage and fix confirmed vulnerabilities within 30 days. We coordinate disclosure with you and credit you in the advisory unless you ask us not to.

## Scope

In scope:

- the Go module `github.com/entur/go-calendar` (packages `calendar`, `snapshot` and the command `cmd/calendar-snapshot`);
- the repository's GitHub Actions workflows and release process.

Incorrect holiday dates are bugs, not vulnerabilities, unless they let someone tamper with a released module or a frozen snapshot. Report them as ordinary issues.

## Supported versions

Only the latest release receives fixes. A bad release is fixed forward and retracted in `go.mod`; tags are never moved or deleted.

## Verifying a release

See "Verifying a release" in the [README](README.md#verifying-a-release).
