"use client";
/** ResolutionTrace — every profile that covered the point, and why one applied (pickup, drop-off, crossed zones). */
import Link from "next/link";
import { Chip } from "@/components/ui/Chip";
import { Table } from "@/components/ui/Table";
import { KIND_LABEL, OUTCOME_LABEL, OUTCOME_TONE } from "./labels";

type Ref = { id?: string; name?: string; version?: number };
type Candidate = {
  scope: { kind: string; id: string; name: string };
  profile: Ref;
  priority: number;
  outcome: string;
};
/** The resolver's trace as the API sends it. */
export type Trace = { candidates?: Candidate[]; dropoff?: Trace | null; crossed?: Candidate[] | null };

function Rows({ point, list }: { point: string; list: Candidate[] }) {
  return list.map((c) => (
    <tr key={`${point}-${c.scope.kind}-${c.scope.id}`}>
      <td className="px-3 py-3 text-xs text-muted">{point}</td>
      <td className="px-3 py-3 text-xs">
        {c.scope.name}
        <span className="ml-1.5 text-[10px] text-muted">{KIND_LABEL[c.scope.kind] ?? c.scope.kind}</span>
      </td>
      <td className="px-3 py-3 text-xs">
        {c.profile.id ? (
          <Link href={`/pricing/fees/${c.profile.id}`} className="font-semibold hover:text-brand-text">
            {c.profile.name}
          </Link>
        ) : (
          c.profile.name
        )}
        {c.profile.version ? <span className="tnum ml-1.5 text-muted">{`v${c.profile.version}`}</span> : null}
      </td>
      <td className="tnum px-3 py-3 text-xs">{c.priority}</td>
      <td className="px-3 py-3">
        <Chip tone={OUTCOME_TONE[c.outcome] ?? "muted"}>{OUTCOME_LABEL[c.outcome] ?? c.outcome}</Chip>
      </td>
    </tr>
  ));
}

export function ResolutionTrace({ trace }: { trace: Trace }) {
  return (
    <Table head={["Point", "Where", "Profile", "Priority", "Outcome"]}>
      <Rows point="Pickup" list={trace.candidates ?? []} />
      <Rows point="Drop-off" list={trace.dropoff?.candidates ?? []} />
      <Rows point="Route" list={trace.crossed ?? []} />
    </Table>
  );
}
