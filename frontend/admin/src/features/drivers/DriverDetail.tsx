"use client";
/**
 * DriverDetail — one driver for review and support. The left column has the
 * application (details, vehicle, documents as attachments, decisions) and the
 * activity (trip figures, offer answers and response time, money, places and
 * routes, ratings, complaints and SOS, sessions, notes). The right column is
 * every trip they drove, searchable, with trips in progress on top.
 */
import { ArrowLeft, LogOut, Phone } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import { Tabs } from "@/components/ui/Tabs";
import { PersonFlagsCard } from "@/features/fraud/PersonFlagsCard";
import { useMe } from "@/features/me";
import { AccountCard } from "@/features/people/AccountCard";
import { ChartsView } from "@/features/people/ChartsView";
import { ComplaintsCard } from "@/features/people/ComplaintsCard";
import { useDriverActivity, useEndPersonSessions } from "@/features/people/hooks";
import { NotesCard } from "@/features/people/NotesCard";
import { PersonStats } from "@/features/people/PersonStats";
import { PlacesCard } from "@/features/people/PlacesCard";
import { RatingsCard } from "@/features/people/RatingsCard";
import { TripsPanel } from "@/features/people/TripsPanel";
import type { Schemas } from "@/lib/api";
import { useClassName } from "@/lib/catalog";
import { km, percent, seconds, when } from "@/lib/format";
import { useUrlState } from "@/lib/urlState";
import { DocumentAttachments } from "./DocumentAttachments";
import { DriverMoneyCard } from "./DriverMoneyCard";
import { useDocumentReview, useDriver, useDriverDecision } from "./hooks";
import { DRIVER_STATUS } from "./labels";

/** The left column shows the details or, optionally, the charts (?view=charts). */
const VIEWS = [
  { value: "details" as const, label: "Details" },
  { value: "charts" as const, label: "Charts" },
];
const DEFAULT_DAYS = 90;

/** Long lists (review history) show this many before "Show all". */
const LIST_SHOWN = 10;

type Ask =
  | { kind: "reject" | "suspend" | "reinstate" }
  | { kind: "doc"; docId: string }
  | { kind: "end"; sessionId?: string };

export function DriverDetail({ id }: { id: string }) {
  const { data: d, isLoading, error } = useDriver(id);
  const activity = useDriverActivity(id);
  const decide = useDriverDecision();
  const review = useDocumentReview();
  const end = useEndPersonSessions("driver");
  const me = useMe();
  const className = useClassName();
  const [ask, setAsk] = useState<Ask>();
  const canReview = me.can("drivers.review");
  const p = d?.profile;
  const v = d?.vehicle;
  const a = activity.data;
  const status = p?.status ?? "";
  const url = useUrlState();
  const view = url.get("view") === "charts" ? "charts" : "details";
  const days = Number(url.get("days") ?? DEFAULT_DAYS);
  return (
    <div>
      <PageHeader
        title={p?.full_name ?? "Driver"}
        subtitle={p ? [p.phone, a?.account.email].filter(Boolean).join(" · ") : undefined}
        actions={
          <Link
            href="/drivers"
            className="inline-flex items-center gap-1.5 text-sm font-semibold text-brand-text"
          >
            <ArrowLeft size={15} aria-hidden="true" /> All drivers
          </Link>
        }
      />
      <ErrorNote error={error ?? activity.error ?? decide.error ?? review.error ?? end.error} />
      {isLoading || !d || !p ? (
        <Skeleton className="h-96" />
      ) : (
        <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
          <div className="grid min-w-0 gap-4">
            <Card index={0}>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex flex-wrap items-center gap-2 text-sm">
                  <Chip tone={DRIVER_STATUS[status]?.tone}>{DRIVER_STATUS[status]?.label ?? status}</Chip>
                  {a?.account.status === "DISABLED" ? <Chip tone="bad">Account blocked</Chip> : null}
                  {a?.places.last_location?.source === "LIVE" ? <Chip tone="ok">Online now</Chip> : null}
                  {p.status_reason ? <span className="text-muted">{p.status_reason}</span> : null}
                  <span className="text-muted">{`Joined ${when(p.created_at)}`}</span>
                  {a?.account.last_seen_at ? (
                    <span className="text-muted">{`· Last active ${when(a.account.last_seen_at)}`}</span>
                  ) : null}
                </div>
                <div className="flex flex-wrap gap-2">
                  <a
                    href={`tel:${p.phone}`}
                    className="inline-flex h-10 items-center gap-2 rounded-[8px] bg-surface-2 px-4 text-sm font-semibold"
                  >
                    <Phone size={15} aria-hidden="true" /> Call
                  </a>
                  {canReview && a ? (
                    <Button
                      variant="soft"
                      icon={<LogOut size={15} aria-hidden="true" />}
                      disabled={!a.account.sessions.some((s) => s.active)}
                      onClick={() => setAsk({ kind: "end" })}
                    >
                      Sign out everywhere
                    </Button>
                  ) : null}
                  {canReview ? (
                    <Decisions
                      status={status}
                      busy={decide.isPending}
                      onApprove={() => decide.mutate({ id: p.user_id, action: "approve" })}
                      onAsk={(kind) => setAsk({ kind })}
                    />
                  ) : null}
                </div>
              </div>
            </Card>
            <Tabs
              id="driver-view"
              value={view}
              options={VIEWS}
              onChange={(v) => url.set({ view: v === "charts" ? v : undefined })}
            />
            {view === "charts" && a ? (
              <ChartsView
                role="driver"
                id={id}
                days={days}
                onDays={(n) => url.set({ days: n === DEFAULT_DAYS ? undefined : String(n) })}
                trips={a.trips}
                ratings={a.ratings}
              />
            ) : (
              <>
                {a ? (
                  <PersonStats trips={a.trips} rating={a.ratings.received} role="driver" />
                ) : (
                  <Skeleton className="h-40" />
                )}
                {a ? <PerformanceCard activity={a} /> : null}
                <Card index={7} title="Details and vehicle">
                  <dl className="grid grid-cols-2 gap-2 text-sm md:grid-cols-3">
                    <Fact label="Phone" value={p.phone} />
                    <Fact label="Country" value={p.country ?? "Not chosen"} />
                    <Fact label="Licence" value={p.licence_number ?? "Missing"} />
                    <Fact label="Licence class" value={p.licence_class ?? "Not given"} />
                    <Fact label="National ID" value={p.national_id ?? "Missing"} />
                    <Fact label="Submitted" value={p.submitted_at ? when(p.submitted_at) : "Not yet"} />
                    <Fact
                      label="Vehicle"
                      value={v ? `${className(v.vehicle_class)} · ${v.plate}` : "Not added"}
                    />
                    <Fact
                      label="Model"
                      value={
                        v
                          ? [v.colour, v.make, v.model, v.year].filter(Boolean).join(" ") || "Not given"
                          : "Not added"
                      }
                    />
                    <Fact label="Reviewed" value={p.reviewed_at ? when(p.reviewed_at) : "Not yet"} />
                  </dl>
                </Card>
                <Card index={8} title="Documents">
                  <DocumentAttachments
                    requirements={d.requirements}
                    documents={d.documents}
                    canReview={canReview}
                    onApprove={(docId) => review.mutate({ id: p.user_id, docId, approve: true })}
                    onReject={(docId) => setAsk({ kind: "doc", docId })}
                  />
                </Card>
                {a ? (
                  <>
                    <DriverMoneyCard money={a.money} index={9} />
                    <PlacesCard places={a.places} index={10} />
                    <RatingsCard ratings={a.ratings} index={11} who="the driver" />
                    <ComplaintsCard ratings={a.ratings} sos={a.sos} index={12} />
                    <PersonFlagsCard personId={id} index={12} />
                    <AccountCard
                      account={a.account}
                      index={13}
                      onEnd={canReview ? (sessionId) => setAsk({ kind: "end", sessionId }) : undefined}
                    />
                    <NotesCard id={id} notes={a.account.notes} index={14} />
                  </>
                ) : null}
                <Card index={15} title="Review history">
                  <History history={d.history} />
                </Card>
              </>
            )}
          </div>
          <TripsPanel role="driver" id={id} index={2} />
        </div>
      )}
      <ReasonDialog
        open={!!ask}
        title={dialogTitle(ask)}
        body={
          ask?.kind === "suspend"
            ? "They go offline at once and cannot take trips until reinstated."
            : ask?.kind === "end"
              ? "They must sign in again with a code on their phone."
              : "The driver sees your reason in the app."
        }
        confirm={
          ask?.kind === "suspend"
            ? "Suspend"
            : ask?.kind === "reinstate"
              ? "Reinstate"
              : ask?.kind === "end"
                ? "Sign out"
                : "Send"
        }
        danger={ask?.kind === "suspend" || ask?.kind === "end"}
        busy={decide.isPending || review.isPending || end.isPending}
        onClose={() => setAsk(undefined)}
        onConfirm={(reason) => {
          if (!p || !ask) return;
          const close = { onSuccess: () => setAsk(undefined) };
          if (ask.kind === "doc")
            review.mutate({ id: p.user_id, docId: ask.docId, approve: false, reason }, close);
          else if (ask.kind === "end") end.mutate({ id, sessionId: ask.sessionId, reason }, close);
          else decide.mutate({ id: p.user_id, action: ask.kind, reason }, close);
        }}
      />
    </div>
  );
}

function dialogTitle(ask?: Ask): string {
  switch (ask?.kind) {
    case "doc":
      return "Reject this document?";
    case "reject":
      return "Send back for changes?";
    case "suspend":
      return "Suspend this driver?";
    case "end":
      return ask.sessionId ? "End this session?" : "Sign this driver out everywhere?";
    default:
      return "Reinstate this driver?";
  }
}

/** The review buttons the driver's status allows. */
function Decisions({
  status,
  busy,
  onApprove,
  onAsk,
}: {
  status: string;
  busy: boolean;
  onApprove: () => void;
  onAsk: (kind: "reject" | "suspend" | "reinstate") => void;
}) {
  if (status === "PENDING_REVIEW")
    return (
      <>
        <Button variant="brand" loading={busy} onClick={onApprove}>
          Approve driver
        </Button>
        <Button variant="soft" onClick={() => onAsk("reject")}>
          Send back for changes
        </Button>
      </>
    );
  if (status === "APPROVED")
    return (
      <Button variant="danger" onClick={() => onAsk("suspend")}>
        Suspend
      </Button>
    );
  if (status === "SUSPENDED")
    return (
      <Button variant="primary" onClick={() => onAsk("reinstate")}>
        Reinstate
      </Button>
    );
  return null;
}

/** How the driver works: offers answered, response time, pickups, rides and payments. */
function PerformanceCard({ activity: a }: { activity: Schemas["AdminDriverActivity"] }) {
  const o = a.offers;
  const t = a.trips;
  const facts: [string, string][] = [
    ["Offers received", `${o?.offered ?? 0}`],
    ["Acceptance rate", o?.acceptance_rate != null ? percent(o.acceptance_rate) : "None yet"],
    ["Average response time", seconds(o?.avg_response_s)],
    ["Declined · timed out", `${o?.declined ?? 0} · ${o?.timed_out ?? 0}`],
    ["Average time to pickup", seconds(t.avg_pickup_s)],
    ["Average ride time", seconds(t.avg_ride_s)],
    ["Distance driven", km(t.distance_m)],
    ["Cash · mobile money", `${t.paid_cash} · ${t.paid_mobile}`],
    ["Cancelled by riders", `${t.cancelled_by_rider}`],
    ["Failed payments", `${t.unpaid_fares}`],
    ["First trip", t.first_trip_at ? when(t.first_trip_at) : "None yet"],
    ["Last offer", o?.last_offer_at ? when(o.last_offer_at) : "None yet"],
  ];
  return (
    <Card index={6} title="Performance">
      <dl className="grid grid-cols-2 gap-2 text-sm md:grid-cols-4">
        {facts.map(([label, value]) => (
          <Fact key={label} label={label} value={value} />
        ))}
      </dl>
    </Card>
  );
}

function History({ history }: { history: Schemas["DriverDetail"]["history"] }) {
  const [all, setAll] = useState(false);
  if (history.length === 0) return <p className="text-sm text-muted">No decisions yet.</p>;
  return (
    <div className="grid gap-3">
      <ol className="grid gap-2 border-l border-line pl-4 text-sm">
        {(all ? history : history.slice(0, LIST_SHOWN)).map((h) => (
          <li key={h.id} className="relative">
            <span
              className="absolute -left-[21px] top-1.5 size-2.5 rounded-full bg-brand"
              aria-hidden="true"
            />
            <span className="font-medium">{h.action.replace(/[._]/g, " ")}</span>
            <span className="text-muted">{` by ${h.actor ?? "system"} · ${when(h.at)}`}</span>
            {h.reason ? <p className="text-muted">{h.reason}</p> : null}
          </li>
        ))}
      </ol>
      {history.length > LIST_SHOWN ? (
        <Button size="sm" variant="ghost" className="justify-self-start" onClick={() => setAll(!all)}>
          {all ? "Show fewer" : `Show all ${history.length}`}
        </Button>
      ) : null}
    </div>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[8px] bg-surface-2 px-3 py-2">
      <dt className="text-xs text-muted">{label}</dt>
      <dd className="tnum truncate font-medium">{value}</dd>
    </div>
  );
}
