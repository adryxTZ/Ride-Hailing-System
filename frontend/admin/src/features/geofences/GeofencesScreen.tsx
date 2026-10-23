"use client";
/**
 * GeofencesScreen (Pricing › Geofences) — the country's districts, zones and
 * places beside a map. Draw a new one, import GeoJSON, see overlaps, and open
 * one for its shape history and the profile applied to it.
 */
import { Plus, Upload } from "lucide-react";
import { motion } from "motion/react";
import { useRouter } from "next/navigation";
import { Suspense, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { enter } from "@/components/ui/motion";
import { PageHeader } from "@/components/ui/PageHeader";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { useMe } from "@/features/me";
import { useScope } from "@/lib/scope";
import { useUrlState } from "@/lib/urlState";
import { type Draft, GeofenceMap } from "./GeofenceMap";
import { type Kind, useGeofences, useOverlaps } from "./hooks";
import { ImportCard } from "./ImportCard";
import { areaText, KIND_LABEL, STATUS } from "./labels";
import { NewGeofence } from "./NewGeofence";

const EMPTY: Draft = { points: [], radiusM: 500 };
const FILTERS = ["ALL", "ZONE", "PLACE", "DISTRICT"] as const;

export function GeofencesScreen() {
  return (
    <Suspense>
      <Geofences />
    </Suspense>
  );
}

function Geofences() {
  const { scope } = useScope();
  const url = useUrlState();
  const router = useRouter();
  const me = useMe();
  const panel = url.get("panel") as "new" | "import" | undefined;
  const filter = (url.get("kind") ?? "ALL") as (typeof FILTERS)[number];
  const { data, isLoading, error } = useGeofences(scope.country);
  const overlaps = useOverlaps(scope.country);
  const [draft, setDraft] = useState<Draft>(EMPTY);
  const [kind, setKind] = useState<Kind>("ZONE");
  const editable = me.can("areas.edit");
  const shown = (data ?? []).filter((f) => filter === "ALL" || f.kind === filter);
  const sameKind = (overlaps.data ?? []).filter((o) => o.kind === "SAME_KIND");
  return (
    <div>
      <PageHeader
        title="Geofences"
        subtitle="Districts, zones and places pricing profiles can be applied to. A place beats a zone, a zone beats a district, and all of them beat the region and the country default."
        actions={
          editable ? (
            <div className="flex gap-2">
              <Button
                variant="soft"
                icon={<Upload size={15} aria-hidden="true" />}
                onClick={() => url.set({ panel: panel === "import" ? undefined : "import" })}
              >
                Import GeoJSON
              </Button>
              <Button
                icon={<Plus size={15} aria-hidden="true" />}
                onClick={() => {
                  setDraft(EMPTY);
                  url.set({ panel: panel === "new" ? undefined : "new" });
                }}
              >
                New geofence
              </Button>
            </div>
          ) : null
        }
      />
      <ErrorNote error={error} />
      {sameKind.length ? (
        <p className="mb-4 rounded-[10px] bg-surface px-4 py-3 text-sm">
          <Chip tone="warn">Overlaps</Chip> {sameKind.map((o) => `${o.a.name} and ${o.b.name}`).join(" · ")}
          <span className="text-muted"> share ground. Profiles on both need different priorities.</span>
        </p>
      ) : null}
      <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
        <motion.aside {...enter(1)} className="min-w-0 rounded-[10px] bg-surface p-5" aria-label="Geofences">
          {panel === "new" ? (
            <NewGeofence
              country={scope.country}
              draft={draft}
              onDraft={setDraft}
              kind={kind}
              onKind={(k) => {
                setKind(k);
                setDraft(EMPTY);
              }}
              onDone={(id) => router.push(`/pricing/geofences/${id}`)}
            />
          ) : panel === "import" ? (
            <ImportCard country={scope.country} />
          ) : (
            <>
              <div className="mb-3 flex flex-wrap gap-1.5" role="group" aria-label="Filter">
                {FILTERS.map((f) => (
                  <button
                    key={f}
                    type="button"
                    aria-pressed={filter === f}
                    onClick={() => url.set({ kind: f === "ALL" ? undefined : f })}
                    className={`rounded-[8px] px-2.5 py-1 text-xs font-medium transition-colors duration-150 ${filter === f ? "bg-nav text-nav-ink" : "bg-surface-2 text-muted hover:text-ink"}`}
                  >
                    {f === "ALL" ? "All" : `${KIND_LABEL[f]}s`}
                  </button>
                ))}
              </div>
              {isLoading ? (
                <Skeleton className="h-64" />
              ) : !shown.length ? (
                <Empty title="No geofences" body="Draw a zone or place, or import district boundaries." />
              ) : (
                <ul className="grid gap-1.5">
                  {shown.map((f) => (
                    <li key={f.id}>
                      <button
                        type="button"
                        onClick={() => router.push(`/pricing/geofences/${f.id}`)}
                        className="w-full rounded-[8px] bg-surface-2/60 px-3 py-2.5 text-left transition-colors duration-150 hover:bg-surface-2"
                      >
                        <span className="flex items-center gap-2">
                          <span className="font-medium">{f.name}</span>
                          <Chip>{KIND_LABEL[f.kind]}</Chip>
                          <Chip tone={STATUS[f.status]?.tone}>{STATUS[f.status]?.label}</Chip>
                        </span>
                        <span className="tnum mt-0.5 block text-xs text-muted">
                          {[
                            f.code,
                            f.kind === "PLACE" ? `${f.radius_m} m radius` : areaText(f.area_m2),
                            `v${f.version}`,
                            f.profile ? `Profile: ${f.profile.name}` : "No profile",
                          ].join(" · ")}
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </>
          )}
        </motion.aside>
        <Card index={2}>
          <GeofenceMap
            fences={data ?? []}
            mode={panel === "new" ? (kind === "PLACE" ? "place" : "polygon") : "off"}
            draft={draft}
            onDraft={setDraft}
            onSelect={(id) => router.push(`/pricing/geofences/${id}`)}
          />
        </Card>
      </div>
    </div>
  );
}
