"use client";
/**
 * DocumentsScreen — driver documents of the selected country: what is asked
 * for (mandatory, optional or off, and expiry rules), and approved drivers'
 * documents that are expiring, expired, missing or waiting with a renewal.
 */
import Link from "next/link";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { PageHeader } from "@/components/ui/PageHeader";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { Tabs } from "@/components/ui/Tabs";
import { useMe } from "@/features/me";
import { useUrlState } from "@/lib/urlState";
import { DocumentTypesCard } from "./DocumentTypesCard";
import { useAdminDocuments } from "./hooks";

const VIEWS = [
  { value: "rules", label: "Requirements" },
  { value: "EXPIRING", label: "Expiring" },
  { value: "EXPIRED", label: "Expired" },
  { value: "MISSING", label: "Missing" },
  { value: "RENEWAL_PENDING", label: "Renewals" },
  { value: "UNKNOWN", label: "No date" },
];

type State = "EXPIRING" | "EXPIRED" | "UNKNOWN" | "RENEWAL_PENDING" | "MISSING";

export function DocumentsScreen() {
  const url = useUrlState();
  const view = url.get("view") ?? "rules";
  const editable = useMe().can("compliance.manage");
  return (
    <div>
      <PageHeader
        title="Documents"
        subtitle="Which documents drivers must provide in this country, and whose are about to expire."
      />
      <div className="mb-4">
        <Tabs
          id="documents-view"
          value={view}
          options={VIEWS}
          onChange={(v) => url.set({ view: v === "rules" ? undefined : v })}
        />
      </div>
      {view === "rules" ? (
        <DocumentTypesCard index={1} editable={editable} />
      ) : (
        <DocumentList state={view as State} />
      )}
    </div>
  );
}

function DocumentList({ state }: { state: State }) {
  const { data, isLoading, error } = useAdminDocuments(state);
  return (
    <Card index={1}>
      <ErrorNote error={error} />
      {isLoading ? (
        <Skeleton className="h-48" />
      ) : !data?.length ? (
        <Empty
          title="Nothing here"
          body="Approved drivers' documents appear here when they need attention."
        />
      ) : (
        <Table head={["Driver", "Document", "Expires", "State", ""]}>
          {data.map((d) => (
            <Row key={`${d.driver_id}-${d.kind}`}>
              <Cell>
                <div className="font-medium">{d.driver_name ?? "No name"}</div>
                <div className="tnum text-xs text-muted">
                  {[d.phone, d.plate].filter(Boolean).join(" · ")}
                </div>
              </Cell>
              <Cell>
                <div>{d.label.en}</div>
                <div className="text-xs text-muted">
                  {d.mandatory ? "Mandatory" : "Optional"}
                  {d.blocks_when_expired ? " · holds when expired" : ""}
                </div>
              </Cell>
              <Cell className="tnum whitespace-nowrap">{d.expires_on ?? ""}</Cell>
              <Cell>
                <StateChip doc={d} />
              </Cell>
              <Cell className="text-right">
                <Link href={`/drivers/${d.driver_id}`} className="text-sm font-semibold text-brand-text">
                  Open driver
                </Link>
              </Cell>
            </Row>
          ))}
        </Table>
      )}
    </Card>
  );
}

function StateChip({ doc }: { doc: { expiry_state: string; days_left?: number; renewal_id?: string } }) {
  if (doc.renewal_id) return <Chip tone="live">Renewal to review</Chip>;
  switch (doc.expiry_state) {
    case "EXPIRED":
      return <Chip tone="bad">{`Expired ${Math.abs(doc.days_left ?? 0)} days ago`}</Chip>;
    case "EXPIRING":
      return <Chip tone="warn">{doc.days_left === 0 ? "Expires today" : `${doc.days_left} days left`}</Chip>;
    case "MISSING":
      return <Chip tone="bad">Not provided</Chip>;
    case "UNKNOWN":
      return <Chip tone="muted">No expiry date</Chip>;
  }
  return <Chip tone="ok">Valid</Chip>;
}
