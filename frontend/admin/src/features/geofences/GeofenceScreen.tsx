"use client";
/** GeofenceScreen — one geofence: its shape on the map, redraw (a new version), rename, switch off, its profile and versions. */
import { ArrowLeft, PenLine } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { useMe } from "@/features/me";
import type { Schemas } from "@/lib/api";
import { when } from "@/lib/format";
import { type Draft, GeofenceMap } from "./GeofenceMap";
import { useGeofence, useReshape, useUpdateGeofence } from "./hooks";
import { areaText, KIND_LABEL, STATUS } from "./labels";
import { PolygonTools } from "./PolygonTools";
import { polygonFrom } from "./shapes";

type Ask = "reshape" | "status";

export function GeofenceScreen({ id }: { id: string }) {
  const { data: f, isLoading, error } = useGeofence(id);
  const reshape = useReshape();
  const update = useUpdateGeofence();
  const me = useMe();
  const editable = me.can("areas.edit");
  const [drawing, setDrawing] = useState(false);
  const [draft, setDraft] = useState<Draft>({ points: [], radiusM: 500 });
  const [ask, setAsk] = useState<Ask>();
  const ready = f?.kind === "PLACE" ? !!draft.center : draft.points.length >= 3;
  return (
    <div>
      <PageHeader
        title={f?.name ?? "Geofence"}
        subtitle={f ? `${KIND_LABEL[f.kind]} · ${f.code} · version ${f.version}` : undefined}
        actions={
          <Link
            href="/pricing/geofences"
            className="inline-flex items-center gap-1.5 text-sm font-semibold text-brand-text"
          >
            <ArrowLeft size={15} aria-hidden="true" /> All geofences
          </Link>
        }
      />
      <ErrorNote error={error ?? reshape.error ?? update.error} />
      {isLoading || !f ? (
        <Skeleton className="h-96" />
      ) : (
        <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
          <div className="grid gap-4">
            <Card index={1}>
              <div className="flex flex-wrap items-center gap-2 text-sm">
                <Chip tone={STATUS[f.status]?.tone}>{STATUS[f.status]?.label}</Chip>
                <span className="tnum text-muted">
                  {f.kind === "PLACE"
                    ? `${f.radius_m} m radius`
                    : `${areaText(f.area_m2)} · ${f.vertex_count} corners`}
                </span>
              </div>
              <p className="mt-3 text-sm">
                {f.profile ? (
                  <>
                    Profile:{" "}
                    <Link className="font-semibold text-brand-text" href={`/pricing/fees/${f.profile.id}`}>
                      {f.profile.name}
                    </Link>
                  </>
                ) : (
                  <span className="text-muted">
                    No pricing profile applied. Apply one from the profile's "Where it applies" tab.
                  </span>
                )}
              </p>
              {drawing && f.kind !== "PLACE" ? (
                <div className="mt-4">
                  <PolygonTools draft={draft} onDraft={setDraft} />
                </div>
              ) : null}
              {editable && f.status !== "ARCHIVED" ? (
                <div className="mt-4 flex flex-wrap gap-2">
                  {drawing ? (
                    <>
                      <Button disabled={!ready} onClick={() => setAsk("reshape")}>
                        Save new shape
                      </Button>
                      <Button
                        variant="ghost"
                        onClick={() => {
                          setDrawing(false);
                          setDraft({ points: [], radiusM: f.radius_m ?? 500 });
                        }}
                      >
                        Cancel
                      </Button>
                    </>
                  ) : (
                    <>
                      <Button
                        variant="soft"
                        icon={<PenLine size={15} aria-hidden="true" />}
                        onClick={() => {
                          setDrawing(true);
                          setDraft({ points: outline(f), radiusM: f.radius_m ?? 500 }); // start from the current shape
                        }}
                      >
                        Edit shape
                      </Button>
                      <Button
                        variant={f.status === "ACTIVE" ? "ghost" : "primary"}
                        onClick={() => setAsk("status")}
                      >
                        {f.status === "ACTIVE" ? "Switch off" : "Switch on"}
                      </Button>
                    </>
                  )}
                </div>
              ) : null}
              {drawing && f.kind === "PLACE" ? (
                <label className="mt-3 grid gap-1.5 text-sm">
                  <span className="flex justify-between font-medium">
                    Radius <span className="tnum text-muted">{`${draft.radiusM} m`}</span>
                  </span>
                  <input
                    type="range"
                    min={50}
                    max={5000}
                    step={50}
                    value={draft.radiusM}
                    onChange={(e) => setDraft({ ...draft, radiusM: Number(e.target.value) })}
                    className="accent-[var(--brand)]"
                  />
                </label>
              ) : null}
              {drawing ? (
                <p className="mt-2 text-xs text-muted">
                  {f.kind === "PLACE"
                    ? "Click the map to place the centre."
                    : "Click the map to add the corners of the new outline."}
                </p>
              ) : null}
            </Card>
            <Card index={2} title="Versions">
              <Table head={["Version", "Saved", "Reason"]}>
                {(f.versions ?? []).map((v) => (
                  <Row key={v.version}>
                    <Cell className="tnum font-semibold">{`v${v.version}`}</Cell>
                    <Cell className="tnum whitespace-nowrap text-muted">{when(v.created_at)}</Cell>
                    <Cell className="text-muted">{v.reason}</Cell>
                  </Row>
                ))}
              </Table>
            </Card>
          </div>
          <Card index={3}>
            <GeofenceMap
              fences={[f]}
              selectedId={f.id}
              mode={drawing ? (f.kind === "PLACE" ? "place" : "polygon") : "off"}
              draft={draft}
              onDraft={setDraft}
            />
          </Card>
        </div>
      )}
      <ReasonDialog
        open={!!ask}
        title={
          ask === "reshape"
            ? "Save the new shape?"
            : f?.status === "ACTIVE"
              ? "Switch this geofence off?"
              : "Switch this geofence on?"
        }
        body={
          ask === "reshape"
            ? "It becomes the next version. Trips priced before keep the version they matched."
            : f?.status === "ACTIVE"
              ? "It stops pricing new trips. Not allowed while a profile is applied to it."
              : "It starts matching trips again."
        }
        confirm={ask === "reshape" ? "Save" : f?.status === "ACTIVE" ? "Switch off" : "Switch on"}
        busy={reshape.isPending || update.isPending}
        onClose={() => setAsk(undefined)}
        onConfirm={(reason) => {
          if (!f) return;
          const done = {
            onSuccess: () => {
              setAsk(undefined);
              setDrawing(false);
            },
          };
          if (ask === "reshape")
            reshape.mutate(
              {
                id: f.id,
                reason,
                shape:
                  f.kind === "PLACE"
                    ? { center: draft.center, radius_m: draft.radiusM }
                    : { geometry: polygonFrom(draft.points) ?? undefined },
              },
              done,
            );
          else
            update.mutate({ id: f.id, status: f.status === "ACTIVE" ? "INACTIVE" : "ACTIVE", reason }, done);
        }}
      />
    </div>
  );
}

/** A polygon geofence's outer ring as editable corners (without the closing point); [] for places or several parts. */
function outline(f: Schemas["Geofence"]): [number, number][] {
  const polys = f.geometry?.coordinates ?? [];
  if (polys.length !== 1) return [];
  const ring = polys[0][0] ?? [];
  return ring.slice(0, -1).map((p) => [p[0], p[1]] as [number, number]);
}
