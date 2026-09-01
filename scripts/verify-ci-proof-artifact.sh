#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 4 ]]; then
  echo "usage: verify-ci-proof-artifact.sh REPOSITORY SOURCE_RUN_ID EXPECTED_SHA OUTPUT" >&2
  exit 64
fi

repository=$1
source_run_id=$2
expected_sha=$3
output=$4

case "$source_run_id" in
  ''|*[!0-9]*) echo 'SOURCE_RUN_ID must be an integer' >&2; exit 64 ;;
esac

mkdir -p "$output/proof" "$output/manifest"
source_run="$output/source-run.json"
artifacts="$output/artifacts.json"
gh api "repos/$repository/actions/runs/$source_run_id" > "$source_run"
gh api "repos/$repository/actions/runs/$source_run_id/artifacts" > "$artifacts"

workflow_name='Proof-preserving IR optimizer conformance'
workflow_file='.github/workflows/conformance.yml'
evidence_name="gooo-proof-preserving-ir-optimizer-evidence-$source_run_id"
manifest_name="gooo-proof-preserving-ir-optimizer-proof-manifest-$source_run_id"

jq -e --arg repository "$repository" --arg sha "$expected_sha" --arg workflow "$workflow_name" --arg workflow_file "$workflow_file" '
  .repository.full_name == $repository and .name == $workflow and .path == $workflow_file and
  .event == "push" and .head_branch == "main" and .head_sha == $sha and
  .conclusion == "success" and (.run_attempt|type) == "number" and .run_attempt >= 1
' "$source_run" >/dev/null

evidence=$(jq -cer --arg name "$evidence_name" --argjson run_id "$source_run_id" '
  [.artifacts[] | select(.name == $name and .workflow_run.id == $run_id and .expired == false)]
  | if length == 1 then .[0] else error("evidence artifact is not unique and unexpired") end
' "$artifacts")
manifest_artifact=$(jq -cer --arg name "$manifest_name" --argjson run_id "$source_run_id" '
  [.artifacts[] | select(.name == $name and .workflow_run.id == $run_id and .expired == false)]
  | if length == 1 then .[0] else error("manifest artifact is not unique and unexpired") end
' "$artifacts")

evidence_id=$(jq -r '.id' <<<"$evidence")
evidence_digest=$(jq -r '.digest' <<<"$evidence")
evidence_url=$(jq -r '.archive_download_url' <<<"$evidence")
manifest_id=$(jq -r '.id' <<<"$manifest_artifact")
manifest_digest=$(jq -r '.digest' <<<"$manifest_artifact")
manifest_url=$(jq -r '.archive_download_url' <<<"$manifest_artifact")

evidence_zip="$output/evidence.zip"
manifest_zip="$output/manifest.zip"
gh api "$evidence_url" > "$evidence_zip"
gh api "$manifest_url" > "$manifest_zip"
test "sha256:$(sha256sum "$evidence_zip" | awk '{print $1}')" = "$evidence_digest"
test "sha256:$(sha256sum "$manifest_zip" | awk '{print $1}')" = "$manifest_digest"
unzip -q "$evidence_zip" -d "$output/proof"
unzip -q "$manifest_zip" -d "$output/manifest"

ci_evidence="$output/proof/ci-evidence.json"
conformance="$output/proof/conformance/cases/conformance.json"
proof_manifest="$output/manifest/proof-manifest.json"
test -f "$ci_evidence" -a -f "$conformance" -a -f "$proof_manifest"

toolchain_digest=$(jq -r '.toolchain_digest' "$conformance")
contract_digest=$(jq -r '.contract_digest' "$conformance")
source_digest=$(jq -r '.source_digest' "$conformance")
go_version=$(jq -r '.go_version' "$ci_evidence")
runner_digest=$(jq -r '.runner_digest' "$ci_evidence")
ci_evidence_digest="sha256:$(sha256sum "$ci_evidence" | awk '{print $1}')"
run_attempt=$(jq -r '.run_attempt' "$source_run")
fixed_vector=$(jq -c '.fixed_vector' "$ci_evidence")
generated=$(jq -c '.generated' "$ci_evidence")

jq -e \
  --arg repository "$repository" --arg workflow "$workflow_name" --arg workflow_file "$workflow_file" \
  --arg event "push" --arg ref "refs/heads/main" --arg sha "$expected_sha" \
  --argjson run_id "$source_run_id" --argjson run_attempt "$run_attempt" \
  --argjson evidence_id "$evidence_id" --arg evidence_name "$evidence_name" --arg evidence_digest "$evidence_digest" \
  --arg toolchain_digest "$toolchain_digest" --arg go_version "$go_version" --arg runner_digest "$runner_digest" \
  --arg contract_digest "$contract_digest" --arg source_digest "$source_digest" --arg ci_evidence_digest "$ci_evidence_digest" \
  --argjson fixed_vector "$fixed_vector" --argjson generated "$generated" \
  '.schema == "gooo-proof-preserving-ir-optimizer/ci-proof-manifest/v1" and
   .repository == $repository and .workflow == $workflow and .workflow_file == $workflow_file and
   .event == $event and .ref == $ref and .head_sha == $sha and .run_id == $run_id and
   .run_attempt == $run_attempt and .conclusion == "success" and
   .artifact.id == $evidence_id and .artifact.name == $evidence_name and .artifact.digest == $evidence_digest and
   .manifest.path == "proof-manifest.json" and .manifest.schema == .schema and
   .toolchain.go_version == $go_version and .toolchain.digest == $toolchain_digest and .toolchain.runner_digest == $runner_digest and
   .contract.path == ".gooo/proof-preserving-ir-optimizer.gooo" and .contract.digest == $contract_digest and
   .source.path == ".gooo/proof-preserving-ir-optimizer.gooo" and .source.digest == $source_digest and
   .ci_evidence_digest == $ci_evidence_digest and .fixed_vector == $fixed_vector and .generated == $generated
  ' "$proof_manifest" >/dev/null

jq -e '
  .schema == "gooo-proof-preserving-ir-optimizer/ci-evidence/v1" and
  .fixed_vector == {total_cases:8,closed:5,unknown:1,refuted:2} and
  .generated == {files:16,bytes:19032,binary_bytes:62154194} and
  .tests.total == 2 and .tests.selected == 2 and .tests.executed == 2 and .tests.reused == 0 and .tests.failed == 0 and .tests.unknown == 0 and
  ([.exact_pair_vectors[] | .improvement.exact_pair] | all)
' "$ci_evidence" >/dev/null

jq -n \
  --arg repository "$repository" --argjson source_run_id "$source_run_id" --argjson source_run_attempt "$run_attempt" \
  --argjson evidence_artifact_id "$evidence_id" --arg evidence_artifact_name "$evidence_name" --arg evidence_artifact_digest "$evidence_digest" \
  --argjson manifest_artifact_id "$manifest_id" --arg manifest_artifact_name "$manifest_name" --arg manifest_artifact_digest "$manifest_digest" \
  --arg source_workflow "$workflow_name" --arg source_event "push" --arg source_sha "$expected_sha" \
  --arg toolchain_digest "$toolchain_digest" --arg contract_digest "$contract_digest" --arg source_digest "$source_digest" \
  --arg ci_evidence_digest "$ci_evidence_digest" --argjson fixed_vector "$fixed_vector" --argjson generated "$generated" \
  '{schema:"gooo-proof-preserving-ir-optimizer/reused-proof/v1",source:{repository:$repository,workflow:$source_workflow,event:$source_event,head_sha:$source_sha,run_id:$source_run_id,run_attempt:$source_run_attempt,conclusion:"success"},evidence_artifact:{id:$evidence_artifact_id,name:$evidence_artifact_name,digest:$evidence_artifact_digest},manifest_artifact:{id:$manifest_artifact_id,name:$manifest_artifact_name,digest:$manifest_artifact_digest},manifest:"proof-manifest.json",toolchain_digest:$toolchain_digest,contract_digest:$contract_digest,source_digest:$source_digest,ci_evidence_digest:$ci_evidence_digest,fixed_vector:$fixed_vector,generated:$generated}' > "$output/reused-proof.json"

printf 'reused_proof=CLOSED source_run_id=%s evidence_artifact_id=%s manifest_artifact_id=%s\n' "$source_run_id" "$evidence_id" "$manifest_id"
