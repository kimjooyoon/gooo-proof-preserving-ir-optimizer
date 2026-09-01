#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 3 ]]; then
  echo "usage: conformance.sh REPOSITORY BINARY OUTPUT" >&2
  exit 64
fi

repository=$1
binary=$2
output=$3
before=$(git -C "$repository" status --porcelain=v1 -z --untracked-files=all | sha256sum | awk '{print $1}')
mkdir -p "$output"

"$binary" conformance \
  --meta "$repository/.gooo/proof-preserving-ir-optimizer.gooo" \
  --root "$repository" \
  --out "$output/cases"

jq -e '
  .schema == "gooo-proof-preserving-ir-optimizer/conformance/v1" and
  .decision == "CLOSED" and
  .fixed_denominator == 8 and
  .summary == {total_cases:8,closed:5,unknown:1,refuted:2} and
  .observed_precedence == ["REFUTED","UNKNOWN","CLOSED"] and
  .authority == {repository_writes:0,output_scope:"CALLER_OWNED_TEMP_OUTPUT_ONLY",automatic_commit:0,automatic_push:0,automatic_merge:0,automatic_release:0} and
  .generated.generated_go_files == 16 and
  .generated.generated_binaries == 16 and
  .no_aggregate_metrics == true and
  ([.cases[] | select(.id=="operator-constant-fold") | .observed] == ["CLOSED"]) and
  ([.cases[] | select(.id=="operator-dead-branch") | .observed] == ["CLOSED"]) and
  ([.cases[] | select(.id=="operator-deterministic-cse") | .observed] == ["CLOSED"]) and
  ([.cases[] | select(.id=="comment-only") | .observed] == ["CLOSED"]) and
  ([.cases[] | select(.id=="effectful-branch-removal") | .observed] == ["REFUTED"]) and
  ([.cases[] | select(.id=="origin-loss") | .observed] == ["REFUTED"]) and
  ([.cases[] | select(.id=="missing-cost-witness") | .observed] == ["UNKNOWN"]) and
  ([.cases[] | select(.id=="replay") | .observed] == ["CLOSED"]) and
  ([.cases[] | select(.observed=="UNKNOWN") |
    (.unknown.stage != "" and .unknown.step != "" and .unknown.reason != "" and .unknown.unknown_class != "" and .unknown.next_operation != "" and (.unknown.blocked_by|length)>0)] | all) and
  ([.cases[] | select(.observed=="CLOSED") |
    (.before.binary_bytes|type)=="number" and (.before.generated_bytes|type)=="number" and
    (.before.wall_ms|type)=="number" and (.before.peak_rss_kib|type)=="number" and
    (.after.binary_bytes|type)=="number" and (.after.generated_bytes|type)=="number" and
    (.after.wall_ms|type)=="number" and (.after.peak_rss_kib|type)=="number" and
    .improvement.exact_pair == true] | all) and
  ([.cases[] | select(.observed=="CLOSED") | .pair_identity |
    (.scenario_id != "" and .source_digest != "" and .contract_digest != "" and .toolchain_digest != "" and .runner_digest != "")] | all) and
  ([.. | objects | keys[]? | select(test("score|percentage|average|weighted|estimate"; "i"))] | length) == 0
' "$output/cases/conformance.json" >/dev/null

after=$(git -C "$repository" status --porcelain=v1 -z --untracked-files=all | sha256sum | awk '{print $1}')
test "$before" = "$after"
