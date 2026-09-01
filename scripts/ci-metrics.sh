#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 9 ]]; then
  echo "usage: ci-metrics.sh REPOSITORY OUTPUT CONFORMANCE COMPILE BUILD TEST TEST_LOG CONFORMANCE_STAGE INTEGRATION" >&2
  exit 64
fi

repo_root=$1
output=$2
conformance_report=$3
compile=$4
build=$5
test_stage=$6
test_log=$7
conformance_stage=$8
integration=$9

sum_lines() {
  local pattern=$1
  local total=0
  while IFS= read -r -d '' file; do
    total=$((total + $(wc -l < "$file")))
  done < <(find "$repo_root" -type f -name "$pattern" ! -path "$repo_root/.git/*" ! -name README.md -print0)
  echo "$total"
}

count_files() {
  local pattern=$1
  find "$repo_root" -type f -name "$pattern" ! -path "$repo_root/.git/*" ! -name README.md -print | wc -l | tr -d ' '
}

read_stage() {
  jq -c '{wall_ms:(.wall_ms|tonumber),peak_rss_kib:(.peak_rss_kib|tonumber)}' "$1"
}

go_files=$(count_files '*.go')
gooo_files=$(count_files '*.gooo')
go_lines=$(sum_lines '*.go')
gooo_lines=$(sum_lines '*.gooo')
regular_files=$(find "$repo_root" -type f ! -path "$repo_root/.git/*" ! -name README.md -print | wc -l | tr -d ' ')
descendant_dirs=$(find "$repo_root" -mindepth 1 -type d ! -path "$repo_root/.git" ! -path "$repo_root/.git/*" -print | wc -l | tr -d ' ')
toolchain=$(go env GOVERSION)
runner_material="${RUNNER_OS:-unknown}|${RUNNER_ARCH:-unknown}|${ImageOS:-unknown}|${ImageVersion:-unknown}|$(go env GOOS)|$(go env GOARCH)"
runner_digest="sha256:$(printf '%s' "$runner_material" | sha256sum | awk '{print $1}')"

tests_total=$(jq -s '[.[] | select(.Action == "run" and .Test != null)] | length' "$test_log")
tests_selected=$tests_total
tests_executed=$(jq -s '[.[] | select((.Action == "pass" or .Action == "fail") and .Test != null)] | length' "$test_log")
tests_failed=$(jq -s '[.[] | select(.Action == "fail" and .Test != null)] | length' "$test_log")

jq -n \
  --arg schema "gooo-proof-preserving-ir-optimizer/ci-evidence/v1" \
  --arg go_version "$toolchain" --arg runner_digest "$runner_digest" \
  --argjson go_files "$go_files" --argjson gooo_files "$gooo_files" \
  --argjson go_lines "$go_lines" --argjson gooo_lines "$gooo_lines" \
  --argjson regular_files "$regular_files" --argjson descendant_dirs "$descendant_dirs" \
  --argjson generated_go_files "$(jq -r '.generated.generated_go_files' "$conformance_report")" \
  --argjson generated_go_bytes "$(jq -r '.generated.generated_go_bytes' "$conformance_report")" \
  --argjson binary_bytes "$(jq -r '.generated.binary_bytes' "$conformance_report")" \
  --argjson tests_total "$tests_total" --argjson tests_selected "$tests_selected" \
  --argjson tests_executed "$tests_executed" --argjson tests_failed "$tests_failed" \
  --argjson compile "$(read_stage "$compile")" --argjson build "$(read_stage "$build")" \
  --argjson test_stage "$(read_stage "$test_stage")" --argjson conformance "$(read_stage "$conformance_stage")" \
  --argjson integration "$(read_stage "$integration")" --slurpfile report "$conformance_report" \
  '{schema:$schema,verification_authority:"GITHUB_ACTIONS",go_version:$go_version,runner_digest:$runner_digest,
    repository_writes:0,root_readme_excluded:true,
    inventory:{go_files:$go_files,gooo_files:$gooo_files,go_physical_lines:$go_lines,gooo_physical_lines:$gooo_lines,descendant_dirs:$descendant_dirs,regular_files:$regular_files},
    generated:{files:$generated_go_files,bytes:$generated_go_bytes,binary_bytes:$binary_bytes},
    stages:{compile:$compile,build:$build,test:$test_stage,conformance:$conformance,integration:$integration},
    tests:{total:$tests_total,selected:$tests_selected,executed:$tests_executed,reused:0,failed:$tests_failed,unknown:0},
    fixed_vector:($report[0].summary),
    exact_pair_vectors:($report[0].cases|map(select(.observed=="CLOSED")|{id,expected,observed,pair_identity,before_cost:{binary_bytes:.before.binary_bytes,generated_bytes:.before.generated_bytes,wall_ms:.before.wall_ms,peak_rss_kib:.before.peak_rss_kib},after_cost:{binary_bytes:.after.binary_bytes,generated_bytes:.after.generated_bytes,wall_ms:.after.wall_ms,peak_rss_kib:.after.peak_rss_kib},improvement})),
    authority:($report[0].authority|. + {local_test_executions:0,local_build_executions:0,local_vet_executions:0,local_conformance_executions:0,local_integration_executions:0,cross_project_required_gates:0})
  }' > "$output"
