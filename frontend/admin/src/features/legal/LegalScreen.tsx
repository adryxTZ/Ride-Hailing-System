"use client";
/**
 * LegalScreen — Settings › Legal: each app's Privacy Notice, Terms of Use and
 * Driver Agreement with its live version and any draft (open one to edit), and
 * the Legal details the texts' placeholders read. New brands start with a
 * starter pack of templates that a lawyer should review.
 */
import { AlertTriangle } from "lucide-react";
import { useRouter } from "next/navigation";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { PageHeader } from "@/components/ui/PageHeader";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { when } from "@/lib/format";
import { DetailsForm } from "./DetailsForm";
import { type LegalItem, useLegal } from "./hooks";
import { APP_LABEL, docHref } from "./labels";

export function LegalScreen() {
  const { data, isLoading, error } = useLegal();
  if (isLoading) return <Skeleton className="h-96" />;
  if (error || !data) return <ErrorNote error={error ?? new Error("Could not load the legal texts")} />;
  const missing = data.placeholders.filter((p) => p.source === "DETAILS" && !p.value);
  const starter = data.documents.some((d) => d.live?.source === "STARTER");
  return (
    <div className="grid gap-4">
      <PageHeader
        title="Legal"
        subtitle="The privacy notices, terms and driver agreement the apps show. Publishing a new version of a text people accept asks them to accept it again."
      />
      {starter || missing.length ? (
        <Attention starter={starter} missing={missing.map((m) => m.label)} />
      ) : null}
      <Card title="Documents" index={1}>
        <DocumentsTable rows={data.documents} />
      </Card>
      <Card title="Legal details" index={2}>
        <DetailsForm details={data.details} placeholders={data.placeholders} />
      </Card>
    </div>
  );
}

/** What still needs doing before the texts can be relied on. */
function Attention({ starter, missing }: { starter: boolean; missing: string[] }) {
  return (
    <div
      role="status"
      className="flex items-start gap-3 rounded-[10px] bg-[color-mix(in_srgb,var(--warn)_12%,transparent)] px-4 py-3 text-sm"
    >
      <AlertTriangle size={16} className="mt-0.5 shrink-0 text-warn" aria-hidden="true" />
      <div className="grid gap-1">
        {starter ? (
          <p>
            Some live texts are still the <strong>starter templates</strong>. Have a lawyer review and adapt
            them for your company and country, then publish the reviewed version.
          </p>
        ) : null}
        {missing.length ? (
          <p className="text-muted">
            Not filled in yet, so the texts show a gap: {missing.join(", ")}. Fill them in under Legal details
            below.
          </p>
        ) : null}
      </div>
    </div>
  );
}

function DocumentsTable({ rows }: { rows: LegalItem[] }) {
  const router = useRouter();
  return (
    <Table head={["App", "Document", "Live version", "Draft", "People accept it"]}>
      {rows.map((d) => (
        <Row key={`${d.app}/${d.kind}`} onClick={() => router.push(docHref(d.app, d.slug))}>
          <Cell className="text-xs text-muted">{APP_LABEL[d.app]}</Cell>
          <Cell className="font-semibold">{d.title}</Cell>
          <Cell>
            {d.live ? (
              <span className="flex items-center gap-2">
                <span className="tnum">v{d.live.version}</span>
                {d.live.source === "STARTER" ? <Chip tone="warn">Starter template</Chip> : null}
                <span className="tnum text-[11px] text-muted">{when(d.live.published_at)}</span>
              </span>
            ) : (
              <Chip tone="bad">Not published</Chip>
            )}
          </Cell>
          <Cell>
            {d.draft ? (
              <Chip tone="live">{`Draft v${d.draft.version} · ${when(d.draft.updated_at)}`}</Chip>
            ) : (
              <span className="text-muted">—</span>
            )}
          </Cell>
          <Cell className="text-xs text-muted">{d.consent ? "Yes, asked again on publish" : "No"}</Cell>
        </Row>
      ))}
    </Table>
  );
}
