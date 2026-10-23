"use client";
/**
 * AuditScreen — two views of the append-only audit log: Sign-ins (a map of
 * where staff signed in from, with device, address and outcome; pick one for its
 * details) and All decisions (who did what to which record, and why).
 */
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { PageHeader } from "@/components/ui/PageHeader";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { Tabs } from "@/components/ui/Tabs";
import { api, unwrap } from "@/lib/api";
import { when } from "@/lib/format";
import { useUrlState } from "@/lib/urlState";
import { SignInMap } from "./SignInMap";
import { placeText, toSignIn } from "./signIns";

const AUDIT_LIMIT = 200;
const VIEWS = [
  { value: "signins", label: "Sign-ins" },
  { value: "all", label: "All decisions" },
];

/** Plain names for the actions that are not self-explanatory. */
const ACTION_LABEL: Record<string, string> = {
  "staff.login": "Signed in",
  "staff.login_failed": "Sign-in failed (wrong password)",
};

export function AuditScreen() {
  const url = useUrlState();
  const view = url.get("view") === "all" ? "all" : "signins";
  return (
    <div>
      <PageHeader
        title="Audit log"
        subtitle="Every sign-in and every staff decision, with where it came from. Entries can never be edited or deleted."
        actions={
          <Tabs
            id="audit-view"
            value={view}
            options={VIEWS}
            onChange={(v) => url.set({ view: v === "signins" ? undefined : v })}
          />
        }
      />
      {view === "signins" ? <SignIns /> : <Decisions />}
    </div>
  );
}

function SignIns() {
  const { data, isLoading, error } = useQuery({
    queryKey: ["admin", "audit", "signins"],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/audit", { params: { query: { action: "staff.login", limit: AUDIT_LIMIT } } }),
      ).data,
    refetchInterval: 30_000,
  });
  const [selectedId, setSelectedId] = useState<number>();
  const signIns = (data ?? []).map(toSignIn);
  const selected = signIns.find((s) => s.id === selectedId);
  const located = signIns.filter((s) => s.location).length;
  const failed = signIns.filter((s) => s.failed).length;
  return (
    <div className="grid gap-4">
      <ErrorNote error={error} />
      {isLoading ? (
        <Skeleton className="h-96" />
      ) : !signIns.length ? (
        <Card index={1}>
          <Empty
            title="No sign-ins recorded yet"
            body="Each staff sign-in appears here with its device, address and, when allowed, its location."
          />
        </Card>
      ) : (
        <>
          <div className="grid items-start gap-4 xl:grid-cols-[1fr_340px]">
            <Card
              index={1}
              title="Where staff signed in"
              action={
                <span className="tnum text-xs text-muted">{`${located} of ${signIns.length} with a location · ${failed} failed`}</span>
              }
            >
              {located ? (
                <SignInMap signIns={signIns} selectedId={selectedId} onSelect={setSelectedId} />
              ) : (
                <Empty
                  title="No locations yet"
                  body="Locations appear once staff allow location access when they sign in."
                />
              )}
            </Card>
            <Card index={2} title={selected ? "Sign-in details" : "Details"}>
              {selected ? (
                <dl className="grid gap-3 text-sm">
                  <Detail label="Who" value={selected.who} />
                  <Detail label="When" value={new Date(selected.at).toLocaleString("en-GB")} />
                  <Detail
                    label="Outcome"
                    value={selected.failed ? "Failed: wrong password" : "Signed in"}
                    tone={selected.failed ? "bad" : "ok"}
                  />
                  <Detail label="Device" value={selected.device} />
                  <Detail label="Network address" value={selected.ip || "Unknown"} mono />
                  <Detail label="Location" value={placeText(selected.location)} mono />
                  {selected.location ? (
                    <a
                      className="text-sm font-semibold text-brand-text"
                      href={`https://www.openstreetmap.org/?mlat=${selected.location.lat}&mlon=${selected.location.lng}#map=16/${selected.location.lat}/${selected.location.lng}`}
                      target="_blank"
                      rel="noreferrer"
                    >
                      Open in a full map
                    </a>
                  ) : null}
                  <Detail label="Browser details" value={selected.userAgent || "Unknown"} mono small />
                </dl>
              ) : (
                <p className="text-sm text-muted">
                  Pick a sign-in on the map or in the table to see its device, address and location.
                </p>
              )}
            </Card>
          </div>
          <Card index={3} title="Recent sign-ins">
            <Table head={["When", "Who", "Outcome", "Device", "Network address", "Location"]}>
              {signIns.map((s) => (
                <Row key={s.id} onClick={() => setSelectedId(s.id)} selected={s.id === selectedId}>
                  <Cell className="tnum whitespace-nowrap text-muted">{s.when}</Cell>
                  <Cell className="font-medium">{s.who}</Cell>
                  <Cell>{s.failed ? <Chip tone="bad">Failed</Chip> : <Chip tone="ok">Signed in</Chip>}</Cell>
                  <Cell className="whitespace-nowrap">{s.device}</Cell>
                  <Cell className="tnum font-mono text-xs text-muted">{s.ip}</Cell>
                  <Cell className="tnum font-mono text-xs text-muted">
                    {s.location ? placeText(s.location) : "Not shared"}
                  </Cell>
                </Row>
              ))}
            </Table>
          </Card>
        </>
      )}
    </div>
  );
}

function Detail({
  label,
  value,
  mono,
  small,
  tone,
}: {
  label: string;
  value: string;
  mono?: boolean;
  small?: boolean;
  tone?: "ok" | "bad";
}) {
  return (
    <div>
      <dt className="text-xs text-muted">{label}</dt>
      <dd
        className={`${mono ? "font-mono" : ""} ${small ? "break-all text-xs" : ""} ${tone === "bad" ? "text-critical" : ""}`}
      >
        {value}
      </dd>
    </div>
  );
}

function Decisions() {
  const { data, isLoading, error } = useQuery({
    queryKey: ["admin", "audit"],
    queryFn: async () =>
      unwrap(await api.GET("/admin/audit", { params: { query: { limit: AUDIT_LIMIT } } })).data,
  });
  return (
    <div>
      <ErrorNote error={error} />
      <Card index={1}>
        {isLoading ? (
          <Skeleton className="h-64" />
        ) : !data?.length ? (
          <Empty title="Nothing recorded yet" />
        ) : (
          <Table head={["When", "Who", "What", "Record", "Where", "Reason"]}>
            {data.map((e) => (
              <Row key={e.id}>
                <Cell className="tnum whitespace-nowrap text-muted">{when(e.at)}</Cell>
                <Cell className="font-medium">{e.actor ?? "System"}</Cell>
                <Cell>{ACTION_LABEL[e.action] ?? e.action.replace(/[._]/g, " ")}</Cell>
                <Cell className="max-w-[220px] truncate font-mono text-xs text-muted">{e.target}</Cell>
                <Cell className="whitespace-nowrap text-xs text-muted">{whereFrom(e.data)}</Cell>
                <Cell className="text-muted">{e.reason ?? ""}</Cell>
              </Row>
            ))}
          </Table>
        )}
      </Card>
    </div>
  );
}

/** Device and network address of an entry, when it carries them. */
function whereFrom(data: Record<string, unknown> | undefined): string {
  const device = typeof data?.device === "string" ? data.device : "";
  const ip = typeof data?.ip === "string" ? data.ip : "";
  return [device, ip].filter(Boolean).join(" · ");
}
