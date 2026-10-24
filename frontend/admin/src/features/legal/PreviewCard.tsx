"use client";
/** PreviewCard — the text as riders or drivers will read it, placeholders filled with today's values. */
import { Card } from "@/components/ui/Card";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import { type LegalApp, type LegalKind, usePreview } from "./hooks";
import { Prose } from "./Prose";

export function PreviewCard({
  app,
  kind,
  lang,
  title,
  body,
  index,
}: {
  app: LegalApp;
  kind: LegalKind;
  lang: string;
  title: string;
  body: string;
  index: number;
}) {
  const { data, isLoading, error } = usePreview(app, kind, lang, title, body);
  return (
    <Card title="Preview" index={index} className="xl:sticky xl:top-4">
      {error ? <ErrorNote error={error} /> : null}
      {data?.unknown.length ? (
        <p className="mb-3 text-xs font-semibold text-critical">
          {`Unknown placeholders (fix before saving): ${data.unknown.map((k) => `{{${k}}}`).join(", ")}`}
        </p>
      ) : null}
      {data?.missing.length ? (
        <p className="mb-3 text-xs text-warn">
          {`No value yet, shown as a gap: ${data.missing.join(", ")}. Fill them in under Legal details.`}
        </p>
      ) : null}
      {isLoading && !data ? <Skeleton className="h-96" /> : null}
      {data && !data.html ? (
        <p className="text-sm text-muted">Nothing written in this language yet.</p>
      ) : null}
      {data?.html ? (
        <div className="max-h-[75vh] overflow-auto pr-1">
          <h1 className="mb-3 text-xl font-semibold">{title}</h1>
          <Prose html={data.html} />
        </div>
      ) : null}
    </Card>
  );
}
