#!/usr/bin/env bash
# Tiny UI driver for the test phones: tap an element by its visible text (regex), or wait for text.
#   tools/adb-ui.sh <serial> tap  '<regex>'      tools/adb-ui.sh <serial> wait '<regex>' [seconds]
#   tools/adb-ui.sh <serial> text                (prints visible texts)
set -uo pipefail
export PATH="$HOME/Library/Android/sdk/platform-tools:$PATH"
S="$1"; CMD="$2"; RE="${3:-}"; T="${4:-20}"
dump() { adb -s "$S" shell uiautomator dump /sdcard/ui.xml >/dev/null 2>&1; adb -s "$S" shell cat /sdcard/ui.xml; }
find_xy() { dump | python3 -c "
import sys,re
x=sys.stdin.read(); pat=re.compile(sys.argv[1])
for m in re.finditer(r'<node [^>]*?text=\"([^\"]*)\"[^>]*?bounds=\"\[(\d+),(\d+)\]\[(\d+),(\d+)\]\"', x):
    if pat.fullmatch(m.group(1)):
        a,b,c,d=map(int,m.groups()[1:]); print((a+c)//2,(b+d)//2); break
" "$1"; }
case "$CMD" in
  text) dump | grep -o 'text="[^"]\+"' | sed 's/text=//' ;;
  tap) xy=$(find_xy "$RE"); [ -n "$xy" ] && adb -s "$S" shell input tap $xy && echo "tapped '$RE' at $xy" || { echo "NOT FOUND: $RE"; exit 1; } ;;
  wait) for _ in $(seq 1 "$T"); do [ -n "$(find_xy "$RE")" ] && { echo "seen '$RE'"; exit 0; }; sleep 1; done; echo "TIMEOUT waiting for $RE"; exit 1 ;;
esac
