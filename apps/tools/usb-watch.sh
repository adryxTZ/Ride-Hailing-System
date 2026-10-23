#!/usr/bin/env bash
# Keeps USB port forwards alive for every attached phone. The forwards vanish when
# the ADB daemon restarts or a cable is replugged, and the apps then show a blank
# screen (they cannot reach Metro or the API on 127.0.0.1). Leave this running:
#
#   tools/usb-watch.sh            # checks every 5 s, re-applies forwards when missing
set -uo pipefail
export PATH="$HOME/Library/Android/sdk/platform-tools:$PATH"
PORTS=(8080 8081 8082)
INTERVAL_S=5
while true; do
  for s in $(adb devices | awk 'NR>1 && $2=="device"{print $1}'); do
    have=$(adb -s "$s" reverse --list 2>/dev/null)
    for p in "${PORTS[@]}"; do
      if ! grep -q "tcp:$p tcp:$p" <<<"$have"; then
        adb -s "$s" reverse "tcp:$p" "tcp:$p" >/dev/null 2>&1 && echo "$(date +%T) $s: forwarded $p"
      fi
    done
  done
  sleep "$INTERVAL_S"
done
