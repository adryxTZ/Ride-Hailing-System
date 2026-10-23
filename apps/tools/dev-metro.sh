#!/bin/zsh
# Development bundlers: rider on :8081 and driver on :8082 for phones (LAN; Expo Go on
# iPhone only opens them when the Mac's Expo CLI and Expo Go use the same account:
# run `npx expo login` once),
# rider web on :8083 and driver web on :8084 for browser tests. Clean cache.
# Logs: apps/tools/logs/metro-<app>-<port>.log
cd "${0:A:h}/.."
mkdir -p tools/logs
pkill -f "expo start" 2>/dev/null; sleep 2
start() { (cd "$1" && nohup npx expo start $=2 --clear > "../tools/logs/metro-$1-${2##* }.log" 2>&1 &) }
start rider "--lan --port 8081"
start driver "--lan --port 8082"
start rider "--web --port 8083"
start driver "--web --port 8084"
for i in {1..60}; do
  n=$(lsof -nP -iTCP -sTCP:LISTEN 2>/dev/null | grep -cE ":(808[1-4]) ")
  [ "$n" -ge 4 ] && { echo "bundlers up on 8081-8084"; exit 0; }; sleep 1
done
echo "bundlers did not all start; see apps/tools/logs" >&2; exit 1
