#!/usr/bin/env bash
set -euo pipefail

repository="flavio-fernandes/go-aioesphomeapi"
status_context="codex-review"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
audit_script="${script_dir}/../.agents/skills/merge-reviewed-pr/scripts/audit_review_threads.py"

usage() {
	cat >&2 <<'EOF'
Usage: ./tools/codex-review.sh request PR_NUMBER
       ./tools/codex-review.sh complete PR_NUMBER

request   Post one explicit exact-head @codex review request.
complete  Audit that review and publish the required codex-review status.
EOF
}

if [[ $# -ne 2 ]] || [[ ! "$2" =~ ^[1-9][0-9]*$ ]]; then
	usage
	exit 2
fi

action="$1"
pull_request="$2"
if [[ "$action" != "request" && "$action" != "complete" ]]; then
	usage
	exit 2
fi

if ! command -v gh >/dev/null 2>&1; then
	echo "GitHub CLI is required; install gh and run gh auth login." >&2
	exit 2
fi
if ! command -v python3 >/dev/null 2>&1; then
	echo "Python 3 is required for the thread-aware review audit." >&2
	exit 2
fi
if ! gh auth status --hostname github.com >/dev/null 2>&1; then
	echo "GitHub authentication is unavailable; run gh auth login." >&2
	exit 2
fi
if [[ ! -s "$audit_script" ]]; then
	echo "Review audit is missing: $audit_script" >&2
	exit 2
fi

pr_data="$(gh pr view "$pull_request" --repo "$repository" \
	--json headRefOid,url,state --jq '[.headRefOid, .url, .state] | @tsv')"
IFS=$'\t' read -r head_sha pull_request_url pull_request_state <<<"$pr_data"
if [[ "$pull_request_state" != "OPEN" ]]; then
	echo "PR #${pull_request} is not open." >&2
	exit 2
fi

set_status() {
	local state="$1"
	local description="$2"
	gh api --method POST "repos/${repository}/statuses/${head_sha}" \
		--raw-field "state=${state}" \
		--raw-field "context=${status_context}" \
		--raw-field "description=${description}" \
		--raw-field "target_url=${pull_request_url}" >/dev/null
}

if [[ "$action" == "request" ]]; then
	existing_request="$(gh api --paginate \
		"repos/${repository}/issues/${pull_request}/comments" \
		--jq ".[] | select(.body | startswith(\"@codex review\")) | select(.body | contains(\"${head_sha}\")) | .id")"
	if [[ -n "$existing_request" ]]; then
		echo "Codex review was already requested for PR #${pull_request} at ${head_sha}; refusing a duplicate paid request." >&2
		exit 1
	fi

	set_status pending "Explicit Codex review requested"
	comment="$(printf '@codex review\n\nPlease review exact head `%s`.' "$head_sha")"
	gh pr comment "$pull_request" --repo "$repository" --body "$comment" >/dev/null
	echo "requested Codex review for PR #${pull_request} at ${head_sha}"
	echo "run ./tools/codex-review.sh complete ${pull_request} after the review finishes"
	exit 0
fi

if python3 "$audit_script" "$repository" "$pull_request" --require-codex; then
	set_status success "Exact-head Codex review is complete"
	echo "codex-review status is successful for PR #${pull_request} at ${head_sha}"
	exit 0
fi

set_status failure "Codex review is incomplete or has open threads"
echo "codex-review remains blocking; finish the exact-head review and resolve every thread" >&2
exit 1
