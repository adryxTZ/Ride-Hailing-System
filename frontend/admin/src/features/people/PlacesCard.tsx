"use client";
/** PlacesCard — the map plus the facts behind it: last location, likely home, saved places and top routes. */
import { Home, MapPin, Navigation, Route as RouteIcon } from "lucide-react";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Empty } from "@/components/ui/States";
import type { Schemas } from "@/lib/api";
import { when } from "@/lib/format";
import { PersonMap } from "./PersonMap";

type Places = Schemas["PersonPlaces"];

/** "-6.7724, 39.2181" when a point has no address. */
const coords = (p: { lat: number; lng: number }) => `${p.lat.toFixed(4)}, ${p.lng.toFixed(4)}`;
const label = (p: { address?: string | null; lat: number; lng: number }) => p.address || coords(p);

export function PlacesCard({ places, index }: { places: Places; index: number }) {
  const nothing =
    !places.last_location && !places.likely_home && !places.saved.length && !places.routes.length;
  return (
    <Card index={index} title="Places and routes">
      {nothing ? (
        <Empty title="No places yet" body="Places appear after their first trip or saved address." />
      ) : (
        <div className="grid gap-5 xl:grid-cols-[1fr_340px]">
          <PersonMap places={places} />
          <div className="grid content-start gap-4 text-sm">
            <Fact icon={<Navigation size={15} aria-hidden="true" />} title="Last known location">
              {places.last_location ? (
                <>
                  <p className="font-medium">{label(places.last_location)}</p>
                  <p className="text-muted">
                    {places.last_location.source === "LIVE" ? (
                      <Chip tone="ok">Live from the app</Chip>
                    ) : (
                      `Trip ${places.last_location.trip_code ?? ""}`
                    )}{" "}
                    {when(places.last_location.at)}
                  </p>
                </>
              ) : (
                <p className="text-muted">Not known yet</p>
              )}
            </Fact>
            <Fact icon={<Home size={15} aria-hidden="true" />} title="Home">
              {places.saved.find((s) => s.kind === "HOME") ? (
                <p className="font-medium">{places.saved.find((s) => s.kind === "HOME")?.address}</p>
              ) : null}
              {places.likely_home ? (
                <p className="text-muted">
                  {`Likely home from trips: ${label(places.likely_home)} (${places.likely_home.sightings} trips)`}
                </p>
              ) : !places.saved.some((s) => s.kind === "HOME") ? (
                <p className="text-muted">Not saved, and no pattern in their trips yet</p>
              ) : null}
            </Fact>
            {places.saved.filter((s) => s.kind !== "HOME").length ? (
              <Fact icon={<MapPin size={15} aria-hidden="true" />} title="Saved places">
                <ul className="grid gap-1">
                  {places.saved
                    .filter((s) => s.kind !== "HOME")
                    .map((s) => (
                      <li key={s.id}>
                        <span className="font-medium">{s.label}</span>
                        <span className="text-muted">{` · ${s.address}`}</span>
                      </li>
                    ))}
                </ul>
              </Fact>
            ) : null}
          </div>
        </div>
      )}
      {places.routes.length ? (
        <div className="mt-5">
          <h3 className="mb-2 flex items-center gap-1.5 text-sm font-semibold">
            <RouteIcon size={15} aria-hidden="true" /> Most common routes
          </h3>
          <ol className="grid gap-1.5 text-sm">
            {places.routes.map((r, i) => (
              <li
                key={`${r.from.lat},${r.to.lat},${i}`}
                className="flex items-center gap-3 rounded-[8px] bg-surface-2 px-3 py-2"
              >
                <span className="tnum w-5 text-muted">{i + 1}</span>
                <span className="min-w-0 flex-1 truncate">{`${label(r.from)} → ${label(r.to)}`}</span>
                <span className="tnum whitespace-nowrap text-muted">{`${r.trips} trips · last ${when(r.last_at)}`}</span>
              </li>
            ))}
          </ol>
        </div>
      ) : null}
    </Card>
  );
}

function Fact({
  icon,
  title,
  children,
}: {
  icon: React.ReactNode;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <p className="mb-1 flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-muted">
        {icon}
        {title}
      </p>
      {children}
    </div>
  );
}
