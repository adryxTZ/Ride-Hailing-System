"use client";
/** ExportCard — download finalised trip pricing records (up to 31 days) as CSV for audit. */
import { Download } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Field, Input } from "@/components/ui/Field";
import { ErrorNote } from "@/components/ui/States";
import { isoOf, localOf, useExportRecords } from "./hooks";

const MAX_DAYS = 31;
const DAY_MS = 86_400_000;

export function ExportCard({ country }: { country: string }) {
  const [from, setFrom] = useState(() => localOf(new Date(Date.now() - 7 * DAY_MS)));
  const [to, setTo] = useState(() => localOf(new Date()));
  const exp = useExportRecords();
  const days = (new Date(to).getTime() - new Date(from).getTime()) / DAY_MS;
  const tooLong = days > MAX_DAYS;
  return (
    <Card title="Export pricing records" index={0}>
      <p className="mb-4 text-xs leading-relaxed text-muted">
        One row per completed trip: the profile and version it was priced under, where it applied, its
        charges, the final split and the record's seal.
      </p>
      <div className="grid gap-4 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
        <Field label="From">
          <Input type="datetime-local" value={from} onChange={(e) => setFrom(e.target.value)} />
        </Field>
        <Field label="To" hint={tooLong ? `Up to ${MAX_DAYS} days at a time.` : undefined}>
          <Input type="datetime-local" value={to} onChange={(e) => setTo(e.target.value)} />
        </Field>
        <Button
          icon={<Download size={15} aria-hidden="true" />}
          disabled={tooLong || days <= 0}
          loading={exp.isPending}
          onClick={() => exp.mutate({ country, from: isoOf(from), to: isoOf(to) })}
        >
          Download CSV
        </Button>
      </div>
      {exp.error ? (
        <div className="mt-3">
          <ErrorNote error={exp.error} />
        </div>
      ) : null}
    </Card>
  );
}
