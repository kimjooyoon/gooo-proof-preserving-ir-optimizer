#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 2 ]]; then
  echo "usage: integration.sh CONFORMANCE_CASES OUTPUT_ROOT" >&2
  exit 64
fi

cases=$1
output_root=$2
mkdir -p "$output_root"
count=0
while IFS= read -r -d '' source; do
  relative=${source#"$cases/"}
  case_id=${relative%%/*}
  variant=${relative#*/}
  variant=${variant%%/*}
  target="$output_root/$case_id/$variant"
  mkdir -p "$target"
  cp "$source" "$target/main.go"
  printf 'module generated.integration.%s.%s\n\ngo 1.27.0\n' "${case_id//-/.}" "$variant" > "$target/go.mod"
  (cd "$target" && GOCACHE="$PWD/.cache" GOTOOLCHAIN=local go build -trimpath -o program .)
  output=$("$target/program")
  jq -e '.value|type == "number"' <<<"$output" >/dev/null
  count=$((count + 1))
done < <(find "$cases" -type f \( -path '*/before/generated/main.go' -o -path '*/after/generated/main.go' \) -print0 | sort -z)
test "$count" -eq 16
jq -n --argjson generated_cases "$count" '{schema:"gooo-proof-preserving-ir-optimizer/integration/v1",generated_cases:$generated_cases,decision:"CLOSED"}' > "$output_root/integration.json"
