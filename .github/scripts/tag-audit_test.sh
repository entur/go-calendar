#!/usr/bin/env bash
# Tests tag-audit.sh against a fake gh on PATH; no network.
#   bash .github/scripts/tag-audit_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT
mkdir -p "${work}/bin"

# The fake answers the read-only API calls tag-audit.sh makes from FAKE_*
# variables, already filtered the way the script's --jq expressions would
# filter real responses (checked against real repositories when written).
cat >"${work}/bin/gh" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
url=""
for arg in "$@"; do
	case "${arg}" in repos/*) url="${arg}" ;; *) ;; esac
done
case "${url}" in
*/git/matching-refs/tags/v) printf '%b' "${FAKE_REFS:-}" ;;
*/git/tags/*) printf '%s\n' "${FAKE_PEELED:?}" ;;
*/commits/*/pulls) printf '%b' "${FAKE_PULLS:-}" ;;
*/commits/*) printf '%s\n' "${FAKE_SUBJECT:?}" ;;
*/compare/*) printf '%s\n' "${FAKE_STATUS:?}" ;;
*/contents/.release-please-manifest.json*)
	if [ -z "${FAKE_MANIFEST:-}" ]; then
		echo '{"message":"Not Found","status":"404"}'
		echo 'gh: Not Found (HTTP 404)' >&2
		exit 1
	fi
	printf '%s\n' "${FAKE_MANIFEST}"
	;;
*)
	echo "fake gh: unexpected call: $*" >&2
	exit 9
	;;
esac
FAKE
chmod +x "${work}/bin/gh"

sha="1111111111111111111111111111111111111111"
tagobj="2222222222222222222222222222222222222222"
failures=0

# check NAME WANT_EXIT WANT_OUTPUT [VAR=value ...]
check() {
	local name="$1" want="$2" pattern="$3" out code=0
	shift 3
	out="$(env PATH="${work}/bin:${PATH}" REPO=entur/go-calendar "$@" bash "${here}/tag-audit.sh" 2>&1)" || code=$?
	if [ "${code}" -ne "${want}" ] || ! grep -q -- "${pattern}" <<<"${out}"; then
		echo "FAIL ${name}: exit ${code} (want ${want}), output must match ${pattern}:"
		printf '    %s\n' "${out//$'\n'/$'\n'    }"
		failures=$((failures + 1))
		return
	fi
	echo "ok   ${name}"
}

genuine=(
	"FAKE_REFS=refs/tags/v0.1.0\tcommit\t${sha}\n"
	"FAKE_SUBJECT=chore(main): release 0.1.0 (#12)"
	"FAKE_STATUS=behind"
	"FAKE_MANIFEST=0.1.0"
	"FAKE_PULLS=release-please--branches--main\tgithub-actions[bot]\n"
)

check "genuine release-please commit" 0 "tag v0.1.0 -> ${sha}: ok" "${genuine[@]}"
check "genuine, subject without PR number, at main's head" 0 ": ok" "${genuine[@]}" \
	"FAKE_SUBJECT=chore(main): release 0.1.0" "FAKE_STATUS=identical"
check "annotated tag is peeled" 0 ": ok" "${genuine[@]}" \
	"FAKE_REFS=refs/tags/v0.1.0\ttag\t${tagobj}\n" "FAKE_PEELED=${sha}"
check "no tags" 0 "" "FAKE_REFS="
check "matching subject from an ordinary pull request" 1 "did not come from a merged release-please pull request" \
	"${genuine[@]}" "FAKE_PULLS=feature/notes\tsomeone\n"
check "release branch opened by another author" 1 "did not come from a merged release-please pull request" \
	"${genuine[@]}" "FAKE_PULLS=release-please--branches--main\tsomeone\n"
check "no merged pull request" 1 "did not come from a merged release-please pull request" \
	"${genuine[@]}" "FAKE_PULLS="
check "manifest mismatch" 1 "does not record 0.1.0" "${genuine[@]}" "FAKE_MANIFEST=0.0.0"
check "manifest missing" 1 "does not record 0.1.0" "${genuine[@]}" "FAKE_MANIFEST="
check "off main" 1 "is not on main (diverged)" "${genuine[@]}" "FAKE_STATUS=diverged"
check "ahead of main" 1 "is not on main (ahead)" "${genuine[@]}" "FAKE_STATUS=ahead"
check "subject for another version" 1 "does not point at the release-please commit" \
	"${genuine[@]}" "FAKE_SUBJECT=chore(main): release 0.1.1 (#12)"
check "not a version tag" 1 "is not a release version tag" "FAKE_REFS=refs/tags/v1\tcommit\t${sha}\n"
check "forged v99.0.0 (review round 1)" 1 "::error::tag v99.0.0" \
	"FAKE_REFS=refs/tags/v99.0.0\tcommit\t${sha}\n" "FAKE_SUBJECT=chore(main): release 99.0.0 (#123)" \
	"FAKE_STATUS=behind" "FAKE_MANIFEST=0.0.0" "FAKE_PULLS=feature/x\tsomeone\n"
check "one bad tag fails the audit" 1 "::error::tag v99.0.0" "${genuine[@]}" \
	"FAKE_REFS=refs/tags/v0.1.0\tcommit\t${sha}\nrefs/tags/v99.0.0\tcommit\t${sha}\n"

if [ "${failures}" -ne 0 ]; then
	echo "${failures} case(s) failed"
	exit 1
fi
echo "all cases passed"
