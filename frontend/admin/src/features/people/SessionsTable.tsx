"use client";
/** SessionsTable — a person's sign-in sessions on a map and in a table, each open one with End. */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { Empty } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { SignInMap } from "@/features/audit/SignInMap";
import { placeText } from "@/features/audit/signIns";
import type { Schemas } from "@/lib/api";
import { when } from "@/lib/format";
import { APP_LABEL } from "./labels";

/** Sessions listed before "Show all". */
const SHOWN = 8;

/** A session location with both coordinates, or nothing (the map needs both). */
function located(l?: Schemas["StaffSession"]["location"]) {
  return l && typeof l.lat === "number" && typeof l.lng === "number"
    ? { lat: l.lat, lng: l.lng, accuracy_m: l.accuracy_m }
    : undefined;
}

export function SessionsTable({
  sessions,
  onEnd,
}: {
  sessions: Schemas["StaffSession"][];
  onEnd?: (id: string) => void;
}) {
  const [selected, setSelected] = useState<number>();
  const [all, setAll] = useState(false);
  if (!sessions.length)
    return <Empty title="No sessions yet" body="Sessions appear when this person signs in." />;
  // The map reuses the sign-in view: one point per session that shared a location.
  const points = sessions.map((s, i) => ({
    id: i,
    who: s.device ?? "Session",
    when: when(s.started_at),
    at: s.started_at,
    failed: false,
    device: s.device ?? "Unknown device",
    ip: s.ip ?? "",
    userAgent: s.user_agent ?? "",
    location: located(s.location),
  }));
  return (
    <div className="grid gap-4">
      {points.some((x) => x.location) ? (
        <SignInMap signIns={points} selectedId={selected} onSelect={setSelected} />
      ) : null}
      <Table head={["Started", "Last active", "Device", "Network address", "Location", "Status", ""]}>
        {(all ? sessions : sessions.slice(0, SHOWN)).map((s, i) => (
          <Row key={s.id} onClick={() => setSelected(i)} selected={selected === i}>
            <Cell className="tnum whitespace-nowrap text-muted">{when(s.started_at)}</Cell>
            <Cell className="tnum whitespace-nowrap text-muted">{when(s.last_used_at)}</Cell>
            <Cell className="whitespace-nowrap">{s.device ?? APP_LABEL[s.app] ?? s.app}</Cell>
            <Cell className="font-mono text-xs text-muted">{s.ip ?? ""}</Cell>
            <Cell className="font-mono text-xs text-muted">
              {s.location ? placeText(located(s.location)) : "Not shared"}
            </Cell>
            <Cell>
              {s.active ? (
                <Chip tone="ok">Open</Chip>
              ) : (
                <Chip tone="muted">{s.end_reason ? "Ended" : "Expired"}</Chip>
              )}
            </Cell>
            <Cell className="text-right">
              {s.active && onEnd ? (
                <Button size="sm" variant="ghost" onClick={() => onEnd(s.id)}>
                  End
                </Button>
              ) : null}
            </Cell>
          </Row>
        ))}
      </Table>
      {sessions.length > SHOWN ? (
        <Button size="sm" variant="ghost" onClick={() => setAll(!all)} className="justify-self-start">
          {all ? "Show fewer" : `Show all ${sessions.length}`}
        </Button>
      ) : null}
    </div>
  );
}
