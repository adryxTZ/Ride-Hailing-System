#!/usr/bin/env bash
# Copies the Songa-era Claude Code session (the scan + redesign chat) into this
# project's Claude Code store so it can be resumed from this folder.
# Run AFTER closing the original chat, so the transcript copy is complete.
set -euo pipefail
ID="${1:-3f35b222-4756-4d2d-a751-434fd3655406}"
SRC="$HOME/.claude/projects/-Users-kalen-PycharmProject-songa"
DST="$HOME/.claude/projects/-Users-kalen-Desktop-ADRIAN-RIDER---HAULER"
mkdir -p "$DST"
cp "$SRC/$ID.jsonl" "$DST/"
[ -d "$SRC/$ID" ] && cp -R "$SRC/$ID" "$DST/"
echo "Copied session $ID into $DST"
echo "Now run:  cd \"/Users/kalen/Desktop/ADRIAN/RIDER & HAULER\" && claude --resume $ID"
echo "Or keep the original untouched and work on a copy:  claude --resume $ID --fork-session"
