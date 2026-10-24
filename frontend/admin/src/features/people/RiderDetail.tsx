"use client";
/**
 * RiderDetail — one rider for support: who they are, trips and money, where
 * they go, ratings, complaints and SOS, sessions and devices, staff notes, and
 * the controls to block or sign them out.
 */
import { ArrowLeft, LogOut, Phone, ShieldBan } from "lucide-react";
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
import { StaffRolesCard } from "@/features/staff/StaffRolesCard";
import type { Schemas } from "@/lib/api";
import { km, money, seconds, when } from "@/lib/format";
import { useUrlState } from "@/lib/urlState";
import { AccountCard } from "./AccountCard";
import { ChartsView } from "./ChartsView";
import { ComplaintsCard } from "./ComplaintsCard";
import { useEndPersonSessions, useRider, useRiderStatus } from "./hooks";
import { NotesCard } from "./NotesCard";
import { PersonStats } from "./PersonStats";
import { PlacesCard } from "./PlacesCard";
import { RatingsCard } from "./RatingsCard";
import { TripsPanel } from "./TripsPanel";

type Ask = { kind: "end"; sessionId?: string } | { kind: "status" };

/** The left column shows the details or, optionally, the charts (?view=charts). */
const VIEWS = [
  { value: "details" as const, label: "Details" },
  { value: "charts" as const, label: "Charts" },
];
const DEFAULT_DAYS = 90;

export function RiderDetail({ id }: { id: string }) {
  const { data: r, isLoading, error } = useRider(id);
  const status = useRiderStatus();
  const end = useEndPersonSessions("rider");
  const me = useMe();
  const [ask, setAsk] = useState<Ask>();
  const canManage = me.can("riders.block");
  const a = r?.account;
  const blocked = a?.status !== "ACTIVE";
  const url = useUrlState();
  const view = url.get("view") === "charts" ? "charts" : "details";
  const days = Number(url.get("days") ?? DEFAULT_DAYS);
  return (
    <div>
      <PageHeader
        title={a?.full_name ?? "Rider"}
        subtitle={a ? [a.phone, a.email].filter(Boolean).join(" · ") : undefined}
        actions={
          <Link
            href="/riders"
            className="inline-flex items-center gap-1.5 text-sm font-semibold text-brand-text"
          >
            <ArrowLeft size={15} aria-hidden="true" /> All riders
          </Link>
        }
      />
      <ErrorNote error={error ?? status.error ?? end.error} />
      {isLoading || !r || !a ? (
        <Skeleton className="h-96" />
      ) : (
        <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
          <div className="grid min-w-0 gap-4">
            <Card index={0}>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex flex-wrap items-center gap-2 text-sm">
                  {blocked ? <Chip tone="bad">Blocked</Chip> : <Chip tone="ok">Active</Chip>}
                  {Number(r.dues) > 0 ? (
                    <Chip tone="warn">{`Owes ${money(r.dues, r.trips.currency || undefined)}`}</Chip>
                  ) : null}
                  <span className="text-muted">{`Joined ${when(a.created_at)}`}</span>
                  <span className="text-muted">
                    {a.last_seen_at ? `· Last active ${when(a.last_seen_at)}` : "· Never signed in"}
                  </span>
                  <span className="text-muted">{`· Language ${a.language === "sw" ? "Swahili" : "English"}`}</span>
                </div>
                <div className="flex flex-wrap gap-2">
                  <a
                    href={`tel:${a.phone}`}
                    className="inline-flex h-10 items-center gap-2 rounded-[8px] bg-surface-2 px-4 text-sm font-semibold"
                  >
                    <Phone size={15} aria-hidden="true" /> Call
                  </a>
                  {canManage ? (
                    <>
                      <Button
                        variant="soft"
                        icon={<LogOut size={15} aria-hidden="true" />}
                        disabled={!a.sessions.some((s) => s.active)}
                        onClick={() => setAsk({ kind: "end" })}
                      >
                        Sign out everywhere
                      </Button>
                      <Button
                        variant={blocked ? "primary" : "danger"}
                        icon={<ShieldBan size={15} aria-hidden="true" />}
                        onClick={() => setAsk({ kind: "status" })}
                      >
                        {blocked ? "Unblock" : "Block"}
                      </Button>
                    </>
                  ) : null}
                </div>
              </div>
            </Card>
            <Tabs
              id="rider-view"
              value={view}
              options={VIEWS}
              onChange={(v) => url.set({ view: v === "charts" ? v : undefined })}
            />
            {view === "charts" ? (
              <ChartsView
                role="rider"
                id={id}
                days={days}
                onDays={(d) => url.set({ days: d === DEFAULT_DAYS ? undefined : String(d) })}
                trips={r.trips}
                ratings={r.ratings}
              />
            ) : (
              <>
                <PersonStats trips={r.trips} rating={r.ratings.received} role="rider" />
                <PaymentsCard rider={r} />
                <PlacesCard places={r.places} index={7} />
                <RatingsCard ratings={r.ratings} index={8} who="the rider" />
                <ComplaintsCard ratings={r.ratings} sos={r.sos} index={9} />
                <PersonFlagsCard personId={id} index={10} />
                <AccountCard
                  account={a}
                  index={11}
                  onEnd={canManage ? (sessionId) => setAsk({ kind: "end", sessionId }) : undefined}
                />
                <NotesCard id={id} notes={a.notes} index={12} />
                <StaffRolesCard phone={a.phone} name={a.full_name} index={13} />
              </>
            )}
          </div>
          <TripsPanel role="rider" id={id} index={2} />
        </div>
      )}
      <ReasonDialog
        open={ask?.kind === "end"}
        title={ask?.kind === "end" && ask.sessionId ? "End this session?" : "Sign this rider out everywhere?"}
        body="They must sign in again with a code on their phone."
        confirm="Sign out"
        danger
        busy={end.isPending}
        onClose={() => setAsk(undefined)}
        onConfirm={(reason) =>
          ask?.kind === "end" &&
          end.mutate({ id, sessionId: ask.sessionId, reason }, { onSuccess: () => setAsk(undefined) })
        }
      />
      <ReasonDialog
        open={ask?.kind === "status"}
        title={
          blocked ? `Unblock ${a?.full_name ?? "this rider"}?` : `Block ${a?.full_name ?? "this rider"}?`
        }
        body={
          blocked
            ? "They can sign in and book again."
            : "They are signed out and cannot sign in or book until unblocked."
        }
        confirm={blocked ? "Unblock" : "Block"}
        danger={!blocked}
        busy={status.isPending}
        onClose={() => setAsk(undefined)}
        onConfirm={(reason) =>
          status.mutate({ id, block: !blocked, reason }, { onSuccess: () => setAsk(undefined) })
        }
      />
    </div>
  );
}

/** How the rider pays: cash vs mobile money, failed payments, dues, distance and waits. */
function PaymentsCard({ rider }: { rider: Schemas["AdminRider"] }) {
  const t = rider.trips;
  const facts: [string, string][] = [
    ["Paid in cash", `${t.paid_cash} trips`],
    ["Paid by mobile money", `${t.paid_mobile} trips`],
    ["Failed payments", `${t.unpaid_fares}`],
    ["Cancellation fees owed", money(rider.dues, t.currency || undefined)],
    ["Distance travelled", km(t.distance_m)],
    ["Average wait for a driver", seconds(t.avg_pickup_s)],
    ["First trip", t.first_trip_at ? when(t.first_trip_at) : "None yet"],
    ["Last trip", t.last_trip_at ? when(t.last_trip_at) : "None yet"],
  ];
  return (
    <Card index={6} title="Payments and usage">
      <dl className="grid grid-cols-2 gap-2 text-sm md:grid-cols-4">
        {facts.map(([label, value]) => (
          <div key={label} className="rounded-[8px] bg-surface-2 px-3 py-2">
            <dt className="text-xs text-muted">{label}</dt>
            <dd className="tnum truncate font-medium">{value}</dd>
          </div>
        ))}
      </dl>
    </Card>
  );
}
