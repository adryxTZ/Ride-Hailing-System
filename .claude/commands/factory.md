---
description: Build a feature through the software factory (spec, build, four parallel reviews, up to two fix rounds, approval)
argument-hint: <feature to build>
---
Run the software factory for: $ARGUMENTS

1. **Spec**: launch the `spec-writer` agent with the feature. Save the spec to `DOC/specs/<yyyy-mm-dd>-<slug>.md`. If it lists owner questions that block the build, ask them now; otherwise use the spec's stated defaults and note them.
2. **Build**: implement the spec in this session (you are the builder), following CLAUDE.md. Make the spec's tests pass.
3. **Review**: launch `security-reviewer`, `ux-reviewer`, `ui-reviewer` and `code-reviewer` in ONE message so they run in parallel. Give each the spec path and the list of changed files. Wait for all four.
4. **Fix**: if any verdict is CHANGES, fix the findings you agree with (say why you reject any), then re-run only the reviewers that asked for changes. At most 2 fix rounds; after that, remaining findings go to the approver as open items.
5. **Approve**: launch `approver` with the spec, the final verdicts and changed files.
   - APPROVE: report to the owner with a short summary. Do not commit unless the owner has asked for commits ("merge to main" is the owner's call).
   - ESCALATE: stop and report what needs a human decision.
6. Append the outcome (date, verdicts, rounds, decision) to the spec file.
