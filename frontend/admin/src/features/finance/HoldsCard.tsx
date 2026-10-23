"use client";
/** HoldsCard — drivers who cannot go online. Staff holds can be released here; debt holds lift when paid. */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { money, when } from "@/lib/format";
import { useHoldAction, useHolds } from "./hooks";
import { HOLD_CAUSE } from "./labels";

export function HoldsCard({
  index,
  currency,
  editable,
}: {
  index: number;
  currency: string;
  editable: boolean;
}) {
  const { data, isLoading, error } = useHolds();
  const act = useHoldAction();
  const [target, setTarget] = useState<{ driverId: string; name: string }>();
  return (
    <Card
      index={index}
      title="Holds"
      action={<span className="tnum text-xs text-muted">{`${data?.data.length ?? 0} on hold`}</span>}
    >
      <ErrorNote error={error ?? act.error} />
      {isLoading ? (
        <Skeleton className="h-40" />
      ) : !data?.data.length ? (
        <Empty
          title="Nobody is on hold"
          body="Drivers appear here when their debt passes the limit or staff place a hold."
        />
      ) : (
        <Table head={["Driver", "Why", "Owes", "Since", ""]}>
          {data.data.map((h) => (
            <Row key={h.hold.id}>
              <Cell>
                <div className="font-medium">{h.name ?? "No name"}</div>
                <div className="tnum text-xs text-muted">{h.phone}</div>
              </Cell>
              <Cell>
                <Chip tone={h.hold.kind === "DEBT" ? "warn" : "bad"}>
                  {HOLD_CAUSE[h.hold.cause] ?? h.hold.cause}
                </Chip>
                {h.hold.reason ? <div className="mt-1 text-xs text-muted">{h.hold.reason}</div> : null}
              </Cell>
              <Cell className="tnum">{money(h.debt, currency)}</Cell>
              <Cell className="tnum text-muted">{when(h.hold.since)}</Cell>
              <Cell className="text-right">
                {h.hold.kind === "DEBT" ? (
                  <span className="whitespace-nowrap text-xs text-muted">Until paid</span>
                ) : editable ? (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setTarget({ driverId: h.driver_id, name: h.name ?? h.phone })}
                  >
                    Release
                  </Button>
                ) : (
                  <span className="text-xs text-muted">Released by staff</span>
                )}
              </Cell>
            </Row>
          ))}
        </Table>
      )}
      <ReasonDialog
        open={!!target}
        title={`Release the hold on ${target?.name ?? ""}?`}
        body="They can go online again at once. A debt hold, if any, stays until paid."
        confirm="Release"
        busy={act.isPending}
        onClose={() => setTarget(undefined)}
        onConfirm={(reason) =>
          target &&
          act.mutate(
            { driverId: target.driverId, release: true, reason },
            { onSuccess: () => setTarget(undefined) },
          )
        }
      />
    </Card>
  );
}
