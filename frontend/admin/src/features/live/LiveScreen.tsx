"use client";

import { cellToBoundary, cellToLatLng } from "h3-js";
import {
  ArrowUpRight,
  CarFront,
  CheckCheck,
  ChevronRight,
  Clock3,
  Flame,
  Layers2,
  LocateFixed,
  MapPin,
  Route,
  Users,
} from "lucide-react";
import maplibregl from "maplibre-gl";
import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";
import { drawAreas } from "@/components/map/areaLayers";
import { useHexGrid } from "@/components/map/useHexGrid";
import { dot, useMap } from "@/components/map/useMap";
import { Chip } from "@/components/ui/Chip";
import { Toggle } from "@/components/ui/Field";
import {
  FilterBar,
  Initials,
  MetricTile,
  OperationsHeader,
  OperationsPanel,
  RefreshButton,
  UpdateIndicator,
} from "@/components/ui/Operations";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import type { Schemas } from "@/lib/api";
import { useClassName } from "@/lib/catalog";
import { count, duration } from "@/lib/format";
import { useAreas, useCountries, useScope } from "@/lib/scope";
import { useLive } from "./hooks";

const STATE_COLOUR: Record<string, string> = { IDLE: "--good", OFFERED: "--warn", ON_TRIP: "--chart-1" };
const STATE_LABEL: Record<string, string> = {
  IDLE: "Available",
  OFFERED: "Offer pending",
  ON_TRIP: "On trip",
};
const DRIVER_FILTERS = [
  { value: "ALL", label: "All drivers" },
  { value: "IDLE", label: "Available" },
  { value: "OFFERED", label: "Offered" },
  { value: "ON_TRIP", label: "On trip" },
] as const;
type DriverFilter = (typeof DRIVER_FILTERS)[number]["value"];
const EMPTY_DRIVERS: Schemas["LiveDriver"][] = [];
const EMPTY_REQUESTS: Schemas["WaitingRequest"][] = [];
const EMPTY_SURGE: Record<string, string> = {};

export function LiveScreen() {
  const live = useLive();
  const { data, error, isLoading, isFetching, dataUpdatedAt } = live;
  const { container, map } = useMap(12);
  const popup = useRef<maplibregl.Popup | null>(null);
  const className = useClassName();
  const drivers = data?.drivers ?? EMPTY_DRIVERS;
  const requests = data?.requests ?? EMPTY_REQUESTS;
  const surge = data?.surge ?? EMPTY_SURGE;
  const [grid, setGrid] = useState(false);
  const [coverage, setCoverage] = useState(false);
  const [driverFilter, setDriverFilter] = useState<DriverFilter>("ALL");
  const [activity, setActivity] = useState<"drivers" | "waiting">("drivers");
  const [selected, setSelected] = useState<string>();
  const { scope } = useScope();
  const { data: areas } = useAreas(scope.country);
  const { data: countries } = useCountries();
  const country = countries?.find((item) => item.code === scope.country);
  const area = areas?.find((item) => item.id === scope.areaId);
  useEffect(() => {
    popup.current?.remove();
    setSelected(undefined);
  }, [driverFilter, scope.country, scope.areaId]);
  const visible = useMemo(
    () => drivers.filter((driver) => driverFilter === "ALL" || driver.state === driverFilter),
    [drivers, driverFilter],
  );
  const waiting = useMemo(() => [...requests].sort((a, b) => b.waiting_s - a.waiting_s), [requests]);
  const gridInput = useMemo(
    () => ({ drivers: drivers.filter((driver) => driver.state === "IDLE"), requests }),
    [drivers, requests],
  );
  useHexGrid(map, gridInput, grid);
  const byState = useMemo(
    () =>
      drivers.reduce<Record<string, number>>((totals, driver) => {
        totals[driver.state] = (totals[driver.state] ?? 0) + 1;
        return totals;
      }, {}),
    [drivers],
  );

  useEffect(() => {
    if (!map || !areas) return;
    drawAreas(map, coverage ? areas : [], scope.areaId);
  }, [map, areas, coverage, scope.areaId]);

  function driverPopup(driver: Schemas["LiveDriver"]) {
    if (!map) return;
    setSelected(driver.driver_id);
    map.flyTo({ center: [driver.location.lng, driver.location.lat], zoom: 14, duration: 600 });
    popup.current?.remove();
    const content = document.createElement("div");
    content.style.cssText = "padding:4px;font-family:var(--font-sans);color:var(--ink);min-width:140px";
    const name = document.createElement("p");
    name.textContent = driver.name || "Driver";
    name.style.cssText = "font-weight:600;font-size:13px;margin:0 0 6px";
    const status = document.createElement("p");
    status.textContent = STATE_LABEL[driver.state] ?? driver.state;
    status.style.cssText = "font-size:11px;color:var(--muted);margin:0 0 10px";
    const link = document.createElement("a");
    link.href = `/drivers/${encodeURIComponent(driver.driver_id)}`;
    link.textContent = "Open driver profile →";
    link.style.cssText = "font-size:11px;font-weight:600;color:var(--brand-text)";
    content.append(name, status, link);
    popup.current = new maplibregl.Popup({ offset: 16, closeButton: false, anchor: "bottom" })
      .setLngLat([driver.location.lng, driver.location.lat])
      .setDOMContent(content)
      .addTo(map);
    const frame = popup.current.getElement().querySelector<HTMLElement>(".maplibregl-popup-content");
    const tip = popup.current.getElement().querySelector<HTMLElement>(".maplibregl-popup-tip");
    if (frame) {
      frame.style.background = "var(--surface)";
      frame.style.borderRadius = "10px";
    }
    if (tip) tip.style.borderTopColor = "var(--surface)";
  }

  useEffect(() => {
    if (!map) return;
    const markers = [
      ...visible.map((driver) => {
        const element = dot(
          STATE_COLOUR[driver.state] ?? "--muted",
          20,
          `${driver.name} · ${STATE_LABEL[driver.state] ?? driver.state}`,
        );
        element.style.cursor = "pointer";
        element.setAttribute("role", "button");
        element.tabIndex = 0;
        element.addEventListener("click", () => driverPopup(driver));
        element.addEventListener("keydown", (event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            driverPopup(driver);
          }
        });
        return new maplibregl.Marker({ element })
          .setLngLat([driver.location.lng, driver.location.lat])
          .addTo(map);
      }),
      ...requests.map((request) =>
        new maplibregl.Marker({
          element: dot("--critical", 15, `Waiting ${duration(request.waiting_s)} · ${request.code}`),
        })
          .setLngLat([request.pickup.lng, request.pickup.lat])
          .addTo(map),
      ),
    ];
    return () => markers.forEach((marker) => marker.remove());
  }, [map, visible, requests]);

  useEffect(
    () => () => {
      popup.current?.remove();
    },
    [],
  );

  useEffect(() => {
    if (!map) return;
    const features = Object.entries(surge).map(([cell, multiplier]) => ({
      type: "Feature" as const,
      properties: { multiplier: Number(multiplier) },
      geometry: { type: "Polygon" as const, coordinates: [cellToBoundary(cell, true)] },
    }));
    const sourceData = { type: "FeatureCollection" as const, features };
    const source = map.getSource("surge") as maplibregl.GeoJSONSource | undefined;
    if (source) source.setData(sourceData);
    else {
      map.addSource("surge", { type: "geojson", data: sourceData });
      map.addLayer({
        id: "surge-fill",
        type: "fill",
        source: "surge",
        paint: {
          "fill-color": getComputedStyle(document.documentElement).getPropertyValue("--chart-2").trim(),
          "fill-opacity": ["interpolate", ["linear"], ["get", "multiplier"], 1, 0.08, 2, 0.4],
        },
      });
    }
  }, [map, surge]);

  function fitActivity() {
    if (!map) return;
    const points = [
      ...visible.map((driver) => driver.location),
      ...requests.map((request) => request.pickup),
    ];
    if (points.length) {
      const bounds = points.reduce(
        (box, point) => box.extend([point.lng, point.lat]),
        new maplibregl.LngLatBounds([points[0].lng, points[0].lat], [points[0].lng, points[0].lat]),
      );
      map.fitBounds(bounds, { padding: 60, maxZoom: 14, duration: 600 });
    } else if (country)
      map.flyTo({ center: [country.map.lng, country.map.lat], zoom: country.map.zoom, duration: 600 });
  }

  return (
    <div>
      <OperationsHeader
        title="Live"
        description="Follow drivers, spot waiting riders, and see where your operation needs support."
        actions={
          <>
            <UpdateIndicator at={dataUpdatedAt} error={error} fetching={isFetching} interval="5s" />
            <RefreshButton busy={isFetching} onClick={() => void live.refetch()} />
          </>
        }
      />
      <div className="mb-5 grid grid-cols-2 gap-3 xl:grid-cols-4">
        <MetricTile
          label="Drivers online"
          value={data ? count(drivers.length) : error ? "—" : undefined}
          note="Drivers currently sharing their location"
          icon={<CarFront size={17} />}
        />
        <MetricTile
          label="Available now"
          value={data ? count(byState.IDLE) : error ? "—" : undefined}
          note="Online and free for a new request"
          icon={<CheckCheck size={17} />}
          accent
        />
        <MetricTile
          label="On a trip"
          value={data ? count(byState.ON_TRIP) : error ? "—" : undefined}
          note="Drivers currently serving a trip"
          icon={<Route size={17} />}
        />
        <MetricTile
          label="Riders waiting"
          value={data ? count(requests.length) : error ? "—" : undefined}
          note={
            waiting.length
              ? `Longest wait ${duration(waiting[0].waiting_s)}`
              : "Requests looking for a driver"
          }
          icon={<Clock3 size={17} />}
        />
      </div>
      {error ? (
        <div className="mb-4">
          <ErrorNote error={error} />
        </div>
      ) : null}
      <div className="grid items-start gap-5 xl:grid-cols-[minmax(0,1fr)_340px]">
        <OperationsPanel
          title={area?.name || country?.name?.en || "Service area"}
          description="Driver locations and waiting pickups"
          action={
            <button
              type="button"
              disabled={!map}
              onClick={fitActivity}
              className="inline-flex items-center gap-1.5 rounded-lg bg-surface-2 px-2.5 py-2 text-[11px] font-medium text-ink hover:bg-line disabled:opacity-40"
            >
              <LocateFixed size={13} aria-hidden="true" />
              Fit activity
            </button>
          }
        >
          <div className="min-w-0 overflow-hidden border-b border-line px-3 py-2">
            <FilterBar
              label="Filter map drivers"
              value={driverFilter}
              options={DRIVER_FILTERS.map((filter) => ({
                ...filter,
                count: data
                  ? filter.value === "ALL"
                    ? drivers.length
                    : (byState[filter.value] ?? 0)
                  : undefined,
              }))}
              onChange={setDriverFilter}
            />
          </div>
          <div className="relative">
            <div
              ref={container}
              className="h-[380px] w-full bg-surface-2 sm:h-[520px] xl:h-[560px]"
              role="region"
              aria-label="Live map of drivers and waiting riders"
            />
            <div className="absolute left-3 top-3 rounded-lg border border-line bg-surface px-3 py-2 text-[10px] font-medium shadow-sm">
              <span className="tnum">
                {data
                  ? `${visible.length} ${visible.length === 1 ? "driver" : "drivers"} shown`
                  : "Loading locations…"}
              </span>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-t border-line px-4 py-3 text-[10px] text-muted">
            {Object.entries(STATE_LABEL).map(([state, label]) => (
              <span key={state} className="inline-flex items-center gap-1.5">
                <span
                  aria-hidden="true"
                  className="size-1.5 rounded-full"
                  style={{ background: `var(${STATE_COLOUR[state]})` }}
                />
                {label}
              </span>
            ))}
            <span className="inline-flex items-center gap-1.5">
              <span aria-hidden="true" className="size-1.5 rounded-full bg-critical" />
              Waiting rider
            </span>
          </div>
        </OperationsPanel>
        <div className="grid min-w-0 gap-5">
          <OperationsPanel
            title="Live activity"
            action={<Users size={16} className="text-muted" aria-hidden="true" />}
          >
            <div className="border-b border-line px-3 py-2">
              <FilterBar
                label="Live activity view"
                value={activity}
                options={[
                  { value: "drivers", label: "Drivers", count: data ? visible.length : undefined },
                  { value: "waiting", label: "Waiting", count: data ? waiting.length : undefined },
                ]}
                onChange={setActivity}
              />
            </div>
            {isLoading ? (
              <div className="p-4">
                <Skeleton className="h-32" />
              </div>
            ) : error && !data ? (
              <Empty title="Activity unavailable" body="Refresh to try again." />
            ) : activity === "drivers" ? (
              visible.length ? (
                <ul className="max-h-[330px] divide-y divide-line overflow-y-auto">
                  {visible.map((driver) => (
                    <li
                      key={driver.driver_id}
                      className={`flex items-center gap-2 px-4 py-3 transition-colors ${selected === driver.driver_id ? "bg-brand-soft/50" : "hover:bg-surface-2"}`}
                    >
                      <button
                        type="button"
                        onClick={() => driverPopup(driver)}
                        aria-label={`Locate driver ${driver.name}`}
                        className="flex min-w-0 flex-1 items-center gap-2.5 text-left"
                      >
                        <Initials name={driver.name} />
                        <span className="min-w-0">
                          <span className="block truncate text-xs font-semibold">
                            {driver.name || "Driver"}
                          </span>
                          <span className="mt-1 flex flex-wrap items-center gap-1.5 text-[10px] text-muted">
                            <span
                              className="size-1.5 rounded-full"
                              style={{ background: `var(${STATE_COLOUR[driver.state]})` }}
                              aria-hidden="true"
                            />
                            {STATE_LABEL[driver.state]}
                            <span>
                              · {driver.vehicle_classes.map(className).join(", ") || "Vehicle unavailable"}
                            </span>
                          </span>
                        </span>
                      </button>
                      <Link
                        href={`/drivers/${driver.driver_id}`}
                        aria-label={`Open driver profile for ${driver.name}`}
                        className="grid size-8 shrink-0 place-items-center rounded-lg text-muted hover:bg-surface-2 hover:text-brand-text"
                      >
                        <ArrowUpRight size={14} aria-hidden="true" />
                      </Link>
                    </li>
                  ))}
                </ul>
              ) : (
                <Empty
                  title="No drivers in this view"
                  body={
                    driverFilter === "ALL"
                      ? "Drivers appear when they come online."
                      : "Try a different driver status."
                  }
                  icon={<CarFront size={21} />}
                />
              )
            ) : waiting.length ? (
              <ul className="max-h-[330px] divide-y divide-line overflow-y-auto">
                {waiting.map((request) => (
                  <li key={request.trip_id} className="px-4 py-3">
                    <div className="flex items-center justify-between gap-2">
                      <Link
                        href={`/trips/${request.trip_id}`}
                        className="inline-flex items-center gap-1 font-mono text-xs font-semibold text-brand-text"
                      >
                        {request.code}
                        <ArrowUpRight size={12} aria-hidden="true" />
                      </Link>
                      <Chip tone={request.waiting_s >= 180 ? "bad" : "warn"}>
                        {duration(request.waiting_s)}
                      </Chip>
                    </div>
                    <button
                      type="button"
                      onClick={() =>
                        map?.flyTo({
                          center: [request.pickup.lng, request.pickup.lat],
                          zoom: 15,
                          duration: 600,
                        })
                      }
                      className="mt-2 inline-flex items-center gap-1.5 text-[11px] text-muted hover:text-brand-text"
                    >
                      <MapPin size={12} aria-hidden="true" />
                      {className(request.vehicle_class)}
                      <span>· Locate pickup</span>
                    </button>
                  </li>
                ))}
              </ul>
            ) : (
              <Empty
                title="No riders waiting"
                body="New requests will appear here while they look for a driver."
                icon={<CheckCheck size={21} />}
              />
            )}
            <p className="border-t border-line px-4 py-3 text-[10px] text-muted">
              {activity === "drivers"
                ? "Select a driver to locate them on the map."
                : "Requests are ordered by longest wait."}
            </p>
          </OperationsPanel>
          <OperationsPanel
            title="Map layers"
            action={<Layers2 size={15} className="text-muted" aria-hidden="true" />}
          >
            <div className="grid gap-4 p-4">
              <Layer
                label="Demand grid"
                description="Available supply and waiting requests"
                checked={grid}
                onChange={setGrid}
              />
              <Layer
                label="Service areas"
                description="Operating boundaries and area names"
                checked={coverage}
                onChange={setCoverage}
              />
            </div>
          </OperationsPanel>
          <OperationsPanel
            title="Busy areas"
            action={
              <Chip tone={Object.keys(surge).length ? "warn" : "muted"}>
                {count(Object.keys(surge).length)}
              </Chip>
            }
          >
            {!data ? (
              <p className="p-4 text-xs text-muted">
                {error ? "Busy areas could not be loaded." : "Loading area activity…"}
              </p>
            ) : !Object.keys(surge).length ? (
              <div className="flex items-start gap-2.5 p-4">
                <span className="grid size-8 shrink-0 place-items-center rounded-lg bg-surface-2 text-muted">
                  <Flame size={15} aria-hidden="true" />
                </span>
                <div>
                  <p className="text-xs font-medium">No surge right now</p>
                  <p className="mt-1 text-[10px] leading-relaxed text-muted">
                    Busy areas appear when surge pricing applies.
                  </p>
                </div>
              </div>
            ) : (
              <ul className="max-h-48 divide-y divide-line overflow-y-auto">
                {Object.entries(surge).map(([cell, multiplier], index) => (
                  <li key={cell}>
                    <button
                      type="button"
                      onClick={() => {
                        const [lat, lng] = cellToLatLng(cell);
                        map?.flyTo({ center: [lng, lat], zoom: 13, duration: 600 });
                      }}
                      className="flex w-full items-center gap-2 px-4 py-3 text-left hover:bg-surface-2"
                    >
                      <Flame size={14} className="text-warn" aria-hidden="true" />
                      <span className="flex-1 text-xs">{`Surge area ${index + 1}`}</span>
                      <Chip tone="warn">{`${multiplier}×`}</Chip>
                      <ChevronRight size={12} className="text-muted" aria-hidden="true" />
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </OperationsPanel>
        </div>
      </div>
    </div>
  );
}

function Layer({
  label,
  description,
  checked,
  onChange,
}: {
  label: string;
  description: string;
  checked: boolean;
  onChange: (value: boolean) => void;
}) {
  return (
    <div className="flex items-center justify-between gap-3">
      <div>
        <p className="text-xs font-medium">{label}</p>
        <p className="mt-1 text-[10px] leading-relaxed text-muted">{description}</p>
      </div>
      <Toggle label={label} checked={checked} onChange={onChange} />
    </div>
  );
}
