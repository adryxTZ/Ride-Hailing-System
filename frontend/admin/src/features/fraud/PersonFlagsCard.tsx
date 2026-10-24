"use client";
/** PersonFlagsCard — a rider's or driver's fraud flags (open first), linking to each review. */
import Link from "next/link";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Empty } from "@/components/ui/States";
import { when } from "@/lib/format";
import { useFlags } from "./hooks";
import { RULE_LABEL, STATUS_LABEL, STATUS_TONE } from "./labels";

const SHOWN = 5;

export function PersonFlagsCard({ personId, index }: { personId: string; index: number }) {
  const { data } = useFlags("", "", "", personId);
  const flags = [...(data ?? [])].sort((a, b) => Number(b.status === "OPEN") - Number(a.status === "OPEN"));
  const open = flags.filter((f) => f.status === "OPEN").length;
  return (
    <Card title={open ? `Fraud flags (${open} to review)` : "Fraud flags"} index={index}>
      {!flags.length ? (
        <Empty title="No flags" body="No fraud check has raised anything about this person." />
      ) : (
        <ul className="divide-y divide-line">
          {flags.slice(0, SHOWN).map((f) => (
            <li key={f.id}>
              <Link
                href={`/compliance/fraud/${f.id}`}
                className="flex items-center justify-between gap-3 py-2.5 text-xs hover:text-brand-text"
              >
                <span className="font-semibold">{RULE_LABEL[f.rule] ?? f.rule}</span>
                <span className="flex items-center gap-2">
                  <span className="tnum text-[11px] text-muted">{when(f.opened_at)}</span>
                  <Chip tone={STATUS_TONE[f.status]}>{STATUS_LABEL[f.status]}</Chip>
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
