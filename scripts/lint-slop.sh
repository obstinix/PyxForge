#!/usr/bin/env bash
# scripts/lint-slop.sh — P0 anti-slop checks. Exit non-zero on any hit.
set -euo pipefail
FAIL=0
SRC="desktop/src extension/src"

check() {
  local pattern="$1" label="$2" exclude="${3:-}"
  local hits
  if [ -n "$exclude" ]; then
    hits=$(grep -rnE "$pattern" $SRC --include="*.ts" --include="*.css" --include="*.html" 2>/dev/null | grep -v "$exclude" || true)
  else
    hits=$(grep -rnE "$pattern" $SRC --include="*.ts" --include="*.css" --include="*.html" 2>/dev/null || true)
  fi
  if [ -n "$hits" ]; then
    echo "❌ $label"
    echo "$hits"
    FAIL=1
  fi
}

check '#(6366f1|4f46e5|4338ca|3730a3|8b5cf6|7c3aed|a855f7|3b82f6|2563eb|1d4ed8|1e1b4b|cba6f7)' \
  "Off-brand accent color found" "tokens.css"
check 'linear-gradient|radial-gradient' "Gradient found"
check 'backdrop-filter' "backdrop-filter (glassmorphism) found"
check 'fonts\.googleapis\.com' "Runtime Google Fonts import found"
check "font-family:\s*['\"]?(Outfit|Inter|Roboto)" "Off-brand display font found" "tokens.css"
check 'Math\.random\(\)' "Math.random() found — verify it isn't faking real data"

if [ "$FAIL" -eq 1 ]; then
  echo "lint-slop: FAILED"
  exit 1
fi
echo "lint-slop: passed"
