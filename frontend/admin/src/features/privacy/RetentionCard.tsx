"use client";
/**
 * RetentionCard — what the daily retention job removes, after how long, and
 * how its last run went. Periods come from the brand file; read-only here.
 */
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { count, dateTime } from "@/lib/format";
import { useRetention } from "./hooks";
import { RETENTION_LABEL } from "./labels";

/** "1 day", "30 days", "2 years". */
function period(days: number): string {
  const YEAR = 365;
  if (days >= YEAR && days % YEAR === 0) return days === YEAR ? "1 year" : `${days / YEAR} years`;
  return days === 1 ? "1 day" : `${days} days`;
}

export function RetentionCard({ index = 2 }: { index?: number }) {
  const { data, isLoading, error } = useRetention();
  return (
    <Card title="Retention" index={index}>
      <p className="mb-4 text-xs leading-relaxed text-muted">
        Once a day the Platform removes data older than its keep period. Trip and payment records are kept as
        the law requires.
      </p>
      {error ? <ErrorNote error={error} /> : null}
      {isLoading ? (
        <Skeleton className="h-48" />
      ) : !data?.length ? (
        <Empty title="No retention rules" />
      ) : (
        <Table head={["Data", "Kept for", "Last run", "Removed", "Result"]}>
          {data.map((r) => (
            <Row key={r.name}>
              <Cell className="text-xs font-semibold">{RETENTION_LABEL[r.name] ?? r.name}</Cell>
              <Cell className="tnum text-xs">{period(r.days)}</Cell>
              <Cell className="tnum whitespace-nowrap text-[11px] text-muted">
                {r.last_run_at ? dateTime(r.last_run_at) : "Not run yet"}
              </Cell>
              <Cell className="tnum text-xs">{count(r.rows_last_run)}</Cell>
              <Cell>
                {r.error ? (
                  <span title={r.error}>
                    <Chip tone="bad">Failed</Chip>
                  </span>
                ) : r.last_run_at ? (
                  <Chip tone="ok">OK</Chip>
                ) : (
                  <Chip tone="muted">Waiting</Chip>
                )}
              </Cell>
            </Row>
          ))}
        </Table>
      )}
    </Card>
  );
}
