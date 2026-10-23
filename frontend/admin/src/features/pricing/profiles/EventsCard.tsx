"use client";
/** EventsCard — the profile's history: who changed what, when and why. */
import { Empty } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import type { Schemas } from "@/lib/api";
import { when } from "@/lib/format";
import { EVENT_LABEL } from "./model";

export function EventsCard({ events }: { events: Schemas["ProfileEvent"][] }) {
  if (!events.length) return <Empty title="No history yet" />;
  return (
    <Table head={["When", "What", "Version", "By", "Reason"]}>
      {events.map((e, i) => (
        <Row key={`${e.at}${i}`}>
          <Cell className="tnum whitespace-nowrap text-muted">{when(e.at)}</Cell>
          <Cell>{EVENT_LABEL[e.kind] ?? e.kind}</Cell>
          <Cell className="tnum">{e.version ? `v${e.version}` : ""}</Cell>
          <Cell>{e.actor_name ?? (e.actor_id ? "Staff" : "System")}</Cell>
          <Cell className="max-w-[280px] truncate text-muted">{e.reason ?? ""}</Cell>
        </Row>
      ))}
    </Table>
  );
}
