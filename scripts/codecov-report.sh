#!/usr/bin/env bash
# Download and analyze Codecov coverage report for kaikenlabs/tag.
#
# Usage:
#   ./scripts/codecov-report.sh              # full summary
#   ./scripts/codecov-report.sh --csv        # export per-file CSV
#   ./scripts/codecov-report.sh --file internal/commands/scaffold.go
#   ./scripts/codecov-report.sh --low 70     # show files below 70%
#
# Requires: CODECOV_API_TOKEN (personal access token from app.codecov.io)
set -euo pipefail

TOKEN="${CODECOV_API_TOKEN:?Set CODECOV_API_TOKEN (personal access token from app.codecov.io settings)}"
BASE="https://api.codecov.io/api/v2/github/kaikenlabs/repos/tag"
AUTH="Authorization: bearer ${TOKEN}"
BRANCH="${CODECOV_BRANCH:-main}"

usage() {
    cat <<'EOF'
Usage: codecov-report.sh [OPTIONS]

Options:
  --csv              Export per-file summary to coverage_summary.csv
  --file PATH        Show line-level detail for a specific file
  --low THRESHOLD    List files with coverage below THRESHOLD% (default: 70)
  --tree             Show directory tree with coverage rollups
  --json             Save full report JSON to coverage_report.json
  --branch NAME      Use a specific branch (default: main)
  -h, --help         Show this help
EOF
}

fetch() {
    local url="$1"
    local result
    result=$(curl -sf -H "$AUTH" "$url" 2>&1) || {
        echo "Error: API request failed. Check your CODECOV_API_TOKEN." >&2
        exit 1
    }
    echo "$result"
}

cmd_summary() {
    local report
    report=$(fetch "${BASE}/report/?branch=${BRANCH}")

    local total
    total=$(echo "$report" | jq -r '.totals | "Coverage: \(.coverage)%  Lines: \(.lines)  Hits: \(.hits)  Misses: \(.misses)  Partials: \(.partials)"')
    echo "$total"
    echo

    echo "$report" | jq -r '
        .files
        | sort_by(.totals.coverage)
        | .[]
        | "\(.totals.coverage | tostring | (. + "      ")[:7])% \(.totals.misses | tostring | ("     " + .)[-5:]) miss  \(.name)"
    ' | column -t
}

cmd_csv() {
    local report
    report=$(fetch "${BASE}/report/?branch=${BRANCH}")

    echo "$report" | jq -r '
        ["file","lines","hits","misses","partials","coverage"],
        (.files | sort_by(.totals.coverage) | .[] |
            [.name, .totals.lines, .totals.hits, .totals.misses, .totals.partials, .totals.coverage])
        | @csv
    ' > coverage_summary.csv

    echo "Saved: coverage_summary.csv ($(echo "$report" | jq '.files | length') files)"
}

cmd_file() {
    local filepath="$1"
    local encoded
    encoded=$(echo "$filepath" | sed 's|/|%2F|g')

    local report
    report=$(fetch "${BASE}/file_report/${encoded}/?branch=${BRANCH}")

    local totals
    totals=$(echo "$report" | jq -r '.totals | "Coverage: \(.coverage)%  Lines: \(.lines)  Hits: \(.hits)  Misses: \(.misses)"')
    echo "$filepath"
    echo "$totals"
    echo

    # Show missed and partial lines
    echo "$report" | jq -r '
        .line_coverage
        | to_entries[]
        | select(.value != null and .value != 0)
        | "  L\(.key + 1): \(if .value == 1 then "MISS" else "PARTIAL" end)"
    '
}

cmd_low() {
    local threshold="${1:-70}"
    local report
    report=$(fetch "${BASE}/report/?branch=${BRANCH}")

    echo "Files below ${threshold}% coverage:"
    echo

    echo "$report" | jq -r --argjson t "$threshold" '
        .files
        | map(select(.totals.coverage < $t))
        | sort_by(.totals.coverage)
        | .[]
        | "\(.totals.coverage | tostring | (. + "      ")[:7])%  \(.totals.misses) miss  \(.name)"
    ' | column -t

    local count
    count=$(echo "$report" | jq --argjson t "$threshold" '[.files[] | select(.totals.coverage < $t)] | length')
    echo
    echo "${count} file(s) below ${threshold}%"
}

cmd_tree() {
    local tree
    tree=$(fetch "${BASE}/report/tree?branch=${BRANCH}&depth=10")

    echo "$tree" | jq -r '
        .results
        | sort_by(.coverage)
        | .[]
        | "\(.coverage | tostring | (. + "      ")[:7])%  \(.full_path)"
    ' | column -t
}

cmd_json() {
    fetch "${BASE}/report/?branch=${BRANCH}" > coverage_report.json
    echo "Saved: coverage_report.json"
}

# Parse arguments
ACTION="summary"
FILE_PATH=""
LOW_THRESHOLD="70"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --csv)       ACTION="csv"; shift ;;
        --file)      ACTION="file"; FILE_PATH="$2"; shift 2 ;;
        --low)       ACTION="low"; LOW_THRESHOLD="${2:-70}"; shift 2 ;;
        --tree)      ACTION="tree"; shift ;;
        --json)      ACTION="json"; shift ;;
        --branch)    BRANCH="$2"; shift 2 ;;
        -h|--help)   usage; exit 0 ;;
        *)           echo "Unknown option: $1" >&2; usage; exit 1 ;;
    esac
done

case "$ACTION" in
    summary) cmd_summary ;;
    csv)     cmd_csv ;;
    file)    cmd_file "$FILE_PATH" ;;
    low)     cmd_low "$LOW_THRESHOLD" ;;
    tree)    cmd_tree ;;
    json)    cmd_json ;;
esac
