#!/usr/bin/env bash
set -e

categories=(
  forvar
  slicescontains
  minmax
  sortslice
  efaceany
  mapsloop
  fmtappendf
  bloop
  rangeint
  stringscutprefix
  waitgroup
)

for category in "${categories[@]}"; do
    echo "Applying modernize fixes for category: $category"
    go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest -category="$category" -fix ./...

    # Check if there are changes to commit
    if ! git diff --quiet -- . ':!modernize.sh'; then
        git add . ':!modernize.sh'
        git commit -m "modernize: apply $category fixes"
    else
        echo "No changes for category $category"
    fi
done
