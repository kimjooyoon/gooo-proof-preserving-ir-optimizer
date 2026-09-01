#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "$0")/.." && pwd)
meta="$repo_root/.gooo/proof-preserving-ir-optimizer.gooo"

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

if rg -nE 'git (commit|merge|push|reset|checkout)|gh (pr merge|release (delete|edit))' "$repo_root/cmd" "$repo_root/internal"; then
  echo 'automatic repository integration authority is forbidden' >&2
  exit 1
fi
if rg -n 'GITHUB_TOKEN.*immutable-releases|immutable-releases.*GITHUB_TOKEN' "$repo_root/.github"; then
  echo 'GITHUB_TOKEN may not query immutable release admin settings' >&2
  exit 1
fi
echo 'semantic_audit=CLOSED'
