"use client";
/** DriverMoneyCard — wallet balance and entries, holds, cash-outs and pay-ins, in tabs. */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Empty } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { Tabs } from "@/components/ui/Tabs";
import type { Schemas } from "@/lib/api";
import { money, when } from "@/lib/format";

type Tab = "wallet" | "payouts" | "payins";

/** Wallet entries listed before "Show all" (the API returns the latest 30). */
const ENTRIES_SHOWN = 10;

const ENTRY: Record<string, string> = {
  "trip.completed": "Trip",
  "trip.cancellation_fee": "Cancellation fee",
  "trip.dues_collected": "Dues collected",
  "payin.collected": "Paid in",
  "payout.paid": "Cashed out",
};
const PAYOUT_TONE: Record<string, "ok" | "warn" | "bad" | "muted"> = {
  PAID: "ok",
  PENDING: "warn",
  REQUESTED: "warn",
  FAILED: "bad",
  REJECTED: "bad",
  CANCELLED: "muted",
};

export function DriverMoneyCard({ money: m, index }: { money: Schemas["DriverMoney"]; index: number }) {
  const [tab, setTab] = useState<Tab>("wallet");
  const [allEntries, setAllEntries] = useState(false);
  const currency = m.wallet.currency || "TZS";
  const balance = Number(m.wallet.balance);
  const options = [
    { value: "wallet" as const, label: "Wallet" },
    { value: "payouts" as const, label: `Cash-outs (${m.payouts.length})` },
    { value: "payins" as const, label: `Pay-ins (${m.payins.length})` },
  ];
  return (
    <Card
      index={index}
      title="Money"
      action={<Tabs id="driver-money" value={tab} options={options} onChange={setTab} />}
    >
      <div className="mb-4 flex flex-wrap items-center gap-3">
        <p className="tnum text-[26px] font-semibold tracking-tight">{money(m.wallet.balance, currency)}</p>
        <span className="text-sm text-muted">
          {balance < 0 ? "owed to the platform" : "owed to the driver"}
        </span>
        {Number(m.holds.debt) > 0 ? <Chip tone="warn">{`Debt ${money(m.holds.debt, currency)}`}</Chip> : null}
        {m.holds.holds.map((h) => (
          <Chip key={h.id} tone="bad">{`On hold: ${h.reason ?? h.kind.toLowerCase()}`}</Chip>
        ))}
      </div>
      {tab === "wallet" ? (
        !m.wallet.entries.length ? (
          <Empty title="No wallet entries" body="Entries appear after the first trip or payment." />
        ) : (
          <>
            <Table head={["When", "What", "Reference", "Amount"]}>
              {(allEntries ? m.wallet.entries : m.wallet.entries.slice(0, ENTRIES_SHOWN)).map((e, i) => (
                <Row key={`${e.reference}${i}`}>
                  <Cell className="tnum whitespace-nowrap text-muted">{when(e.at)}</Cell>
                  <Cell>{ENTRY[e.kind] ?? e.kind}</Cell>
                  <Cell className="max-w-[200px] truncate font-mono text-xs text-muted">
                    {e.memo || e.reference}
                  </Cell>
                  <Cell
                    className={`tnum whitespace-nowrap text-right font-semibold ${Number(e.amount) < 0 ? "text-critical" : ""}`}
                  >
                    {money(e.amount, currency)}
                  </Cell>
                </Row>
              ))}
            </Table>
            {m.wallet.entries.length > ENTRIES_SHOWN ? (
              <Button size="sm" variant="ghost" className="mt-2" onClick={() => setAllEntries(!allEntries)}>
                {allEntries ? "Show fewer" : `Show all ${m.wallet.entries.length}`}
              </Button>
            ) : null}
          </>
        )
      ) : tab === "payouts" ? (
        !m.payouts.length ? (
          <Empty title="No cash-outs" body="Cash-outs the driver asks for appear here." />
        ) : (
          <Table head={["Asked", "To", "Status", "Amount"]}>
            {m.payouts.map((p) => (
              <Row key={p.id}>
                <Cell className="tnum whitespace-nowrap text-muted">{when(p.created_at)}</Cell>
                <Cell className="tnum">{p.phone}</Cell>
                <Cell>
                  <Chip tone={PAYOUT_TONE[p.status] ?? "muted"}>{p.status.toLowerCase()}</Chip>
                </Cell>
                <Cell className="tnum whitespace-nowrap text-right font-semibold">
                  {money(p.amount, p.currency)}
                </Cell>
              </Row>
            ))}
          </Table>
        )
      ) : !m.payins.length ? (
        <Empty title="No pay-ins" body="Debt the driver pays from their phone appears here." />
      ) : (
        <Table head={["When", "For", "Status", "Amount"]}>
          {m.payins.map((p) => (
            <Row key={p.id}>
              <Cell className="tnum whitespace-nowrap text-muted">{when(p.created_at)}</Cell>
              <Cell>{p.purpose === "RIDE_FARE" ? "Ride fare" : "Debt"}</Cell>
              <Cell>
                <Chip tone={p.status === "COLLECTED" ? "ok" : p.status === "FAILED" ? "bad" : "warn"}>
                  {p.status.toLowerCase()}
                </Chip>
              </Cell>
              <Cell className="tnum whitespace-nowrap text-right font-semibold">
                {money(p.amount, p.currency)}
              </Cell>
            </Row>
          ))}
        </Table>
      )}
    </Card>
  );
}
