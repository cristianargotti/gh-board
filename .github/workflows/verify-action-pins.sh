#!/usr/bin/env bash
set -euo pipefail

# Resolving commits also peels annotated tags, unlike comparing tag-object SHAs.
sed -nE 's/.*uses: ([^@ ]+)@([0-9a-f]{40}) # (v[0-9.]+)$/\1 \2 \3/p' \
  .github/workflows/*.yml | sort -u |
  while read -r action expected version; do
    repository=$(printf '%s' "$action" | cut -d / -f 1,2)
    actual=$(gh api "repos/$repository/commits/$version" --jq .sha)
    if [ "$actual" != "$expected" ]; then
      printf 'Action pin mismatch: %s %s: expected %s, got %s\n' \
        "$action" "$version" "$expected" "$actual" >&2
      exit 1
    fi
    printf 'Verified %s %s %s\n' "$action" "$version" "$actual"
  done
