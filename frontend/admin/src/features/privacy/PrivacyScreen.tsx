"use client";
/**
 * PrivacyScreen — data-protection requests: people asking for a copy of their
 * data or for their account to be deleted. Open requests come first, by due
 * date; staff open one to approve or reject it. The Retention card below shows
 * what the daily clean-up removes.
 */
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Suspense } from "react";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Select } from "@/components/ui/Field";
import { FilterBar } from "@/components/ui/Operations";
import { PageHeader } from "@/components/ui/PageHeader";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { dateTime } from "@/lib/format";
import { useUrlState } from "@/lib/urlState";
import { DueChip } from "./DueChip";
import { OPEN_STATUSES, type PrivacyRequest, type RequestKind, useRequests } from "./hooks";
import { APP_LABEL, KIND_LABEL, STATUS_LABEL, STATUS_TONE } from "./labels";
import { personHref, personName } from "./links";
import { RetentionCard } from "./RetentionCard";

type Group = "open" | "closed" | "all";

const GROUPS: readonly { value: Group; label: string }[] = [
  { value: "open", label: "To handle" },
  { value: "closed", label: "Done" },
  { value: "all", label: "All" },
];

function inGroup(r: PrivacyRequest, group: Group): boolean {
  if (group === "all") return true;
  const open = OPEN_STATUSES.includes(r.status);
  return group === "open" ? open : !open;
}

export function PrivacyScreen() {
  return (
    <Suspense fallback={<Skeleton className="h-96" />}>
      <Queue />
    </Suspense>
  );
}

function Queue() {
  const url = useUrlState();
  const group = (url.get("show") ?? "open") as Group;
  const kind = (url.get("kind") ?? "") as RequestKind | "";
  const { data, isLoading, error } = useRequests(kind);
  const rows = data?.filter((r) => inGroup(r, group));
  const openCount = data?.filter((r) => inGroup(r, "open")).length;
  return (
    <div className="grid gap-4">
      <PageHeader
        title="Privacy"
        subtitle="People asking for a copy of their data or for their account to be deleted. Handle each one before its due date."
      />
      <Card index={1}>
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <FilterBar
            label="Show"
            value={group}
            options={GROUPS.map((g) => (g.value === "open" ? { ...g, count: openCount } : g))}
            onChange={(v) => url.set({ show: v === "open" ? undefined : v })}
          />
          <div className="w-full sm:w-56">
            <Select
              aria-label="Kind"
              value={kind}
              onChange={(e) => url.set({ kind: e.target.value || undefined })}
            >
              <option value="">Every kind</option>
              {Object.entries(KIND_LABEL).map(([k, v]) => (
                <option key={k} value={k}>
                  {v}
                </option>
              ))}
            </Select>
          </div>
        </div>
        {error ? <ErrorNote error={error} /> : null}
        {isLoading ? (
          <Skeleton className="h-64" />
        ) : !rows?.length ? (
          <Empty
            title={group === "open" ? "Nothing to handle" : "No requests here"}
            body={
              group === "open"
                ? "Requests appear here when someone asks from the app."
                : "Try another filter."
            }
          />
        ) : (
          <RequestsTable rows={rows} />
        )}
      </Card>
      <RetentionCard index={2} />
    </div>
  );
}

function RequestsTable({ rows }: { rows: PrivacyRequest[] }) {
  const router = useRouter();
  return (
    <Table head={["Person", "Request", "Asked", "Due", "Status"]}>
      {rows.map((r) => {
        const href = personHref(r);
        const name = personName(r);
        return (
          <Row key={r.id} onClick={() => router.push(`/compliance/privacy/${r.id}`)}>
            <Cell className="text-xs">
              {href ? (
                <Link
                  href={href}
                  onClick={(e) => e.stopPropagation()}
                  className="font-semibold hover:text-brand-text"
                >
                  {name}
                </Link>
              ) : (
                <span className="font-semibold">{name}</span>
              )}
              {APP_LABEL[r.app] ? (
                <span className="ml-1.5 text-[10px] text-muted">{APP_LABEL[r.app]}</span>
              ) : null}
            </Cell>
            <Cell>
              <Chip tone={r.kind === "DELETE" ? "bad" : "muted"}>{KIND_LABEL[r.kind]}</Chip>
            </Cell>
            <Cell className="tnum whitespace-nowrap text-[11px] text-muted">{dateTime(r.created_at)}</Cell>
            <Cell>
              <DueChip request={r} />
            </Cell>
            <Cell>
              <Chip tone={STATUS_TONE[r.status]}>{STATUS_LABEL[r.status]}</Chip>
            </Cell>
          </Row>
        );
      })}
    </Table>
  );
}
