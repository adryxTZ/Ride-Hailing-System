"use client";
/**
 * DocumentScreen — one app's legal document: the editor (one tab per
 * language) beside a live preview, the placeholders, and the version history.
 * Save draft keeps the work; Publish (with a reason) makes the draft live and,
 * for a text people accept, asks every rider or driver to accept it again.
 */
import { ArrowLeft, ExternalLink } from "lucide-react";
import Link from "next/link";
import { useRef, useState } from "react";
import { Card } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import { Tabs } from "@/components/ui/Tabs";
import { DocumentActions } from "./DocumentActions";
import { type BodyInserter, EditorCard } from "./EditorCard";
import { HistoryCard } from "./HistoryCard";
import { type LegalApp, type LegalKind, useLegal, useVersions } from "./hooks";
import { APP_LABEL, LANG_LABEL } from "./labels";
import { PlaceholdersCard } from "./PlaceholdersCard";
import { PreviewCard } from "./PreviewCard";
import { useEditorState } from "./useEditorState";

export function DocumentScreen({ app, kind }: { app: LegalApp; kind: LegalKind }) {
  const overview = useLegal();
  const versions = useVersions(app, kind);
  const editor = useEditorState(versions.data?.versions);
  const inserter = useRef<BodyInserter>(null);
  const locales = overview.data?.locales ?? ["en"];
  const [lang, setLang] = useState<string>();
  const current = lang ?? overview.data?.default_locale ?? locales[0];
  if (versions.isLoading || overview.isLoading) return <Skeleton className="h-96" />;
  const item = overview.data?.documents.find((d) => d.app === app && d.kind === kind);
  if (versions.error || overview.error || !versions.data || !item) {
    return <ErrorNote error={versions.error ?? overview.error ?? new Error("Document not found")} />;
  }
  const doc = versions.data;
  return (
    <div className="grid gap-4">
      <PageHeader
        title={item.title}
        subtitle={`${APP_LABEL[app]}. ${doc.consent ? "People accept this text: publishing a new version asks them again." : ""}`}
        actions={<HeaderLinks pageUrl={doc.page_url} live={!!editor.live} />}
      />
      <div className="grid gap-4 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        <Card index={1}>
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <Tabs
              id="legal-lang"
              value={current}
              onChange={setLang}
              options={locales.map((l) => ({ value: l, label: LANG_LABEL[l] ?? l }))}
            />
            <DocumentActions app={app} kind={kind} consent={doc.consent} editor={editor} />
          </div>
          <EditorCard
            lang={current}
            texts={editor.texts}
            consent={doc.consent}
            onChange={editor.set}
            inserter={inserter}
          />
        </Card>
        <PreviewCard
          app={app}
          kind={kind}
          lang={current}
          title={editor.texts.title[current] ?? ""}
          body={editor.texts.body[current] ?? ""}
          index={2}
        />
      </div>
      <PlaceholdersCard
        placeholders={overview.data?.placeholders ?? []}
        onInsert={(t) => inserter.current?.insert(t)}
        index={3}
      />
      <HistoryCard versions={doc.versions} lang={current} onCopy={editor.load} index={4} />
    </div>
  );
}

function HeaderLinks({ pageUrl, live }: { pageUrl: string; live: boolean }) {
  return (
    <>
      <Link
        href="/settings/legal"
        className="inline-flex items-center gap-1.5 text-xs font-semibold text-muted hover:text-ink"
      >
        <ArrowLeft size={14} aria-hidden="true" /> Legal
      </Link>
      {live ? (
        <a
          href={pageUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1.5 text-xs font-semibold text-brand-text"
        >
          Public page <ExternalLink size={12} aria-hidden="true" />
        </a>
      ) : null}
    </>
  );
}
