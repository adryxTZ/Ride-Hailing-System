"use client";
/** AccountCard — sessions and devices, plus what staff did to this person. */
import { useState } from "react";
import { Card } from "@/components/ui/Card";
import { Empty, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { Tabs } from "@/components/ui/Tabs";
import type { Schemas } from "@/lib/api";
import { when } from "@/lib/format";
import { usePersonLog } from "./hooks";
import { APP_LABEL } from "./labels";
import { SessionsTable } from "./SessionsTable";

type Tab = "sessions" | "devices" | "log";

export function AccountCard({
  account,
  index,
  onEnd,
}: {
  account: Schemas["PersonAccount"];
  index: number;
  onEnd?: (sessionId: string) => void;
}) {
  const [tab, setTab] = useState<Tab>("sessions");
  const options = [
    { value: "sessions" as const, label: "Sessions" },
    { value: "devices" as const, label: `Devices (${account.devices.length})` },
    { value: "log" as const, label: "Staff actions" },
  ];
  return (
    <Card
      index={index}
      title="Account"
      action={<Tabs id="account" value={tab} options={options} onChange={setTab} />}
    >
      {tab === "sessions" ? (
        <SessionsTable sessions={account.sessions} onEnd={onEnd} />
      ) : tab === "devices" ? (
        <Devices devices={account.devices} />
      ) : (
        <StaffLog id={account.id} />
      )}
    </Card>
  );
}

function Devices({ devices }: { devices: Schemas["PersonDevice"][] }) {
  if (!devices.length)
    return <Empty title="No devices" body="A phone appears once the app registers for notifications." />;
  return (
    <Table head={["App", "Phone", "Version", "Last seen"]}>
      {devices.map((d, i) => (
        <Row key={`${d.platform}${i}`}>
          <Cell>{APP_LABEL[d.app] ?? d.app}</Cell>
          <Cell>{d.platform === "IOS" ? "iPhone" : d.platform === "ANDROID" ? "Android" : "Web"}</Cell>
          <Cell className="font-mono text-xs text-muted">{d.app_version ?? "Unknown"}</Cell>
          <Cell className="tnum text-muted">{when(d.last_seen_at)}</Cell>
        </Row>
      ))}
    </Table>
  );
}

function StaffLog({ id }: { id: string }) {
  const { data, isLoading } = usePersonLog(id);
  if (isLoading) return <Skeleton className="h-40" />;
  if (!data?.length)
    return <Empty title="Nothing yet" body="Blocks, ended sessions and notes appear here." />;
  return (
    <Table head={["When", "What", "By", "Reason"]}>
      {data.map((e) => (
        <Row key={e.id}>
          <Cell className="tnum whitespace-nowrap text-muted">{when(e.at)}</Cell>
          <Cell>{e.action.replace(/[._]/g, " ")}</Cell>
          <Cell>{e.actor ?? "System"}</Cell>
          <Cell className="text-muted">{e.reason ?? ""}</Cell>
        </Row>
      ))}
    </Table>
  );
}
