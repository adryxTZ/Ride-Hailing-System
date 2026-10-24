"use client";
import maplibregl from "maplibre-gl";
/** EvidenceView — what a check saw: GPS ping pairs on a map, self-booking signals, or the cancellation numbers. */
import Link from "next/link";
import { useEffect } from "react";
import { DEFAULT_CENTRE, dot, useMap } from "@/components/map/useMap";
import { Cell, Row, Table } from "@/components/ui/Table";
import { dateTime, km } from "@/lib/format";
import type { Flag } from "./hooks";
import { SIGNAL_LABEL } from "./labels";

type GpsPing = { lat: number; lng: number; at: string; mocked?: boolean };
type Pair = { from?: GpsPing; to: GpsPing; speed_mps?: number; jump_m?: number };
type Signal = { code: string; severity: string; detail?: string };
type Rate = { count: number; of: number; rate: number; threshold: number; trip_ids?: string[] };

const LINE = "fraud-pairs";
const FIT_PADDING = 48;
const KMH_PER_MPS = 3.6;

function GpsMap({ pairs }: { pairs: Pair[] }) {
  const { container, map } = useMap(12, DEFAULT_CENTRE, false);
  useEffect(() => {
    if (!map || !pairs.length) return;
    const pts = pairs.flatMap((p) => [...(p.from ? [p.from] : []), p.to]);
    const markers = pts.map((p, i) =>
      new maplibregl.Marker({
        element: dot(i % 2 ? "--critical" : "--ink", 12, dateTime(p.at)),
      })
        .setLngLat([p.lng, p.lat])
        .addTo(map),
    );
    const colour = getComputedStyle(document.documentElement).getPropertyValue("--critical").trim();
    map.addSource(LINE, {
      type: "geojson",
      data: {
        type: "FeatureCollection",
        features: pairs
          .filter((p) => p.from)
          .map((p) => ({
            type: "Feature",
            properties: {},
            geometry: {
              type: "LineString",
              coordinates: [
                [p.from?.lng ?? 0, p.from?.lat ?? 0],
                [p.to.lng, p.to.lat],
              ],
            },
          })),
      },
    });
    map.addLayer({
      id: LINE,
      type: "line",
      source: LINE,
      paint: { "line-color": colour, "line-width": 2, "line-dasharray": [2, 2] },
    });
    const bounds = pts.reduce(
      (b, p) => b.extend([p.lng, p.lat]),
      new maplibregl.LngLatBounds([pts[0].lng, pts[0].lat], [pts[0].lng, pts[0].lat]),
    );
    map.fitBounds(bounds, { padding: FIT_PADDING, maxZoom: 14, duration: 0 });
    return () => {
      markers.forEach((m) => m.remove());
      if (!map.style) return;
      if (map.getLayer(LINE)) map.removeLayer(LINE);
      if (map.getSource(LINE)) map.removeSource(LINE);
    };
  }, [map, pairs]);
  return (
    <div
      ref={container}
      className="h-[300px] w-full overflow-hidden rounded-[10px] bg-surface-2"
      role="img"
      aria-label="Map of the GPS jumps that struck"
    />
  );
}

function Gps({ ev }: { ev: { mocked?: boolean; pairs?: Pair[] } }) {
  const pairs = ev.pairs ?? [];
  return (
    <div className="grid gap-4">
      {ev.mocked ? (
        <p className="text-xs">The phone itself reported the location as mocked (a fake-GPS app is on).</p>
      ) : null}
      {pairs.some((p) => p.from) ? <GpsMap pairs={pairs} /> : null}
      <Table head={["When", "Moved", "In", "Speed"]}>
        {pairs.map((p) => (
          <Row key={p.to.at}>
            <Cell className="tnum text-xs">{dateTime(p.to.at)}</Cell>
            <Cell className="tnum text-xs">{p.jump_m ? km(p.jump_m) : "—"}</Cell>
            <Cell className="tnum text-xs">
              {p.from ? `${Math.round((Date.parse(p.to.at) - Date.parse(p.from.at)) / 1000)} s` : "—"}
            </Cell>
            <Cell className="tnum text-xs">
              {p.speed_mps ? `${Math.round(p.speed_mps * KMH_PER_MPS)} km/h` : p.to.mocked ? "Mocked" : "—"}
            </Cell>
          </Row>
        ))}
      </Table>
    </div>
  );
}

function Rates({ ev }: { ev: Rate }) {
  return (
    <div className="grid gap-3 text-xs">
      <p className="tnum text-sm">
        <span className="text-2xl font-semibold">{`${Math.round(ev.rate * 100)}%`}</span>
        <span className="ml-2 text-muted">{`${ev.count} of ${ev.of} in the last 7 days (flags at ${Math.round(ev.threshold * 100)}%)`}</span>
      </p>
      {ev.trip_ids?.length ? (
        <p className="flex flex-wrap gap-2">
          {ev.trip_ids.map((id) => (
            <Link
              key={id}
              href={`/trips/${id}`}
              className="tnum rounded-[8px] bg-surface-2 px-2 py-1 hover:text-brand-text"
            >
              {id.slice(0, 8)}
            </Link>
          ))}
        </p>
      ) : null}
    </div>
  );
}

export function EvidenceView({ flag }: { flag: Flag }) {
  const ev = flag.evidence as Record<string, unknown>;
  if (flag.rule === "GPS_SPOOF") return <Gps ev={ev as { mocked?: boolean; pairs?: Pair[] }} />;
  if (flag.rule === "SELF_BOOKING")
    return (
      <ul className="grid gap-2 text-xs">
        {((ev.signals as Signal[]) ?? []).map((s) => (
          <li key={s.code} className="rounded-[8px] bg-surface-2 px-3 py-2">
            {SIGNAL_LABEL[s.code] ?? s.code}
            {s.detail ? <span className="ml-1 text-muted">{`(${s.detail})`}</span> : null}
          </li>
        ))}
      </ul>
    );
  return <Rates ev={ev as unknown as Rate} />;
}
