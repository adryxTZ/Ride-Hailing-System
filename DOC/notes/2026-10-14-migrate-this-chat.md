# Migrating the scan + redesign chat into this project (2026-10-14)

Claude Code keeps conversations per project folder under `~/.claude/projects/<encoded path>/`, where every non-alphanumeric character of the path becomes a dash. This project's store is `~/.claude/projects/-Users-kalen-Desktop-ADRIAN-RIDER---HAULER/`. The chat that produced these documents was started in the Songa folder, so it lives under `-Users-kalen-PycharmProject-songa/` as `3f35b222-4756-4d2d-a751-434fd3655406.jsonl` plus a same-named folder of tool results.

## Already done
- Project memory seeded in this project's store (`memory/MEMORY.md` + the redesign, separate-app-developer, no-AI-trailer and Songa-layout notes).
- `CLAUDE.md`, `.claude/settings.json` and the `DOC` set carry the full context, so a fresh session here already knows the project.

## To move the chat itself (optional)
1. Close the original chat (so its transcript is complete).
2. Run `DOC/notes/migrate-chat.sh` (copies the transcript and its companion folder into this project's store; the original stays where it is).
3. From this folder: `claude --resume 3f35b222-4756-4d2d-a751-434fd3655406`, or add `--fork-session` to continue under a new ID and leave the original untouched.
4. In VS Code: open "RIDER & HAULER" as the workspace folder, then pick the conversation from the Claude Code resume list for this project.

## Caveats
- Old tool results inside the transcript refer to Songa paths; new work uses this folder as the working directory.
