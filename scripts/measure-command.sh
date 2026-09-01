#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -lt 5 ]]; then
  echo "usage: measure-command.sh OUTPUT_JSON LABEL LOG COMMAND..." >&2
  exit 64
fi

output=$1
label=$2
log=$3
shift 3
start=$(date +%s%3N)
rss_file=$(mktemp)
trap 'rm -f "$rss_file"' EXIT
set +e
/usr/bin/time -f '%M' -o "$rss_file" "$@" >"$log" 2>&1
status=$?
set -e
end=$(date +%s%3N)
wall_ms=$((end - start))
peak_rss_kib=$(awk 'NR == 1 {print $1 + 0}' "$rss_file")
if [[ -z "$peak_rss_kib" ]]; then
  peak_rss_kib=0
fi
jq -n --arg label "$label" --argjson status "$status" --argjson wall_ms "$wall_ms" --argjson peak_rss_kib "$peak_rss_kib" \
  '{label:$label,status:$status,wall_ms:$wall_ms,peak_rss_kib:$peak_rss_kib}' > "$output"
cat "$log"
exit "$status"
