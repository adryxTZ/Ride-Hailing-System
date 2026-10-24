"use client";
/**
 * RequestScreen — one data-protection request: who asked, what for, when it
 * is due, what blocks a deletion right now, and the decision (approve or
 * reject with a reason). A handled request never changes.
 */
import { ArrowLeft, ArrowUpRight } from "lucide-react";
import Link from "next/link";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { PageHeader } from "@/components/ui/PageHeader";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import { dateTime } from "@/lib/format";
import { DecisionForm } from "./DecisionForm";
import { DueChip } from "./DueChip";
import { type PrivacyRequest, useRequest } from "./hooks";
import { APP_LABEL, BLOCKER_FIX, BLOCKER_LABEL, KIND_LABEL, STATUS_LABEL, STATUS_TONE } from "./labels";
import { blockerLink, isAnonymised, personHref, personName } from "./links";

const KIND_HELP: Record<string, string> = {
  EXPORT:
    "They asked for a copy of the data the Platform holds about them. Approving builds a file they download from the app.",
  DELETE: "They asked for their account to be deleted. Approving anonymises it at once; it cannot be undone.",
};

export function RequestScreen({ id }: { id: string }) {
  const { data: r, isLoading, error } = useRequest(id);
  if (isLoading) return <Skeleton className="h-96" />;
  if (error || !r) return <ErrorNote error={error ?? new Error("Request not found")} />;
  return (
    <div>
      <PageHeader
        title={KIND_LABEL[r.kind] ?? r.kind}
        subtitle={KIND_HELP[r.kind]}
        actions={
          <Link
            href="/compliance/privacy"
            className="inline-flex items-center gap-1.5 text-xs font-semibold text-muted hover:text-ink"
          >
            <ArrowLeft size={14} aria-hidden="true" /> Privacy
          </Link>
        }
      />
      <div className="grid gap-4 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        <div className="grid content-start gap-4">
          <Card title="Request" index={1}>
            <About r={r} />
          </Card>
          {r.blockers.length ? (
            <Card title="Blocking the deletion" index={2}>
              <Blockers r={r} />
            </Card>
          ) : null}
        </div>
        <Card title="Decision" index={3}>
          <DecisionForm request={r} />
        </Card>
      </div>
    </div>
  );
}

function About({ r }: { r: PrivacyRequest }) {
  const href = personHref(r);
  const name = personName(r);
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2.5 text-xs">
      <dt className="text-muted">Who</dt>
      <dd>
        {href ? (
          <Link href={href} className="inline-flex items-center gap-1 font-semibold hover:text-brand-text">
            {name} <ArrowUpRight size={12} aria-hidden="true" />
          </Link>
        ) : (
          <span className="font-semibold">{name}</span>
        )}
        <span className="ml-1.5 text-muted">
          {[APP_LABEL[r.app], r.user_name && !isAnonymised(r) ? r.user_phone : undefined]
            .filter(Boolean)
            .join(" · ")}
        </span>
      </dd>
      <dt className="text-muted">Asked for</dt>
      <dd>
        <Chip tone={r.kind === "DELETE" ? "bad" : "muted"}>{KIND_LABEL[r.kind]}</Chip>
      </dd>
      <dt className="text-muted">Asked</dt>
      <dd className="tnum">{dateTime(r.created_at)}</dd>
      <dt className="text-muted">Due</dt>
      <dd>
        <DueChip request={r} />
      </dd>
      <dt className="text-muted">Status</dt>
      <dd>
        <Chip tone={STATUS_TONE[r.status]}>{STATUS_LABEL[r.status]}</Chip>
      </dd>
      {r.kind === "DELETE" && r.status === "REQUESTED" ? (
        <>
          <dt className="text-muted">Blockers</dt>
          <dd className={r.blockers.length ? "font-semibold text-critical" : "text-good"}>
            {r.blockers.length
              ? `${r.blockers.length} to settle first (see below)`
              : "None. It can be deleted now."}
          </dd>
        </>
      ) : null}
      {r.download_expires_at ? (
        <>
          <dt className="text-muted">Download until</dt>
          <dd className="tnum">{dateTime(r.download_expires_at)}</dd>
        </>
      ) : null}
    </dl>
  );
}

function Blockers({ r }: { r: PrivacyRequest }) {
  return (
    <div className="grid gap-3">
      <p className="text-xs leading-relaxed text-muted">
        The account cannot be deleted while money or a trip is still open. Settle these first, then approve.
      </p>
      <ul className="divide-y divide-line">
        {r.blockers.map((b) => {
          const link = blockerLink(r, b);
          return (
            <li key={b} className="flex flex-wrap items-start justify-between gap-2 py-2.5">
              <div className="grid gap-0.5">
                <span className="text-xs font-semibold text-critical">{BLOCKER_LABEL[b] ?? b}</span>
                <span className="text-[11px] text-muted">{BLOCKER_FIX[b]}</span>
              </div>
              {link ? (
                <Link
                  href={link.href}
                  className="inline-flex items-center gap-1 text-xs font-semibold text-brand-text"
                >
                  {link.label} <ArrowUpRight size={12} aria-hidden="true" />
                </Link>
              ) : null}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
