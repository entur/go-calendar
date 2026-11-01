#!/usr/bin/env bash
# Audits every v* tag of the repository using read-only GitHub API calls
# (gh api). It never checks out or runs code from the tags it inspects.
#
# A tag passes only if the commit it points at
#   1. has the subject of the release-please release commit for the tag's
#      version ("chore(main): release X.Y.Z", optionally " (#N)");
#   2. is on main;
#   3. records the version in its root .release-please-manifest.json; and
#   4. came from a merged pull request whose head branch is release-please's
#      and whose author is an allowed release author.
# The subject alone proves nothing: anyone who can merge can choose a title.
#
# Environment: REPO (owner/name) and GH_TOKEN (contents: read).
# Exit status: 0 when every tag passes, 1 otherwise.
set -euo pipefail

# Accounts allowed to open release pull requests. When a release GitHub App
# replaces GITHUB_TOKEN, add its bot login here.
ALLOWED_RELEASE_AUTHORS=("github-actions[bot]")
RELEASE_BRANCH_PREFIX="release-please--branches--main"

: "${REPO:?REPO must be set to owner/name}"

fail=0
error() {
	echo "::error::$*"
	fail=1
}

allowed_author() {
	local author="$1" allowed
	for allowed in "${ALLOWED_RELEASE_AUTHORS[@]}"; do
		if [ "${author}" = "${allowed}" ]; then
			return 0
		fi
	done
	return 1
}

# from_release_pr reports whether commit $1 came from a merged release-please
# pull request opened by an allowed author.
from_release_pr() {
	local pulls head author
	if ! pulls="$(gh api "repos/${REPO}/commits/$1/pulls" \
		--jq '.[] | select(.merged_at != null) | [.head.ref, .user.login] | @tsv')"; then
		return 1
	fi
	while IFS=$'\t' read -r head author; do
		case "${head}" in
		"${RELEASE_BRANCH_PREFIX}"*)
			if allowed_author "${author}"; then
				return 0
			fi
			;;
		*) ;;
		esac
	done <<<"${pulls}"
	return 1
}

refs="$(gh api --paginate "repos/${REPO}/git/matching-refs/tags/v" \
	--jq '.[] | [.ref, .object.type, .object.sha] | @tsv')"
while IFS=$'\t' read -r ref type sha; do
	[ -n "${ref}" ] || continue
	tag="${ref#refs/tags/}"
	if [ "${type}" = "tag" ]; then
		if ! sha="$(gh api "repos/${REPO}/git/tags/${sha}" --jq '.object.sha')"; then
			error "tag ${tag}: cannot peel the annotated tag"
			continue
		fi
	fi
	version="${tag#v}"
	if ! [[ "${version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
		error "tag ${tag} is not a release version tag"
		continue
	fi
	if ! subject="$(gh api "repos/${REPO}/commits/${sha}" --jq '.commit.message | split("\n") | .[0]')"; then
		error "tag ${tag} (${sha}): cannot read the commit"
		continue
	fi
	expected="^chore\(main\): release ${version//./\\.}( \(#[0-9]+\))?\$"
	if ! [[ "${subject}" =~ ${expected} ]]; then
		error "tag ${tag} (${sha}) does not point at the release-please commit for ${version}"
		continue
	fi
	status="$(gh api "repos/${REPO}/compare/main...${sha}" --jq '.status')" || status="unknown"
	if [ "${status}" != "behind" ] && [ "${status}" != "identical" ]; then
		error "tag ${tag} (${sha}) is not on main (${status})"
		continue
	fi
	# A missing manifest (HTTP 404) counts as a mismatch.
	manifest="$(gh api -H "Accept: application/vnd.github.raw+json" \
		"repos/${REPO}/contents/.release-please-manifest.json?ref=${sha}" --jq '."."' 2>/dev/null)" || manifest=""
	if [ "${manifest}" != "${version}" ]; then
		error "tag ${tag} (${sha}): .release-please-manifest.json at that commit does not record ${version}"
		continue
	fi
	if ! from_release_pr "${sha}"; then
		error "tag ${tag} (${sha}) did not come from a merged release-please pull request by an allowed author"
		continue
	fi
	echo "tag ${tag} -> ${sha}: ok"
done <<<"${refs}"
exit "${fail}"
