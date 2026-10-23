"use client";
/**
 * PayoutsCard — drivers' cash-out requests: approve or reject the ones waiting
 * (with a reason, audited), follow the ones with the gateway, and see stuck ones.
 */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { Tabs } from "@/components/ui/Tabs";
import type { Schemas } from "@/lib/api";
import { money, when } from "@/lib/format";
import { useUrlState } from "@/lib/urlState";
import { usePayoutDecision, usePayouts } from "./hooks";
import { PAYOUT_FAILURE } from "./labels";

const STATUSES = [
  { value: "", label: "All" },
  { value: "REQUESTED", label: "To approve" },
  { value: "PENDING", label: "Sending" },
  { value: "PAID", label: "Paid" },
  { value: "FAILED", label: "Failed" },
];

type Payout = Schemas["AdminPayout"];

export function PayoutsCard({ index, editable }: { index: number; editable: boolean }) {
  const url = useUrlState();
  const status = url.get("payouts") ?? "";
  const { data, isLoading, error } = usePayouts(status);
  const decide = usePayoutDecision();
  const [ask, setAsk] = useState<{ payout: Payout; approve: boolean }>();
  return (
    <Card
      index={index}
      title="Cash-outs"
      action={
        <Tabs
          id="payouts"
          value={status}
          options={STATUSES}
          onChange={(v) => url.set({ payouts: v || undefined })}
        />
      }
    >
      <ErrorNote error={error ?? decide.error} />
      {data && !data.enabled ? (
        <p className="mb-3 text-sm text-muted">
          No payout gateway is configured, so drivers cannot cash out yet.
        </p>
      ) : null}
      {isLoading ? (
        <Skeleton className="h-40" />
      ) : !data?.data.length ? (
        <Empty
          title="No cash-outs"
          body="Drivers ask to cash out what the platform owes them; requests appear here."
        />
      ) : (
        <Table head={["Asked", "Driver", "Amount", "Balance now", "Status", ""]}>
          {data.data.map((p) => (
            <Row key={p.id}>
              <Cell className="tnum whitespace-nowrap text-muted">{when(p.created_at)}</Cell>
              <Cell>
                <div className="font-medium">{p.name ?? "No name"}</div>
                <div className="tnum text-xs text-muted">{p.phone}</div>
              </Cell>
              <Cell className="tnum">{money(p.amount, p.currency)}</Cell>
              <Cell
                className={`tnum ${Number(p.balance) < Number(p.amount) && p.status === "REQUESTED" ? "text-critical" : "text-muted"}`}
              >
                {money(p.balance, p.currency)}
              </Cell>
              <Cell>
                <PayoutStatus payout={p} />
              </Cell>
              <Cell className="whitespace-nowrap text-right">
                {editable && p.status === "REQUESTED" ? (
                  <div className="flex justify-end gap-2">
                    <Button size="sm" variant="ghost" onClick={() => setAsk({ payout: p, approve: false })}>
                      Reject
                    </Button>
                    <Button size="sm" onClick={() => setAsk({ payout: p, approve: true })}>
                      Approve
                    </Button>
                  </div>
                ) : null}
              </Cell>
            </Row>
          ))}
        </Table>
      )}
      <ReasonDialog
        open={!!ask}
        title={
          ask
            ? `${ask.approve ? "Send" : "Reject"} ${money(ask.payout.amount, ask.payout.currency)} to ${ask.payout.name ?? ask.payout.phone}?`
            : ""
        }
        body={
          ask?.approve
            ? `The money goes to ${ask.payout.phone}. The balance is checked again before sending.`
            : "The driver sees your reason."
        }
        confirm={ask?.approve ? "Approve" : "Reject"}
        danger={!ask?.approve}
        busy={decide.isPending}
        onClose={() => setAsk(undefined)}
        onConfirm={(reason) =>
          ask &&
          decide.mutate(
            { id: ask.payout.id, approve: ask.approve, reason },
            { onSuccess: () => setAsk(undefined) },
          )
        }
      />
    </Card>
  );
}

function PayoutStatus({ payout: p }: { payout: Payout }) {
  if (p.stuck) return <Chip tone="warn">Stuck at the gateway</Chip>;
  switch (p.status) {
    case "REQUESTED":
      return <Chip tone="live">To approve</Chip>;
    case "PENDING":
      return <Chip tone="live">Sending</Chip>;
    case "PAID":
      return <Chip tone="ok">Paid</Chip>;
    case "REJECTED":
      return (
        <span title={p.decision_reason ?? ""}>
          <Chip tone="muted">Rejected</Chip>
        </span>
      );
    case "CANCELLED":
      return <Chip tone="muted">Cancelled by driver</Chip>;
    default:
      return <Chip tone="bad">{PAYOUT_FAILURE[p.failure_code ?? ""] ?? "Failed"}</Chip>;
  }
}
