"use client";
/**
 * PayinsCard — recent mobile-money payments with their outcome: drivers paying
 * debt and riders paying fares, filtered by status and kind.
 */
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { Tabs } from "@/components/ui/Tabs";
import { money, when } from "@/lib/format";
import { useUrlState } from "@/lib/urlState";
import { usePayins } from "./hooks";
import { FAILURE_LABEL } from "./labels";

const PURPOSES = [
  { value: "", label: "All kinds" },
  { value: "DRIVER_DEBT", label: "Driver debt" },
  { value: "RIDE_FARE", label: "Trip fares" },
];

const STATUSES = [
  { value: "", label: "All" },
  { value: "PENDING", label: "Waiting" },
  { value: "COLLECTED", label: "Received" },
  { value: "FAILED", label: "Failed" },
];

export function PayinsCard({ index }: { index: number }) {
  const url = useUrlState();
  const status = url.get("payins") ?? "";
  const purpose = url.get("payin_kind") ?? "";
  const { data, isLoading, error } = usePayins(status, purpose);
  return (
    <Card
      index={index}
      title="Mobile money in"
      action={
        <Tabs
          id="payins"
          value={status}
          options={STATUSES}
          onChange={(v) => url.set({ payins: v || undefined })}
        />
      }
    >
      <div className="mb-3">
        <Tabs
          id="payin-kind"
          value={purpose}
          options={PURPOSES}
          onChange={(v) => url.set({ payin_kind: v || undefined })}
        />
      </div>
      <ErrorNote error={error} />
      {isLoading ? (
        <Skeleton className="h-40" />
      ) : !data?.data.length ? (
        <Empty
          title="No payments yet"
          body="Drivers paying debt and riders paying fares by mobile money appear here within seconds."
        />
      ) : (
        <Table head={["When", "Payer", "Amount", "Outcome", "Reference"]}>
          {data.data.map((p) => (
            <Row key={p.id}>
              <Cell className="tnum whitespace-nowrap text-muted">{when(p.created_at)}</Cell>
              <Cell>
                <div className="font-medium">{p.name ?? "No name"}</div>
                <div className="tnum text-xs text-muted">
                  {`${p.purpose === "RIDE_FARE" ? "Rider · trip fare" : "Driver · debt"} · ${p.phone}`}
                </div>
              </Cell>
              <Cell className="tnum whitespace-nowrap">{money(p.amount, p.currency)}</Cell>
              <Cell>
                {p.status === "COLLECTED" ? (
                  <Chip tone="ok">Received</Chip>
                ) : p.status === "PENDING" ? (
                  <Chip tone="live">Waiting for PIN</Chip>
                ) : (
                  <Chip tone="bad">
                    {p.purpose === "RIDE_FARE" && p.failure_code === "REJECTED_BY_USER"
                      ? "Declined by the rider"
                      : (FAILURE_LABEL[p.failure_code ?? ""] ?? "Failed")}
                  </Chip>
                )}
              </Cell>
              <Cell className="max-w-[12rem] font-mono text-xs text-muted">
                <span className="block truncate" title={p.provider_ref ?? ""}>
                  {p.provider_ref ?? p.gateway}
                </span>
              </Cell>
            </Row>
          ))}
        </Table>
      )}
    </Card>
  );
}
