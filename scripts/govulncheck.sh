#!/usr/bin/env bash
# Wrapper for govulncheck that allows excluding specific vulnerabilities.
# Each excluded vulnerability MUST link to a tracking issue with rationale.
# See https://github.com/golang/go/issues/59507 for upstream support tracking.
set -Eeuo pipefail

# Vulnerabilities excluded until upstream ships a fix.
# Each entry must link to a tracking issue and explain why suppression is safe.
excludeVulns="$(jq -nc '[
  # docker vulnerabilities (no fix available upstream).
  # Issue: https://github.com/pipecrew/pisyn/issues/26
  # Both affect Docker daemon AuthZ plugin paths, not the client SDK we use.
  "GO-2026-4887", # CVE: Docker AuthZ plugin bypass
  "GO-2026-4883", # CVE: Docker plugin privilege off-by-one

  empty
]')"
export excludeVulns

# Fast path: if govulncheck passes outright, no filtering needed.
if govulncheck ./...; then
  exit 0
fi

# Re-run in JSON mode so we can filter by ID.
json="$(govulncheck -json ./...)"

# Pull every vuln that has at least one called function in the trace.
# (Vulns without a called function are informational and never fail govulncheck.)
vulns="$(jq <<<"$json" -cs '
  (map(.osv // empty | {key: .id, value: .}) | from_entries) as $meta
  | map(.finding // empty | select((.trace[0].function // "") != "") | .osv)
  | unique
  | map($meta[.])
')"

# Drop excluded IDs.
filtered="$(jq <<<"$vulns" -c '
  (env.excludeVulns | fromjson) as $exclude
  | map(select(.id as $id | $exclude | index($id) | not))
')"

text="$(jq <<<"$filtered" -r 'map("- \(.id) (aka \(.aliases | join(", ")))\n\n\t\(.details | gsub("\n"; "\n\t"))") | join("\n\n")')"

if [ -z "$text" ]; then
  echo "govulncheck passed (all findings are in the known-excluded list)"
  exit 0
else
  printf '%s\n' "$text"
  exit 1
fi
