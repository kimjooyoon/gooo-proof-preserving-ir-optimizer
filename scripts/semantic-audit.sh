#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "$0")/.." && pwd)
meta="$repo_root/.gooo/proof-preserving-ir-optimizer.gooo"
reuse="$repo_root/contracts/release-reuse-v1.json"

test "$(grep -c '^  scenario ' "$meta")" -eq 8
grep -q '^  precedence "REFUTED" "UNKNOWN" "CLOSED"$' "$meta"
grep -q '^  denominator "proof-preserving-ir-optimizer-v1" count "8"$' "$meta"
grep -q '^  rewrite "constant-fold" ' "$meta"
grep -q '^  rewrite "dead-branch" ' "$meta"
grep -q '^  rewrite "deterministic-cse" ' "$meta"
grep -q '^  forbidden_effect "IO" ' "$meta"
grep -q '^  origin_map "source-origin" "derived-origin" preservation "all-source-anchors"$' "$meta"
grep -q '^  cost_observation vector ' "$meta"
grep -q '^  unknown_fields "stage" "step" "reason" "unknown_class" "next_operation" "blocked_by"$' "$meta"
grep -q '^  authority_rule .*repository_writes "0" .*automatic_release "0"$' "$meta"
jq -e '
  .schema == "gooo-proof-preserving-ir-optimizer/release-reuse/v1" and
  .fixed_vector == {total_cases:8,closed:5,unknown:1,refuted:2} and
  .generated == {files:16,bytes:19032,binary_bytes:62154194} and
  .duplicate_release_baseline.expensive_executions == 2 and
  .duplicate_release_target.expensive_executions == 0 and
  .duplicate_release_target.decision == "CLOSED" and
  .physical_runner_comparison.decision == "UNKNOWN"
' "$reuse" >/dev/null

if rg -nE 'git (commit|merge|push|reset|checkout)|gh (pr merge|release (delete|edit))' "$repo_root/cmd" "$repo_root/internal"; then
  echo 'automatic repository integration authority is forbidden' >&2
  exit 1
fi
if rg -n 'secrets\.|IMMUTABLE_RELEASES_API_TOKEN|immutable-releases|/settings/|actions/secrets|actions/permissions' "$repo_root/.github"; then
  echo 'release workflow may not use user secrets or Actions admin/capability endpoints' >&2
  exit 1
fi
echo 'semantic_audit=CLOSED'
