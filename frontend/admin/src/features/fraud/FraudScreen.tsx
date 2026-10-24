"use client";
/**
 * FraudScreen — the review queue: flags raised by the fake-GPS, self-booking
 * and cancellation checks in the selected country. Nothing here punishes
 * anyone; staff open a flag, look at the evidence and dismiss or confirm it.
 */
import { useRouter } from "next/navigation";
import { Suspense } from "react";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Select } from "@/components/ui/Field";
import { FilterBar } from "@/components/ui/Operations";
import { PageHeader } from "@/components/ui/PageHeader";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { when } from "@/lib/format";
import { useScope } from "@/lib/scope";
import { useUrlState } from "@/lib/urlState";
import { type FlagStatus, type Rule, useFlags } from "./hooks";
import { RULE_LABEL, SEVERITY_LABEL, SEVERITY_TONE, STATUS_LABEL, STATUS_TONE } from "./labels";

const STATUSES = [
  { value: "OPEN", label: "To review" },
  { value: "CONFIRMED", label: "Confirmed" },
  { value: "DISMISSED", label: "Dismissed" },
  { value: "", label: "All" },
] as const;

export function FraudScreen() {
  return (
    <Suspense fallback={<Skeleton className="h-96" />}>
      <Queue />
    </Suspense>
  );
}

function Queue() {
  const { scope } = useScope();
  const url = useUrlState();
  const router = useRouter();
  const raw = url.get("status") ?? "OPEN";
  const status = (raw === "ALL" ? "" : raw) as FlagStatus | ""; // "ALL" in the URL, "" to the API
  const rule = (url.get("rule") ?? "") as Rule | "";
  const { data, isLoading, error } = useFlags(scope.country, status, rule);
  return (
    <div>
      <PageHeader
        title="Fraud flags"
        subtitle="Possible fake GPS, self-booked rides and cancellation abuse. Review each one; suspend, hold or block from the person's page if it is real."
      />
      <Card index={1}>
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <FilterBar
            label="Status"
            value={status}
            options={STATUSES}
            onChange={(v) => url.set({ status: v === "OPEN" ? undefined : v || "ALL" })}
          />
          <div className="w-full sm:w-56">
            <Select
              aria-label="Check"
              value={rule}
              onChange={(e) => url.set({ rule: e.target.value || undefined })}
            >
              <option value="">Every check</option>
              {Object.entries(RULE_LABEL).map(([k, v]) => (
                <option key={k} value={k}>
                  {v}
                </option>
              ))}
            </Select>
          </div>
        </div>
        {error ? <ErrorNote error={error} /> : null}
        {isLoading ? (
          <Skeleton className="h-64" />
        ) : !data?.length ? (
          <Empty
            title={status === "OPEN" ? "Nothing to review" : "No flags here"}
            body={
              status === "OPEN"
                ? "New flags appear here as the checks raise them."
                : "Try another status or check."
            }
          />
        ) : (
          <Table head={["Check", "Severity", "Who", "Trip", "Seen", "Raised", "Status"]}>
            {data.map((f) => (
              <Row key={f.id} onClick={() => router.push(`/compliance/fraud/${f.id}`)}>
                <Cell className="text-xs font-semibold">{RULE_LABEL[f.rule] ?? f.rule}</Cell>
                <Cell>
                  <Chip tone={SEVERITY_TONE[f.severity]}>{SEVERITY_LABEL[f.severity]}</Chip>
                </Cell>
                <Cell className="text-xs">
                  {f.subject.name ?? f.subject.phone}
                  <span className="ml-1.5 text-[10px] text-muted">
                    {f.subject.kind === "DRIVER" ? "Driver" : "Rider"}
                  </span>
                </Cell>
                <Cell className="tnum text-xs text-muted">{f.trip_code ?? "—"}</Cell>
                <Cell className="tnum text-xs">{f.hits === 1 ? "Once" : `${f.hits} times`}</Cell>
                <Cell className="tnum whitespace-nowrap text-[11px] text-muted">{when(f.opened_at)}</Cell>
                <Cell>
                  <Chip tone={STATUS_TONE[f.status]}>{STATUS_LABEL[f.status]}</Chip>
                </Cell>
              </Row>
            ))}
          </Table>
        )}
      </Card>
    </div>
  );
}
