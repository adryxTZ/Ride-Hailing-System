"use client";
/**
 * TripPricingRecord — the immutable record of how the trip was priced: the
 * profile and version, where it applied, the country default's version, every
 * extra charge and who it went to, why that profile won, and the seal (hash)
 * once the trip completed. Download JSON gives the full record for an audit.
 */
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, Download, ShieldCheck } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Chip } from "@/components/ui/Chip";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import { api, type Schemas, unwrap } from "@/lib/api";
import { money, when } from "@/lib/format";

const OUTCOME: Record<string, string> = {
  WON: "Used",
  LOWER_PRECEDENCE: "Lower precedence",
  INACTIVE: "Switched off",
  DRAFT: "Draft",
  OUT_OF_WINDOW: "Outside its dates",
  NO_VERSION_IN_FORCE: "No version in force yet",
  TIE_BROKEN: "Tie (older one used)",
};
const RECIPIENT: Record<string, string> = {
  FARE: "Fare",
  PLATFORM: "Platform",
  DRIVER: "Driver",
  THIRD_PARTY: "Payee",
};
const SCOPE: Record<string, string> = {
  COUNTRY: "Country default",
  AREA: "Region",
  DISTRICT: "District",
  ZONE: "Zone",
  PLACE: "Place",
};

interface Candidate {
  scope: { kind: string; name: string };
  profile: { id: string; name: string; version?: number };
  outcome: string;
}

export function TripPricingRecord({ trip }: { trip: Schemas["Trip"] }) {
  const [why, setWhy] = useState(false);
  const {
    data: r,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["admin", "pricing-record", trip.id, trip.status],
    queryFn: async () =>
      unwrap(await api.GET("/admin/trips/{id}/pricing-record", { params: { path: { id: trip.id } } })),
    retry: false,
  });
  if (isLoading) return <Skeleton className="mx-5 my-4 h-20" />;
  if (!r)
    return error ? (
      <div className="px-5 py-4">
        <ErrorNote error={error} />
      </div>
    ) : null;
  const candidates = ((r.trace as { candidates?: Candidate[] }).candidates ?? []) as Candidate[];
  return (
    <section aria-label="Pricing record" className="grid gap-3 border-t border-line px-5 py-4 text-sm">
      <h3 className="flex items-center gap-2 text-xs font-semibold">
        <ShieldCheck size={15} className="text-muted" aria-hidden="true" />
        Pricing record
        <span className="ml-auto">
          {r.stage === "FINAL" ? (
            <Chip tone="ok">Final, cannot change</Chip>
          ) : (
            <Chip tone="live">Quoted</Chip>
          )}
        </span>
      </h3>
      <dl className="grid grid-cols-2 gap-2">
        <Fact label="Profile">
          <Link
            href={`/pricing/fees/${r.profile.id}`}
            className="font-medium text-brand-text"
          >{`${r.profile.name} · v${r.profile.version}`}</Link>
        </Fact>
        <Fact label="Applied as">{`${SCOPE[r.scope.kind ?? ""] ?? r.scope.kind}${r.scope.kind === "COUNTRY" ? "" : ` · ${r.scope.name}`}`}</Fact>
        <Fact label="Country default">
          {r.default_profile ? `${r.default_profile.name} · v${r.default_profile.version}` : "Not recorded"}
        </Fact>
        <Fact label="Recorded">{`${r.source === "LIVE" ? "At booking" : r.source === "BACKFILL" ? "From history" : "At settlement"} · ${when(r.created_at)}`}</Fact>
      </dl>
      {r.charges.length ? (
        <ul className="grid gap-1">
          {r.charges.map((c) => (
            <li
              key={c.code}
              className="flex items-baseline justify-between gap-2 rounded-[8px] bg-surface-2 px-3 py-1.5"
            >
              <span>{`${c.label?.en ?? c.code} · ${RECIPIENT[c.recipient ?? ""] ?? c.recipient}`}</span>
              <span className="tnum font-medium">{money(c.amount, trip.currency)}</span>
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-xs text-muted">No extra charges.</p>
      )}
      {candidates.length ? (
        <div>
          <button
            type="button"
            aria-expanded={why}
            onClick={() => setWhy(!why)}
            className="flex items-center gap-1 text-xs font-semibold text-brand-text"
          >
            Why this profile{" "}
            <ChevronDown
              size={13}
              aria-hidden="true"
              className={`transition-transform duration-200 ${why ? "rotate-180" : ""}`}
            />
          </button>
          {why ? (
            <ul className="mt-2 grid gap-1 text-xs">
              {candidates.map((c, i) => (
                <li key={`${c.profile.id}${i}`} className="flex justify-between gap-2">
                  <span>{`${SCOPE[c.scope.kind] ?? c.scope.kind}: ${c.profile.name}${c.profile.version ? ` v${c.profile.version}` : ""}`}</span>
                  <span className={c.outcome === "WON" ? "font-semibold" : "text-muted"}>
                    {OUTCOME[c.outcome] ?? c.outcome}
                  </span>
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : null}
      <div className="flex items-center justify-between gap-2 text-xs text-muted">
        <span className="font-mono">
          {r.record_hash ? `Seal ${r.record_hash.slice(0, 16)}…` : "Sealed when the trip completes"}
        </span>
        <button
          type="button"
          onClick={() => download(r)}
          className="inline-flex items-center gap-1 font-semibold text-brand-text"
        >
          <Download size={13} aria-hidden="true" /> Download JSON
        </button>
      </div>
    </section>
  );
}

function Fact({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="rounded-[8px] bg-surface-2 px-3 py-2">
      <dt className="text-xs text-muted">{label}</dt>
      <dd className="truncate">{children}</dd>
    </div>
  );
}

/** Saves the full record as a file for an auditor. */
function download(r: Schemas["PricingRecord"]) {
  const blob = new Blob([JSON.stringify(r, null, 2)], { type: "application/json" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = `pricing-record-${r.trip_id}.json`;
  a.click();
  URL.revokeObjectURL(a.href);
}
