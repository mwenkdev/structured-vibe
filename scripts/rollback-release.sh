#!/usr/bin/env bash
#
# Rolls back a Structured Vibe release transaction.
#
#   rollback-release.sh <version> <prerelease> [repo]
#
# Called by .github/workflows/release.yml when the release pipeline reaches any
# observable non-success conclusion: step failure, manual cancellation, or job
# timeout. See docs/specs/release-rollback-coverage.md (decisions D2, D3, D4)
# and docs/specs/releasing.md, which stays authoritative for release mechanics.
#
# Arguments:
#   version     the release version/tag, e.g. 1.2.3 (required, non-empty)
#   prerelease  the GitHub prerelease flag, exactly "true" or "false"
#   repo        owner/name (default: $GITHUB_REPOSITORY)
#
# Environment:
#   GH          the gh executable (default: gh). Tests inject a stub here.
#   GH_TOKEN    passed through to gh for authentication.
#
# Behavior (D2): delete what exists, tolerate what is already absent, re-check
# the end state, and fail naming any surviving object. The operation is safe to
# run twice. It deliberately does not clean individual assets, create retry
# tags, move an existing tag, retry against a failing API, or attempt to repair
# a failed rollback.
#
# Prereleases (D4) are exempt: the script is a complete no-op and reports the
# exemption rather than deleting anything.
#
# Absence is detected without parsing human-readable error text. Existence
# queries return HTTP 200 with a possibly-empty JSON result, so "absent" is an
# empty success rather than an error, and a genuine API failure stays
# distinguishable from a missing object.

# Deliberately no -e: this script tolerates individual failures and inspects
# them explicitly. An early exit would skip the end-state verification that is
# the entire point of D2.
set -uo pipefail

GH="${GH:-gh}"

log() { printf '%s\n' "$*"; }
warn() { printf '::warning::%s\n' "$*"; }
die() {
	printf '::error::%s\n' "$*" >&2
	exit 1
}

version="${1:-}"
prerelease="${2:-}"
repo="${3:-${GITHUB_REPOSITORY:-}}"

# A cancelled run can reach rollback before the step that would have resolved a
# version output, so an empty version is a real possibility and must fail loudly
# rather than roll back "" (D3).
if [[ -z "$version" ]]; then
	die "rollback requires a release version as the first argument; got an empty value"
fi

case "$prerelease" in
	true | false) ;;
	"")
		die "rollback requires the prerelease flag as the second argument (true or false); got an empty value"
		;;
	*)
		die "rollback requires the prerelease flag to be exactly 'true' or 'false'; got '$prerelease'"
		;;
esac

if [[ -z "$repo" ]]; then
	die "rollback requires a repository as owner/name, passed as the third argument or GITHUB_REPOSITORY"
fi

# The GitHub prerelease flag is authoritative, not the SemVer suffix (D4). The
# two can disagree, and silently preferring either would surprise someone, so
# the disagreement is reported while the flag decides.
# Build metadata is stripped first: 1.2.3+linux-amd64 carries a hyphen without
# being a prerelease, and warning about it would be noise.
has_suffix=false
if [[ "${version%%+*}" == *-* ]]; then
	has_suffix=true
fi

if [[ "$prerelease" == true && "$has_suffix" == false ]]; then
	warn "release '$version' has no SemVer prerelease suffix but the GitHub prerelease flag is true; the flag decides and the release is retained"
elif [[ "$prerelease" == false && "$has_suffix" == true ]]; then
	warn "release '$version' has a SemVer prerelease suffix but the GitHub prerelease flag is false; the flag decides and the release is rolled back"
fi

if [[ "$prerelease" == true ]]; then
	log "prerelease '$version' is exempt from automatic rollback; retaining the release, its tag, and any assets already uploaded"
	log "verify the prerelease by hand: a non-success pipeline may have left an incomplete asset set"
	exit 0
fi

# Existence queries return HTTP 200 with a possibly-empty JSON array, so absence
# is an empty success rather than an error and stays distinguishable from a
# genuine API failure.
#
# The version is deliberately NOT interpolated into the jq filters. A tag name
# may legitimately contain characters such as a double quote, and rollback has
# to work for a malformed version precisely because version-format validation is
# one of the failures it compensates for. The filters therefore emit raw rows and
# the exact match is made in shell, where the value is data rather than code.

# Emits one "<tag_name><TAB><id>" row per release.
release_rows() {
	"$GH" api "repos/$repo/releases" \
		--paginate \
		--jq '.[] | [.tag_name, (.id | tostring)] | @tsv'
}

# Emits one ref per matching tag. Matching refs is a prefix query, so a request
# for tags/1.2.3 also returns 1.2.30 and the exact ref must be selected here.
tag_rows() {
	"$GH" api "repos/$repo/git/matching-refs/tags/$version" \
		--jq '.[].ref'
}

# Prints the id of the release whose tag is exactly $version, or nothing when no
# such release exists. Returns non-zero only when the query itself failed.
release_id() {
	local rows tag id
	rows=$(release_rows) || return 1
	while IFS=$'\t' read -r tag id; do
		if [[ "$tag" == "$version" ]]; then
			printf '%s' "$id"
			return 0
		fi
	done <<< "$rows"
	return 0
}

# Prints the tag ref when it exists exactly, otherwise nothing. Returns non-zero
# only when the query itself failed.
tag_ref() {
	local rows ref
	rows=$(tag_rows) || return 1
	while read -r ref; do
		if [[ "$ref" == "refs/tags/$version" ]]; then
			printf '%s' "$ref"
			return 0
		fi
	done <<< "$rows"
	return 0
}

if ! id=$(release_id); then
	die "could not determine whether release '$version' exists in $repo; rollback did not run and the release may still exist"
fi

if [[ -n "$id" ]]; then
	log "deleting release '$version' (id $id)"
	# Not fatal on its own: the end-state re-check below decides the exit status,
	# so this is reported as a warning rather than an error that would claim the
	# step failed even when the object turns out to be gone.
	if ! "$GH" api --method DELETE "repos/$repo/releases/$id" --silent; then
		warn "the delete call for release '$version' (id $id) failed; verifying the end state"
	fi
else
	log "release '$version' is already absent"
fi

if ! ref=$(tag_ref); then
	die "could not determine whether tag '$version' exists in $repo; the release or tag may still exist"
fi

if [[ -n "$ref" ]]; then
	log "deleting tag '$version'"
	if ! "$GH" api --method DELETE "repos/$repo/git/refs/tags/$version" --silent; then
		warn "the delete call for tag '$version' failed; verifying the end state"
	fi
else
	log "tag '$version' is already absent"
fi

# A partial rollback currently reports success, which is the defect D2 fixes.
# Re-check both objects and let the end state decide the exit status.
if ! remaining_id=$(release_id); then
	die "could not verify the end state of release '$version' in $repo; treat the rollback as incomplete"
fi

if ! remaining_ref=$(tag_ref); then
	die "could not verify the end state of tag '$version' in $repo; treat the rollback as incomplete"
fi

if [[ -n "$remaining_id" || -n "$remaining_ref" ]]; then
	survivors=""
	verb="still exists"
	if [[ -n "$remaining_id" ]]; then
		survivors="release '$version' (id $remaining_id)"
	fi
	if [[ -n "$remaining_ref" ]]; then
		if [[ -n "$survivors" ]]; then
			survivors="$survivors and tag '$version'"
			verb="still exist"
		else
			survivors="tag '$version'"
		fi
	fi
	die "rollback incomplete in $repo: $survivors $verb; delete by hand before recreating the release"
fi

log "rolled back release '$version' and its tag in $repo"
