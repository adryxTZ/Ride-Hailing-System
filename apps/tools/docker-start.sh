#!/bin/bash
# Starts the four Expo bundlers inside the apps container (see apps/Dockerfile).
# HOST_IP is the address phones use to reach this machine; API_BASE_URL the API they call.
set -e
cd "$(dirname "$0")/.."
export REACT_NATIVE_PACKAGER_HOSTNAME="${HOST_IP:-localhost}"
start() { (cd "$1" && npx expo start $2 > "/tmp/metro-$1-${2##* }.log" 2>&1 &) }
start rider  "--lan --port 8081"
start driver "--lan --port 8082"
start rider  "--web --port 8083"
start driver "--web --port 8084"
echo "rider  phones: exp://${REACT_NATIVE_PACKAGER_HOSTNAME}:8081   web: http://localhost:8083"
echo "driver phones: exp://${REACT_NATIVE_PACKAGER_HOSTNAME}:8082   web: http://localhost:8084"
# Keep the container alive and stream the logs.
tail -f /tmp/metro-*.log
