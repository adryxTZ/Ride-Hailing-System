#!/usr/bin/env bash
# Sets up USB port forwards for every attached Android phone and opens the apps
# in Expo Go. Run after plugging phones in, and again if `adb` restarts (the
# forwards are lost when the ADB daemon restarts).
#
#   tools/usb-phones.sh                 # first phone → rider, second → driver
#   RIDER=<serial> DRIVER=<serial> tools/usb-phones.sh
#
# Dev servers must be running:  (cd rider  && BRAND=nexa API_BASE_URL=http://127.0.0.1:8080/v1 npx expo start --lan --port 8081)
#                                (cd driver && BRAND=nexa API_BASE_URL=http://127.0.0.1:8080/v1 npx expo start --lan --port 8082)
set -euo pipefail
export PATH="$HOME/Library/Android/sdk/platform-tools:$PATH"
DEVICES=()
while IFS= read -r line; do DEVICES+=("$line"); done < <(adb devices | awk 'NR>1 && $2=="device"{print $1}')
RIDER="${RIDER:-${DEVICES[0]:-}}"
DRIVER="${DRIVER:-${DEVICES[1]:-}}"
launch() { # serial, metro port
  local s="$1" port="$2"
  adb -s "$s" reverse "tcp:$port" "tcp:$port" >/dev/null
  adb -s "$s" reverse tcp:8080 tcp:8080 >/dev/null
  adb -s "$s" shell am force-stop host.exp.exponent
  adb -s "$s" shell am start -W -a android.intent.action.VIEW -d "exp://127.0.0.1:$port" >/dev/null 2>&1
  echo "$s → exp://127.0.0.1:$port ($(adb -s "$s" shell getprop ro.product.model | tr -d '\r'))"
}
[ -n "$RIDER" ] && launch "$RIDER" 8081 || echo "no rider phone attached"
[ -n "$DRIVER" ] && launch "$DRIVER" 8082 || echo "no driver phone attached"
