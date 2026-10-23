"use client";
/** ComplaintsCard — low ratings about them, low ratings they gave, and SOS on their trips. */
import Link from "next/link";
import { useState } from "react";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Empty } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { Tabs } from "@/components/ui/Tabs";
import type { Schemas } from "@/lib/api";
import { when } from "@/lib/format";

type Tab = "about" | "by" | "sos";

export function ComplaintsCard({
  ratings,
  sos,
  index,
}: {
  ratings: Schemas["PersonRatings"];
  sos: Schemas["SosAlert"][];
  index: number;
}) {
  const [tab, setTab] = useState<Tab>("about");
  const options = [
    { value: "about" as const, label: `About them (${ratings.complaints_about.length})` },
    { value: "by" as const, label: `Made by them (${ratings.complaints_by.length})` },
    { value: "sos" as const, label: `SOS (${sos.length})` },
  ];
  return (
    <Card
      index={index}
      title="Complaints and safety"
      action={<Tabs id="complaints" value={tab} options={options} onChange={setTab} />}
    >
      {tab === "sos" ? (
        <SosTable sos={sos} />
      ) : (
        <ComplaintTable
          list={tab === "about" ? ratings.complaints_about : ratings.complaints_by}
          mine={tab === "by"}
        />
      )}
      <p className="mt-3 text-xs text-muted">
        Complaints are ratings of 2 stars or less, with what the person wrote.
      </p>
    </Card>
  );
}

function ComplaintTable({ list, mine }: { list: Schemas["RatingEntry"][]; mine: boolean }) {
  if (!list.length)
    return (
      <Empty
        title="No complaints"
        body={mine ? "They have not rated anyone 2 stars or less." : "Nobody rated them 2 stars or less."}
      />
    );
  return (
    <Table head={["When", "Trip", mine ? "About" : "From", "Stars", "What they said"]}>
      {list.map((r) => (
        <Row key={`${r.trip_id}${r.rater_role}`}>
          <Cell className="tnum whitespace-nowrap text-muted">{when(r.created_at)}</Cell>
          <Cell>
            <Link href={`/trips/${r.trip_id}`} className="font-mono text-xs font-semibold text-brand-text">
              {r.trip_code}
            </Link>
          </Cell>
          <Cell>{r.other_name ?? "Unknown"}</Cell>
          <Cell>
            <Chip tone="bad">{`${r.score} ★`}</Chip>
          </Cell>
          <Cell className="text-muted">
            {[r.comment, ...r.aspects].filter(Boolean).join(" · ") || "No comment"}
          </Cell>
        </Row>
      ))}
    </Table>
  );
}

function SosTable({ sos }: { sos: Schemas["SosAlert"][] }) {
  if (!sos.length) return <Empty title="No SOS" body="No alert was raised on their trips." />;
  return (
    <Table head={["When", "Trip", "Raised by", "Contacts told", "Status"]}>
      {sos.map((a) => (
        <Row key={a.id}>
          <Cell className="tnum whitespace-nowrap text-muted">{when(a.created_at)}</Cell>
          <Cell>
            <Link href={`/trips/${a.trip_id}`} className="font-mono text-xs font-semibold text-brand-text">
              {a.trip_code}
            </Link>
          </Cell>
          <Cell>{a.user_name ?? a.user_phone}</Cell>
          <Cell className="tnum">{a.contacts_alerted}</Cell>
          <Cell>
            {a.resolved_at ? (
              <Chip tone="muted">{a.resolution ? `Resolved: ${a.resolution}` : "Resolved"}</Chip>
            ) : (
              <Chip tone="bad">Open</Chip>
            )}
          </Cell>
        </Row>
      ))}
    </Table>
  );
}
