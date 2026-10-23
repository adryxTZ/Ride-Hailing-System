"use client";
/** ScopesCard — where a profile applies now, every place it applied before, and assigning it to a region. */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { Field, Input, Select } from "@/components/ui/Field";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Empty, ErrorNote } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { useGeofences } from "@/features/geofences/hooks";
import type { Schemas } from "@/lib/api";
import { when } from "@/lib/format";
import { useAreas } from "@/lib/scope";
import { type ScopeKind, useAssign, useUnassign } from "./hooks";

const KIND_LABEL: Record<string, string> = {
  COUNTRY: "Whole country (default)",
  AREA: "Region",
  DISTRICT: "District",
  ZONE: "Zone",
  PLACE: "Place",
};

export function ScopesCard({ profile, editable }: { profile: Schemas["PricingProfile"]; editable: boolean }) {
  const { data: areas } = useAreas(profile.country);
  const { data: fences } = useGeofences(profile.country);
  const assign = useAssign();
  const unassign = useUnassign();
  const [kind, setKind] = useState<ScopeKind>("AREA");
  const [areaId, setAreaId] = useState("");
  const choices =
    kind === "AREA"
      ? (areas ?? []).map((a) => ({ id: a.id, name: a.name }))
      : (fences ?? [])
          .filter((f) => f.kind === kind && f.status === "ACTIVE")
          .map((f) => ({ id: f.id, name: f.name }));
  const [priority, setPriority] = useState("0");
  const [ask, setAsk] = useState<
    { kind: "assign" } | { kind: "remove"; assignmentId: string; name: string }
  >();
  const history = profile.assignments ?? [];
  return (
    <div className="grid gap-4">
      <ErrorNote error={assign.error ?? unassign.error} />
      {editable && !profile.is_default ? (
        <div className="grid items-end gap-3 md:grid-cols-[160px_1fr_120px_auto]">
          <Field label="Apply to a">
            <Select
              value={kind}
              onChange={(e) => {
                setKind(e.target.value as ScopeKind);
                setAreaId("");
              }}
            >
              <option value="AREA">Region</option>
              <option value="DISTRICT">District</option>
              <option value="ZONE">Zone</option>
              <option value="PLACE">Place</option>
            </Select>
          </Field>
          <Field
            label="Which"
            hint={kind === "AREA" ? undefined : "Draw or import them on the Geofences page"}
          >
            <Select value={areaId} onChange={(e) => setAreaId(e.target.value)}>
              <option value="">Choose…</option>
              {choices.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Priority" hint="Higher wins">
            <Input
              inputMode="numeric"
              value={priority}
              onChange={(e) => setPriority(e.target.value.replace(/[^\d-]/g, ""))}
            />
          </Field>
          <Button disabled={!areaId} onClick={() => setAsk({ kind: "assign" })}>
            Apply
          </Button>
        </div>
      ) : null}
      {profile.is_default ? (
        <p className="text-sm text-muted">
          This is the country default: it prices every trip a region, district, zone or place profile does
          not.
        </p>
      ) : null}
      {history.length === 0 ? (
        <Empty
          title="Not applied anywhere"
          body="Apply it to a region; it prices trips picked up there while it is on."
        />
      ) : (
        <Table head={["Where", "Kind", "Priority", "From", "Until", "Reason", ""]}>
          {history.map((s) => (
            <Row key={s.assignment_id}>
              <Cell className="font-medium">{s.name}</Cell>
              <Cell className="text-muted">{KIND_LABEL[s.kind] ?? s.kind}</Cell>
              <Cell className="tnum">{s.priority}</Cell>
              <Cell className="tnum whitespace-nowrap text-muted">{when(s.valid_from)}</Cell>
              <Cell className="tnum whitespace-nowrap">
                {s.valid_to ? (
                  <span className="text-muted">{when(s.valid_to)}</span>
                ) : (
                  <Chip tone="ok">Now</Chip>
                )}
              </Cell>
              <Cell className="max-w-[220px] truncate text-muted">{s.reason}</Cell>
              <Cell className="text-right">
                {editable && !s.valid_to && s.kind !== "COUNTRY" ? (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setAsk({ kind: "remove", assignmentId: s.assignment_id, name: s.name })}
                  >
                    Remove
                  </Button>
                ) : null}
              </Cell>
            </Row>
          ))}
        </Table>
      )}
      <ReasonDialog
        open={!!ask}
        title={
          ask?.kind === "remove"
            ? `Stop applying this profile in ${ask.name}?`
            : "Apply this profile to the region?"
        }
        body={
          ask?.kind === "remove"
            ? "New trips there use the next profile that covers them. Trips already priced keep their record."
            : "New trips picked up there are priced by this profile while it is on."
        }
        confirm={ask?.kind === "remove" ? "Remove" : "Apply"}
        busy={assign.isPending || unassign.isPending}
        onClose={() => setAsk(undefined)}
        onConfirm={(reason) => {
          const done = { onSuccess: () => setAsk(undefined) };
          if (ask?.kind === "remove")
            unassign.mutate({ id: profile.id, assignmentId: ask.assignmentId, reason }, done);
          else
            assign.mutate(
              { id: profile.id, scope_kind: kind, scope_id: areaId, priority: Number(priority || 0), reason },
              done,
            );
        }}
      />
    </div>
  );
}
