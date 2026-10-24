"use client";
/** CompareView — one version's changes against the version before it (line diff), or its Markdown text. */
import { useMemo, useState } from "react";
import { Tabs } from "@/components/ui/Tabs";
import { diffCounts, foldUnchanged, lineDiff } from "./diff";
import type { LegalDoc } from "./hooks";
import { LANG_LABEL } from "./labels";

type Mode = "changes" | "text";

const LINE_STYLE = {
  same: "text-muted",
  added: "bg-[color-mix(in_srgb,var(--good)_12%,transparent)] text-ink",
  removed: "bg-[color-mix(in_srgb,var(--critical)_10%,transparent)] text-critical line-through",
} as const;
const MARK = { same: " ", added: "+", removed: "−" } as const;

export function CompareView({
  version,
  previous,
  lang,
}: {
  version: LegalDoc;
  previous?: LegalDoc;
  lang: string;
}) {
  const [mode, setMode] = useState<Mode>(previous ? "changes" : "text");
  const lines = useMemo(
    () => lineDiff((previous?.body[lang] ?? "").trimEnd(), (version.body[lang] ?? "").trimEnd()),
    [previous, version, lang],
  );
  const counts = diffCounts(lines);
  return (
    <div className="mt-4 grid gap-3 border-t border-line pt-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-xs text-muted">
          {previous
            ? `v${version.version} against v${previous.version}, ${LANG_LABEL[lang] ?? lang}: ${counts.added} lines added, ${counts.removed} removed`
            : `v${version.version} is the first version (${LANG_LABEL[lang] ?? lang})`}
        </p>
        <Tabs
          id="legal-compare"
          value={mode}
          onChange={setMode}
          options={[
            ...(previous ? [{ value: "changes" as const, label: "Changes" }] : []),
            { value: "text" as const, label: "Text" },
          ]}
        />
      </div>
      <pre className="max-h-[520px] overflow-auto rounded-[8px] bg-surface-2 p-3 font-mono text-[12px] leading-relaxed whitespace-pre-wrap">
        {mode === "text"
          ? (version.body[lang] ?? "")
          : foldUnchanged(lines).map((l, i) =>
              // Diff lines have no identity of their own; the list is rebuilt whole.
              l.kind === "fold" ? (
                <div key={i} className="my-1 text-[11px] text-muted italic">
                  {`… ${l.count} unchanged lines`}
                </div>
              ) : (
                <div key={i} className={LINE_STYLE[l.kind]}>
                  {`${MARK[l.kind]} ${l.text}`}
                </div>
              ),
            )}
        {mode === "changes" && counts.added + counts.removed === 0 ? "No changes in this language." : null}
      </pre>
    </div>
  );
}
