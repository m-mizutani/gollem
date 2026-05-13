#!/usr/bin/env bash
# Detect invisible / ambiguous Unicode characters in tracked text files.
#
# Scans files tracked by git for characters that are either invisible
# (zero-width, BiDi controls, BOM, etc.) or easily confused with ASCII
# whitespace (NBSP, ideographic space, soft hyphen). Such characters are
# common vectors for Trojan Source attacks (CVE-2021-42574) and silent
# copy/paste corruption.
#
# Exit codes:
#   0 - no findings
#   1 - findings detected (CI should fail)
#   2 - internal error (missing tool, etc.)

set -euo pipefail

# Disallowed code points:
#   200B-200D  Zero-Width Space / Non-Joiner / Joiner
#   2060-2064  Word Joiner / invisible operators
#   206A-206F  Deprecated formatting controls
#   202A-202E  BiDi formatting (LRE/RLE/PDF/LRO/RLO)
#   2066-2069  BiDi isolate (Trojan Source: CVE-2021-42574)
#   FEFF       BOM / Zero-Width No-Break Space (in-line; start-of-file BOM is checked separately)
#   180E       Mongolian Vowel Separator
#   FFF9-FFFB  Interlinear annotations
#   00A0       No-Break Space (NBSP)
#   3000       Ideographic Space
#   00AD       Soft Hyphen
#   115F, 1160, 3164, FFA0  Hangul fillers
PATTERN='[\x{200B}-\x{200D}\x{2060}-\x{2064}\x{206A}-\x{206F}\x{202A}-\x{202E}\x{2066}-\x{2069}\x{FEFF}\x{180E}\x{FFF9}-\x{FFFB}\x{00A0}\x{3000}\x{00AD}\x{115F}\x{1160}\x{3164}\x{FFA0}]'

# Glob excludes for ripgrep (binary / generated files).
EXCLUDES=(
  -g '!*.png' -g '!*.jpg' -g '!*.jpeg' -g '!*.gif' -g '!*.ico'
  -g '!*.svg' -g '!*.webp' -g '!*.bmp'
  -g '!*.woff' -g '!*.woff2' -g '!*.ttf' -g '!*.eot' -g '!*.otf'
  -g '!*.pdf' -g '!*.zip' -g '!*.tar' -g '!*.gz' -g '!*.tgz'
  -g '!*.bz2' -g '!*.xz'
  -g '!*.mp4' -g '!*.mp3' -g '!*.wav' -g '!*.ogg' -g '!*.webm'
  -g '!*.class' -g '!*.jar' -g '!*.so' -g '!*.dll' -g '!*.dylib' -g '!*.exe'
  -g '!cmd/gollem/frontend/dist/**'
)

# Pre-flight checks.
if ! command -v rg >/dev/null 2>&1; then
  echo "error: ripgrep (rg) is required but not installed" >&2
  exit 2
fi
if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "error: must be run inside a git working tree" >&2
  exit 2
fi

cd "$(git rev-parse --show-toplevel)"

findings_file=$(mktemp)
trap 'rm -f "$findings_file"' EXIT

# --- Check 1: in-line invisible / ambiguous Unicode characters ---
# ripgrep walks files while respecting .gitignore; binary files are auto-skipped.
# --column reports the column of the first match on each line; that location is
# sufficient for developers to navigate and inspect adjacent chars.
set +e
rg_output=$(rg --pcre2 --column --line-number --no-heading --color=never \
  "${EXCLUDES[@]}" -e "$PATTERN" .)
rg_status=$?
set -e

case "$rg_status" in
  0)
    while IFS= read -r line; do
      file=${line%%:*}
      file=${file#./}
      rest=${line#*:}
      lnum=${rest%%:*}
      rest=${rest#*:}
      col=${rest%%:*}
      printf '%s\t%s\t%s\n' "$file" "$lnum" "$col" >> "$findings_file"
    done <<< "$rg_output"
    ;;
  1) ;;  # no matches
  *)
    echo "error: ripgrep failed with exit code $rg_status" >&2
    exit 2
    ;;
esac

# --- Check 2: UTF-8 BOM (U+FEFF) at the start of a file ---
# ripgrep transparently strips a leading BOM before searching, so a BOM at
# byte offset 0 is invisible to Check 1. Catch it here by inspecting the
# first three bytes of each tracked text file.
BOM=$'\xef\xbb\xbf'
while IFS= read -r -d '' path; do
  case "$path" in
    *.png|*.jpg|*.jpeg|*.gif|*.ico|*.svg|*.webp|*.bmp) continue ;;
    *.woff|*.woff2|*.ttf|*.eot|*.otf) continue ;;
    *.pdf|*.zip|*.tar|*.gz|*.tgz|*.bz2|*.xz) continue ;;
    *.mp4|*.mp3|*.wav|*.ogg|*.webm) continue ;;
    *.class|*.jar|*.so|*.dll|*.dylib|*.exe) continue ;;
    cmd/gollem/frontend/dist/*) continue ;;
  esac
  [[ -f "$path" ]] || continue
  first3=$(head -c 3 -- "$path" 2>/dev/null || true)
  if [[ "$first3" == "$BOM" ]]; then
    printf '%s\t1\t1\n' "$path" >> "$findings_file"
  fi
done < <(git ls-files -z)

# --- Emit findings ---
if [[ ! -s "$findings_file" ]]; then
  echo "OK: no invisible/ambiguous Unicode characters detected"
  exit 0
fi

count=0
while IFS=$'\t' read -r file lnum col; do
  msg="invisible or ambiguous Unicode character detected; remove or replace with ASCII equivalent"
  # GitHub Actions annotation for inline PR display.
  echo "::error file=${file},line=${lnum},col=${col}::${msg}"
  # Mirror to stderr for human-readable log.
  echo "${file}:${lnum}:${col}: ${msg}" >&2
  count=$((count + 1))
done < "$findings_file"

echo "" >&2
echo "total findings: $count" >&2
exit 1
