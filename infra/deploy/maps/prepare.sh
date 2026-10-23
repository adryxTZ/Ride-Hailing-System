#!/bin/bash
# Downloads OpenStreetMap extracts for the countries served, merges them, and builds
# the OSRM routing graph (MLD) that the "maps" profile serves. Nominatim imports the
# same merged file on its first start.
#
#   ./maps/prepare.sh                      Tanzania + Kenya (default)
#   COUNTRIES="tanzania kenya uganda" ./maps/prepare.sh
#   PROFILE=car ./maps/prepare.sh          OSRM profile: car (default) | bicycle | foot
#
# Output (MAPS_DATA_DIR, default /var/lib/platform/maps):
#   region.osm.pbf           merged extract (Nominatim input)
#   osrm/region.osrm*        routing graph (OSRM input)
# Re-run monthly (cron) and restart: docker compose ... --profile maps restart osrm.
# Needs ~4 GB RAM and ~5 GB disk for TZ+KE. Only Docker is required on the host.
set -euo pipefail

GEOFABRIK="https://download.geofabrik.de/africa"
COUNTRIES="${COUNTRIES:-tanzania kenya}"
PROFILE="${PROFILE:-car}"
DATA="${MAPS_DATA_DIR:-/var/lib/platform/maps}"
OSRM_IMAGE="ghcr.io/project-osrm/osrm-backend:v26.9.0-debian"
OSMIUM_IMAGE="iboates/osmium:1.16.0"

mkdir -p "$DATA/extracts" "$DATA/osrm"
log() { echo "$(date -u +%H:%M:%S) maps: $*"; }

# 1) Download each country extract, verifying Geofabrik's published MD5.
inputs=()
for c in $COUNTRIES; do
  file="$DATA/extracts/$c-latest.osm.pbf"
  log "downloading $c"
  curl -fsSL -o "$file" "$GEOFABRIK/$c-latest.osm.pbf"
  expected="$(curl -fsSL "$GEOFABRIK/$c-latest.osm.pbf.md5" | awk '{print $1}')"
  actual="$( (md5sum "$file" 2>/dev/null || md5 -r "$file") | awk '{print $1}')"
  [ "$expected" = "$actual" ] || { echo "checksum mismatch for $c" >&2; exit 1; }
  inputs+=("/data/extracts/$c-latest.osm.pbf")
done

# 2) Merge into one extract (a single country is copied as is).
log "merging ${#inputs[@]} extract(s)"
docker run --rm -v "$DATA:/data" "$OSMIUM_IMAGE" \
  osmium merge --overwrite -o /data/region.osm.pbf "${inputs[@]}"

# 3) Build the OSRM graph: extract -> partition -> customize (multi-level Dijkstra).
cp "$DATA/region.osm.pbf" "$DATA/osrm/region.osm.pbf"
log "osrm extract ($PROFILE)"
docker run --rm -v "$DATA/osrm:/data" "$OSRM_IMAGE" osrm-extract -p "/opt/$PROFILE.lua" /data/region.osm.pbf
log "osrm partition"
docker run --rm -v "$DATA/osrm:/data" "$OSRM_IMAGE" osrm-partition /data/region.osrm
log "osrm customize"
docker run --rm -v "$DATA/osrm:/data" "$OSRM_IMAGE" osrm-customize /data/region.osrm
rm -f "$DATA/osrm/region.osm.pbf"
log "done: $DATA/region.osm.pbf and $DATA/osrm/region.osrm"
