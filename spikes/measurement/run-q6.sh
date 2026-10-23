#!/bin/sh
# Re-run the question 6 (search) spike and capture its output.
#
# The corpus is at ../../../fixtures relative to spikes/measurement/search.
# Set CORPUS to point somewhere else.
set -e
cd "$(dirname "$0")/search"
CORPUS="${CORPUS:-../../../fixtures}"
OUT="${OUT:-../results}"
mkdir -p "$OUT"

echo "== index =="
go run . index -root "$CORPUS" -schema headings -db /tmp/opencode/search-head.db | tee "$OUT/q6-index.txt"
go run . index -root "$CORPUS" -schema body -db /tmp/opencode/search-bodyonly.db | tee "$OUT/q6-index-bodyonly.txt"
go run . index -root "$CORPUS" -schema headings -keep-community=false -db /tmp/opencode/search-nocomm.db | tee "$OUT/q6-index-nocommunity.txt"

echo "== coverage =="
go run . coverage -root "$CORPUS" | tee "$OUT/q6-coverage.txt"

echo "== schema comparison =="
go run . schema -root "$CORPUS" | tee "$OUT/q6-schema.txt"

echo "== literal index =="
go run . literal -root "$CORPUS" | tee "$OUT/q6-literal.txt"

echo "== merged =="
go run . merged -root "$CORPUS" -db /tmp/opencode/search-head.db | tee "$OUT/q6-merged.txt"

echo "== per-query detail =="
go run . query -root "$CORPUS" -db /tmp/opencode/search-head.db -top 8 | tee "$OUT/q6-queries.txt"

echo "== rank inflation =="
go run . inflate -root "$CORPUS" | tee "$OUT/q6-inflate.txt"

echo "== the synthetic campaign =="
go run . index -root ../testdata/campaign -schema headings -db /tmp/opencode/campaign.db | tee "$OUT/q6-campaign-index.txt"
go run . query -root ../testdata/campaign -db /tmp/opencode/campaign.db -top 5 \
  -queries "the duke's letter,duke,goblin,toll road,letter,thistle" | tee "$OUT/q6-campaign-queries.txt"

echo "== operators =="
go run . ops | tee "$OUT/q6-ops.txt"

echo "== BM25 score magnitude against corpus size =="
go run . scale | tee "$OUT/q6-scale.txt"

echo "== OR against AND =="
go run . andor -root "$CORPUS" -db /tmp/opencode/search-head.db | tee "$OUT/q6-andor.txt"
go run . coordination -root "$CORPUS" -db /tmp/opencode/search-head.db | tee "$OUT/q6-coordination.txt"
