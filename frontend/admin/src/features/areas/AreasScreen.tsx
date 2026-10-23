"use client";
import { uncompactCells } from "h3-js";
/**
 * AreasScreen — the regions of the scoped country: pick one (or create one),
 * mark its cells on the map by clicking, dragging or drawing a shape, set its
 * status and surge cap, and save with a reason. Other areas' cells are locked.
 */
import { CheckCheck, Clock3, Hexagon, LocateFixed, MapPin, Pencil, Plus, Shapes, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { drawAreas } from "@/components/map/areaLayers";
import { DEFAULT_CENTRE, useMap } from "@/components/map/useMap";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Field, Input, Select, Toggle } from "@/components/ui/Field";
import { MetricTile, OperationsHeader } from "@/components/ui/Operations";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { useMe } from "@/features/me";
import { useAreas, useCountries, useScope } from "@/lib/scope";
import { useCreateArea, useSaveCells, useUpdateArea } from "./hooks";
import { type PaintMode, useCellPainter } from "./useCellPainter";

const AREA_MAP_ZOOM = 9;
const STATUS_TONE: Record<string, "ok" | "warn" | "muted"> = {
  ACTIVE: "ok",
  DRAFT: "warn",
  ARCHIVED: "muted",
};
const STATUS_LABEL: Record<string, string> = { ACTIVE: "Live", DRAFT: "Draft", ARCHIVED: "Archived" };

export function AreasScreen() {
  const { scope } = useScope();
  const me = useMe();
  const editable = me.can("areas.edit");
  const { data: countries } = useCountries();
  const country = countries?.find((c) => c.code === scope.country);
  const term = country?.area_term?.en ?? "Region";
  const { data: areas, isLoading, error } = useAreas(scope.country);
  const [selectedId, setSelectedId] = useState<string>();
  const selected = areas?.find((a) => a.id === selectedId);
  const centre: [number, number] = country ? [country.map.lng, country.map.lat] : [0, 0];
  // Created once; the effect below moves it to the country (re-creating it mid-load aborts tile requests).
  const { container, map } = useMap(AREA_MAP_ZOOM, DEFAULT_CENTRE, false);
  const [mode, setMode] = useState<PaintMode>("off");
  const initial = useMemo(() => new Set(selected ? uncompactCells(selected.cells, 7) : []), [selected]);
  const painter = useCellPainter(map, initial, editable ? mode : "off");
  const [creating, setCreating] = useState(false);
  const [asking, setAsking] = useState<"cells" | "status">();
  const [status, setStatus] = useState<string>();
  const save = useSaveCells();
  const update = useUpdateArea();

  // Selecting another area reloads the painter with its cells.
  useEffect(() => {
    painter.reset(initial);
    setMode("off");
  }, [initial]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!map || !areas) return;
    drawAreas(
      map,
      areas.filter((a) => a.id !== selectedId),
      selectedId,
    );
  }, [map, areas, selectedId]);

  useEffect(() => {
    if (!map || !country) return;
    map.jumpTo({ center: centre, zoom: country.map.zoom });
  }, [map, country?.code]); // eslint-disable-line react-hooks/exhaustive-deps

  const added = [...painter.cells].filter((c) => !initial.has(c)).length;
  const removed = [...initial].filter((c) => !painter.cells.has(c)).length;
  const dirty = added + removed > 0;
  function selectArea(id?: string) {
    setSelectedId(id);
    const area = areas?.find((item) => item.id === id);
    if (map && area?.cell_count) {
      const [west, south, east, north] = area.bbox;
      map.fitBounds(
        [
          [west, south],
          [east, north],
        ],
        { padding: 40, duration: 500 },
      );
    }
  }

  return (
    <div>
      <OperationsHeader
        title={`${term}s`}
        description={`Manage service boundaries in ${country?.name?.en ?? "the selected country"}. Regions determine where trips can be booked and which prices apply.`}
        actions={
          editable ? (
            <Button
              icon={<Plus size={15} aria-hidden="true" />}
              onClick={() => setCreating(true)}
            >{`New ${term.toLowerCase()}`}</Button>
          ) : null
        }
      />
      <div className="mb-5 grid grid-cols-2 gap-3 xl:grid-cols-4">
        <MetricTile
          label={`${term}s`}
          value={areas ? String(areas.length) : error ? "—" : undefined}
          note="Configured service boundaries"
          icon={<MapPin size={17} />}
          accent
        />
        <MetricTile
          label="Live regions"
          value={
            areas ? String(areas.filter((area) => area.status === "ACTIVE").length) : error ? "—" : undefined
          }
          note="Currently serving ride requests"
          icon={<CheckCheck size={17} />}
        />
        <MetricTile
          label="Draft regions"
          value={
            areas ? String(areas.filter((area) => area.status === "DRAFT").length) : error ? "—" : undefined
          }
          note="Boundaries being prepared"
          icon={<Clock3 size={17} />}
        />
        <MetricTile
          label="Boundary cells"
          value={
            areas
              ? areas.reduce((sum, area) => sum + area.cell_count, 0).toLocaleString("en-US")
              : error
                ? "—"
                : undefined
          }
          note="Assigned across configured regions"
          icon={<Hexagon size={17} />}
        />
      </div>
      <ErrorNote error={error ?? save.error ?? update.error} />
      <div className="grid items-start gap-5 xl:grid-cols-[300px_minmax(0,1fr)]">
        <Card
          index={1}
          title={`${term}s`}
          className="xl:sticky xl:top-24"
          action={<span className="text-[11px] text-muted">{country?.name?.en}</span>}
        >
          {isLoading ? (
            <Skeleton className="h-40" />
          ) : !areas?.length ? (
            <Empty
              title={`No ${term.toLowerCase()}s yet`}
              body={`Create one, then mark its cells on the map.`}
            />
          ) : (
            <ul className="grid max-h-96 gap-2 overflow-y-auto">
              {areas.map((a) => (
                <li key={a.id}>
                  <button
                    type="button"
                    onClick={() => selectArea(a.id === selectedId ? undefined : a.id)}
                    aria-pressed={a.id === selectedId}
                    className={`flex w-full items-center gap-3 rounded-lg border px-3 py-3 text-left transition-colors ${a.id === selectedId ? "border-brand/20 bg-brand-soft text-brand-text" : "border-line hover:bg-surface-2"}`}
                  >
                    <Hexagon size={16} aria-hidden="true" />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium">{a.name}</span>
                      <span
                        className={`block text-xs ${a.id === selectedId ? "opacity-80" : "text-muted"}`}
                      >{`${a.cell_count.toLocaleString("en-US")} cells`}</span>
                    </span>
                    <Chip tone={STATUS_TONE[a.status] ?? "muted"}>{STATUS_LABEL[a.status] ?? a.status}</Chip>
                  </button>
                </li>
              ))}
            </ul>
          )}
          <div className="mt-4 border-t border-line pt-4">
            <p className="text-xs font-medium">Boundary editing</p>
            <p className="mt-1.5 text-[11px] leading-relaxed text-muted">
              Select a region to see its boundary and settings. Changes to cells stay unpublished until you
              save them.
            </p>
          </div>
        </Card>
        <div className="grid gap-4">
          <Card
            index={2}
            title={selected ? selected.name : "Service coverage"}
            action={
              editable && selected ? (
                <div className="flex flex-wrap items-center gap-1.5">
                  <ModeButton
                    active={false}
                    onClick={() => selectArea(selected.id)}
                    icon={<LocateFixed size={14} aria-hidden="true" />}
                    label="Fit region"
                  />
                  <ModeButton
                    active={mode === "paint"}
                    onClick={() => setMode(mode === "paint" ? "off" : "paint")}
                    icon={<Pencil size={14} aria-hidden="true" />}
                    label="Mark cells"
                  />
                  <ModeButton
                    active={mode === "erase"}
                    onClick={() => setMode(mode === "erase" ? "off" : "erase")}
                    icon={<Trash2 size={14} aria-hidden="true" />}
                    label="Remove cells"
                  />
                  <ModeButton
                    active={mode === "polygon"}
                    onClick={() => setMode(mode === "polygon" ? "off" : "polygon")}
                    icon={<Shapes size={14} aria-hidden="true" />}
                    label="Draw a shape"
                  />
                  {mode === "polygon" && painter.ring.length >= 3 ? (
                    <>
                      <Button size="sm" variant="soft" onClick={() => painter.closePolygon(false)}>
                        Add shape
                      </Button>
                      <Button size="sm" variant="ghost" onClick={() => painter.closePolygon(true)}>
                        Remove shape
                      </Button>
                    </>
                  ) : null}
                </div>
              ) : null
            }
          >
            <div
              ref={container}
              className="h-[380px] w-full overflow-hidden rounded-lg bg-surface-2 sm:h-[560px]"
              role="region"
              aria-label={`Map of ${term.toLowerCase()}s`}
            />
            <div className="mt-3 flex flex-wrap items-center justify-between gap-3 text-sm">
              <span className="text-muted">
                {!selected
                  ? `Select a ${term.toLowerCase()} to edit its boundary. Zoom in to select individual cells.`
                  : dirty
                    ? `${added} added, ${removed} removed (unsaved)`
                    : `${painter.cells.size.toLocaleString("en-US")} cells`}
              </span>
              {selected && dirty ? (
                <div className="flex gap-2">
                  <Button variant="ghost" size="sm" onClick={() => painter.reset(initial)}>
                    Discard
                  </Button>
                  <Button size="sm" onClick={() => setAsking("cells")}>
                    Save cells
                  </Button>
                </div>
              ) : null}
            </div>
          </Card>
          {selected ? (
            <Card key={selected.id} index={3} title={`${selected.name} settings`}>
              <div className="grid gap-3 md:grid-cols-3">
                <Field label="Status" hint="Live areas serve rides; drafts can be prepared first">
                  <Select
                    value={selected.status}
                    disabled={!editable}
                    onChange={(e) => {
                      setStatus(e.target.value);
                      setAsking("status");
                    }}
                  >
                    <option value="DRAFT">Draft</option>
                    <option value="ACTIVE">Live</option>
                    <option value="ARCHIVED">Archived</option>
                  </Select>
                </Field>
                <Field label="Surge pricing">
                  <label className="flex h-10 items-center gap-2 text-sm">
                    <Toggle
                      label="Surge pricing"
                      checked={selected.surge_enabled}
                      onChange={(v) =>
                        editable &&
                        update.mutate({
                          id: selected.id,
                          body: {
                            surge_enabled: v,
                            reason: `Surge ${v ? "on" : "off"} for ${selected.name}`,
                          },
                        })
                      }
                    />
                    {selected.surge_enabled ? "On" : "Off"}
                  </label>
                </Field>
                <Field label="Surge cap (×)" hint="Highest multiplier here; empty keeps the ride type caps">
                  <Input
                    defaultValue={selected.surge_max ?? ""}
                    disabled={!editable}
                    onBlur={(e) => {
                      const v = e.target.value.trim();
                      if ((v || null) !== (selected.surge_max ?? null))
                        update.mutate({
                          id: selected.id,
                          body: {
                            surge_max: v || null,
                            reason: `Surge cap ${v || "removed"} for ${selected.name}`,
                          },
                        });
                    }}
                  />
                </Field>
              </div>
            </Card>
          ) : null}
        </div>
      </div>
      {creating ? (
        <CreateForm
          country={scope.country}
          term={term}
          onClose={() => setCreating(false)}
          onCreated={(id) => {
            setCreating(false);
            setSelectedId(id);
          }}
        />
      ) : null}
      <ReasonDialog
        open={asking === "cells"}
        title={`Save the cells of ${selected?.name ?? ""}?`}
        body="Quotes use the new boundary within seconds. Trips already booked keep their area."
        confirm="Save"
        busy={save.isPending}
        onClose={() => setAsking(undefined)}
        onConfirm={(reason) =>
          selected &&
          save.mutate(
            { id: selected.id, cells: [...painter.cells], reason },
            { onSuccess: () => setAsking(undefined) },
          )
        }
      />
      <ReasonDialog
        open={asking === "status"}
        title={`Set ${selected?.name ?? ""} to ${STATUS_LABEL[status ?? ""] ?? status}?`}
        body={
          status === "ACTIVE"
            ? "Riders in this area can book from now on. It needs cells, a fee policy and at least one price."
            : status === "ARCHIVED"
              ? "Its cells are freed for other areas; drivers must be moved first."
              : "Rides stop being quoted here while it is a draft."
        }
        confirm="Change status"
        danger={status === "ARCHIVED"}
        busy={update.isPending}
        onClose={() => setAsking(undefined)}
        onConfirm={(reason) =>
          selected &&
          status &&
          update.mutate(
            { id: selected.id, body: { status: status as "ACTIVE" | "DRAFT" | "ARCHIVED", reason } },
            { onSuccess: () => setAsking(undefined) },
          )
        }
      />
    </div>
  );
}

function ModeButton({
  active,
  onClick,
  icon,
  label,
}: {
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  label: string;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={`inline-flex items-center gap-1.5 rounded-[8px] px-3 py-1.5 text-xs font-semibold transition-colors ${active ? "bg-nav text-nav-ink" : "bg-surface-2 text-muted hover:text-ink"}`}
    >
      {icon}
      {label}
    </button>
  );
}

function CreateForm({
  country,
  term,
  onClose,
  onCreated,
}: {
  country: string;
  term: string;
  onClose: () => void;
  onCreated: (id: string) => void;
}) {
  const create = useCreateArea();
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const slug =
    code ||
    name
      .toLowerCase()
      .trim()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-|-$/g, "");
  return (
    <Card index={4} title={`New ${term.toLowerCase()}`}>
      <div className="grid gap-3 md:grid-cols-3">
        <Field label="Name">
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Arusha" />
        </Field>
        <Field label="Code" hint="Letters, digits and dashes">
          <Input value={slug} onChange={(e) => setCode(e.target.value)} />
        </Field>
        <div className="flex items-end gap-2">
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button
            disabled={!name.trim() || !/^[a-z][a-z0-9-]{1,31}$/.test(slug)}
            loading={create.isPending}
            onClick={() =>
              create.mutate(
                {
                  country,
                  code: slug,
                  name: name.trim(),
                  reason: `New ${term.toLowerCase()} ${name.trim()}`,
                },
                { onSuccess: (a) => onCreated((a as { id: string }).id) },
              )
            }
          >
            Create as draft
          </Button>
        </div>
      </div>
      <ErrorNote error={create.error} />
    </Card>
  );
}
