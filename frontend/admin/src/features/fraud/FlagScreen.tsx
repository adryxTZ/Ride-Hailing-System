"use client";
/**
 * FlagScreen — one fraud flag: what the check saw, who it is about, and the
 * review. Dismiss or confirm with a note; a reviewed flag never changes.
 * Suspending, holding or blocking happens on the person's own page.
 */
import { ArrowLeft, ArrowUpRight } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Field } from "@/components/ui/Field";
import { PageHeader } from "@/components/ui/PageHeader";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import { useMe } from "@/features/me";
import { when } from "@/lib/format";
import { EvidenceView } from "./EvidenceView";
import { type Flag, useFlag, useReview } from "./hooks";
import { RULE_HELP, RULE_LABEL, SEVERITY_LABEL, SEVERITY_TONE, STATUS_LABEL, STATUS_TONE } from "./labels";

const NOTE_MAX = 1000;

export function FlagScreen({ id }: { id: string }) {
  const { data: flag, isLoading, error } = useFlag(id);
  if (isLoading) return <Skeleton className="h-96" />;
  if (error || !flag) return <ErrorNote error={error ?? new Error("Flag not found")} />;
  const person =
    flag.subject.kind === "DRIVER" ? `/drivers/${flag.subject.id}` : `/riders/${flag.subject.id}`;
  return (
    <div>
      <PageHeader
        title={RULE_LABEL[flag.rule] ?? flag.rule}
        subtitle={RULE_HELP[flag.rule]}
        actions={
          <Link
            href="/compliance/fraud"
            className="inline-flex items-center gap-1.5 text-xs font-semibold text-muted hover:text-ink"
          >
            <ArrowLeft size={14} aria-hidden="true" /> Fraud flags
          </Link>
        }
      />
      <div className="grid gap-4 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        <Card title="What the check saw" index={1}>
          <EvidenceView flag={flag} />
        </Card>
        <div className="grid content-start gap-4">
          <Card title="About" index={2}>
            <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2.5 text-xs">
              <dt className="text-muted">Who</dt>
              <dd>
                <Link
                  href={person}
                  className="inline-flex items-center gap-1 font-semibold hover:text-brand-text"
                >
                  {flag.subject.name ?? flag.subject.phone} <ArrowUpRight size={12} aria-hidden="true" />
                </Link>
                <span className="ml-1.5 text-muted">{`${flag.subject.kind === "DRIVER" ? "Driver" : "Rider"} · ${flag.subject.phone}`}</span>
              </dd>
              <dt className="text-muted">Severity</dt>
              <dd>
                <Chip tone={SEVERITY_TONE[flag.severity]}>{SEVERITY_LABEL[flag.severity]}</Chip>
              </dd>
              {flag.trip_id ? (
                <>
                  <dt className="text-muted">Trip</dt>
                  <dd>
                    <Link
                      href={`/trips/${flag.trip_id}`}
                      className="tnum font-semibold hover:text-brand-text"
                    >
                      {flag.trip_code ?? flag.trip_id.slice(0, 8)}
                    </Link>
                  </dd>
                </>
              ) : null}
              <dt className="text-muted">Raised</dt>
              <dd className="tnum">{`${when(flag.opened_at)}${flag.hits > 1 ? `, seen ${flag.hits} times (last ${when(flag.last_seen_at)})` : ""}`}</dd>
              <dt className="text-muted">Status</dt>
              <dd>
                <Chip tone={STATUS_TONE[flag.status]}>{STATUS_LABEL[flag.status]}</Chip>
              </dd>
            </dl>
          </Card>
          <Card title="Review" index={3}>
            {flag.status === "OPEN" ? <ReviewForm flag={flag} person={person} /> : <Reviewed flag={flag} />}
          </Card>
        </div>
      </div>
    </div>
  );
}

function Reviewed({ flag }: { flag: Flag }) {
  return (
    <div className="grid gap-2 text-xs">
      <p>{`${STATUS_LABEL[flag.status]} by ${flag.reviewed_by ?? "staff"}, ${when(flag.reviewed_at)}.`}</p>
      {flag.review_note ? (
        <p className="rounded-[8px] bg-surface-2 px-3 py-2 leading-relaxed">{flag.review_note}</p>
      ) : null}
    </div>
  );
}

function ReviewForm({ flag, person }: { flag: Flag; person: string }) {
  const review = useReview(flag.id);
  const [note, setNote] = useState("");
  const canReview = useMe().can("fraud.review");
  if (!canReview)
    return (
      <p className="text-xs text-muted">
        You can see this flag, but reviewing needs the "Review fraud flags" permission.
      </p>
    );
  const send = (decision: "DISMISS" | "CONFIRM") =>
    note.trim() && review.mutate({ decision, note: note.trim() });
  return (
    <div className="grid gap-3">
      <Field label="Note" hint="What you checked and why. Kept with the flag and in the audit log.">
        <textarea
          value={note}
          maxLength={NOTE_MAX}
          rows={4}
          onChange={(e) => setNote(e.target.value)}
          className="w-full rounded-[8px] bg-surface-2 px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-brand"
        />
      </Field>
      <div className="flex flex-wrap gap-2">
        <Button
          variant="soft"
          disabled={!note.trim()}
          loading={review.isPending && review.variables?.decision === "DISMISS"}
          onClick={() => send("DISMISS")}
        >
          Dismiss
        </Button>
        <Button
          variant="danger"
          disabled={!note.trim()}
          loading={review.isPending && review.variables?.decision === "CONFIRM"}
          onClick={() => send("CONFIRM")}
        >
          Confirm
        </Button>
      </div>
      <p className="text-[11px] leading-relaxed text-muted">
        Confirming does not punish anyone by itself. To suspend, hold or block, use{" "}
        <Link href={person} className="font-semibold text-brand-text">
          the person's page
        </Link>
        .
      </p>
      {review.error ? <ErrorNote error={review.error} /> : null}
    </div>
  );
}
