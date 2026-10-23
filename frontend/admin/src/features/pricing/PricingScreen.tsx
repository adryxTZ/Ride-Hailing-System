"use client";

import { ArrowUpRight, Bike, CarFront, ChevronRight, Flame, MapPin, Package, Route } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Chip } from "@/components/ui/Chip";
import {
  FilterBar,
  MetricTile,
  OperationsHeader,
  RecordsTable,
  RefreshButton,
} from "@/components/ui/Operations";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { useMe } from "@/features/me";
import type { Schemas } from "@/lib/api";
import { serviceLabel, useClassName } from "@/lib/catalog";
import { count, money } from "@/lib/format";
import { usePricing } from "./hooks";
import { RoutesCard } from "./RoutesCard";
import { useScopeLabel } from "./scopeLabel";

const ORDER = ["boda", "bajaji", "standard", "xl"];
const ENGINE_LABEL: Record<string, string> = {
  per_km_min: "Per km + minute",
  bracket_meter: "Distance brackets",
  flat_rate: "Flat price",
};

export function PricingScreen() {
  const router = useRouter();
  const pricing = usePricing();
  const { data, isLoading, error } = pricing;
  const me = useMe();
  const className = useClassName();
  const where = useScopeLabel();
  const [service, setService] = useState("all");
  const tariffs = [...(data?.tariffs ?? [])].sort(
    (a, b) =>
      b.service.localeCompare(a.service) || ORDER.indexOf(a.vehicle_class) - ORDER.indexOf(b.vehicle_class),
  );
  const visible = tariffs.filter((tariff) => service === "all" || tariff.service === service);
  const services = [...new Set(tariffs.map((tariff) => tariff.service))];
  return (
    <div>
      <OperationsHeader
        title="Vehicle types"
        description="Manage ride and delivery prices, pricing engines, and fixed fares for your service area."
        actions={
          <>
            <Link
              href="/pricing/fees"
              className="inline-flex h-9 items-center gap-2 rounded-lg bg-nav px-3 text-xs font-semibold text-nav-ink"
            >
              Fees & profiles <ArrowUpRight size={14} aria-hidden="true" />
            </Link>
            <RefreshButton busy={pricing.isFetching} onClick={() => void pricing.refetch()} />
          </>
        }
      />
      <div className="mb-5 grid grid-cols-2 gap-3 xl:grid-cols-4">
        <MetricTile
          label="Ride types"
          value={data ? count(tariffs.filter((t) => t.service === "taxi").length) : error ? "—" : undefined}
          note="Configured passenger vehicle types"
          icon={<CarFront size={17} />}
          accent
        />
        <MetricTile
          label="Delivery types"
          value={data ? count(tariffs.filter((t) => t.service === "parcel").length) : error ? "—" : undefined}
          note="Vehicle types for package delivery"
          icon={<Package size={17} />}
        />
        <MetricTile
          label="Fixed fares"
          value={data ? count(data.fixed_routes.length) : error ? "—" : undefined}
          note="Set prices between specific places"
          icon={<Route size={17} />}
        />
        <MetricTile
          label="Surge enabled"
          value={data ? count(tariffs.filter((t) => t.surge_enabled).length) : error ? "—" : undefined}
          note="Vehicle prices with surge switched on"
          icon={<Flame size={17} />}
        />
      </div>
      <div className="mb-5 flex items-start gap-3 rounded-xl border border-line bg-surface px-4 py-3">
        <MapPin size={17} className="mt-0.5 shrink-0 text-brand-text" aria-hidden="true" />
        <div>
          <p className="text-xs font-semibold">{where.label || "Selected country"}</p>
          <p className="mt-1 text-[11px] leading-relaxed text-muted">
            {where.inArea
              ? `Country prices apply until you set an override for this ${where.term.toLowerCase()}.`
              : `Country prices apply across all ${where.term.toLowerCase()}s. Select a ${where.term.toLowerCase()} in the top bar to manage its overrides.`}
          </p>
        </div>
      </div>
      {error ? (
        <div className="mb-4">
          <ErrorNote error={error} />
        </div>
      ) : null}
      {isLoading ? (
        <Skeleton className="h-80" />
      ) : !data ? (
        <Empty title="Prices unavailable" body="Use Refresh to try again." />
      ) : (
        <div className="grid gap-5">
          <section
            aria-label="Vehicle prices"
            className="min-w-0 overflow-hidden rounded-xl border border-line bg-surface"
          >
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-4 py-3">
              <FilterBar
                label="Filter pricing services"
                value={service}
                options={[
                  { value: "all", label: "All services", count: tariffs.length },
                  ...services.map((value) => ({
                    value,
                    label: serviceLabel(value),
                    count: tariffs.filter((t) => t.service === value).length,
                  })),
                ]}
                onChange={setService}
              />
              <span className="text-[11px] text-muted">Open a type to manage its price</span>
            </div>
            {!visible.length ? (
              <Empty title="No prices in this view" />
            ) : (
              <>
                <RecordsTable
                  head={[
                    "Vehicle type",
                    "Engine",
                    "Base fare",
                    "Per km",
                    "Per minute",
                    "Minimum",
                    "Cancel fee",
                    "Surge",
                    "Price scope",
                    "",
                  ]}
                >
                  {visible.map((t) => (
                    <tr
                      key={`${t.service}/${t.vehicle_class}`}
                      className="cursor-pointer transition-colors hover:bg-surface-2"
                      onClick={(event) => {
                        if (!(event.target as HTMLElement).closest("a"))
                          router.push(`/pricing/${t.service}/${t.vehicle_class}`);
                      }}
                    >
                      <td className="px-5 py-4">
                        <Link
                          href={`/pricing/${t.service}/${t.vehicle_class}`}
                          className="group flex items-center gap-2.5"
                        >
                          <VehicleIcon vehicle={t.vehicle_class} />
                          <span className="min-w-0">
                            <span className="block whitespace-nowrap text-xs font-semibold group-hover:text-brand-text">
                              {className(t.vehicle_class)}
                            </span>
                            <span className="mt-1 block text-[10px] text-muted">
                              {serviceLabel(t.service)}
                            </span>
                          </span>
                        </Link>
                      </td>
                      <td className="px-4 py-4 text-[11px] text-muted">
                        {ENGINE_LABEL[t.engine] ?? t.engine}
                      </td>
                      {[
                        t.params.base_fare,
                        t.params.per_km,
                        t.params.per_min,
                        t.minimum_fare,
                        t.cancellation_fee,
                      ].map((value, index) => (
                        <td key={index} className="tnum whitespace-nowrap px-4 py-4 text-xs">
                          {value != null ? money(String(value), t.currency) : "—"}
                        </td>
                      ))}
                      <td className="whitespace-nowrap px-4 py-4 text-[11px]">
                        {t.surge_enabled ? (
                          <span className="inline-flex items-center gap-1 text-warn">
                            <Flame size={12} aria-hidden="true" />
                            {t.params.surge_max ? `Up to ${t.params.surge_max}×` : "On"}
                          </span>
                        ) : (
                          <span className="text-muted">Off</span>
                        )}
                      </td>
                      <td className="px-4 py-4">
                        <Chip tone={t.inherited ? "muted" : "ok"}>
                          {t.inherited
                            ? "Country price"
                            : where.inArea
                              ? where.areaName || "This region"
                              : "Country price"}
                        </Chip>
                      </td>
                      <td className="pr-4">
                        <Link
                          href={`/pricing/${t.service}/${t.vehicle_class}`}
                          aria-label={`Open ${serviceLabel(t.service)} ${className(t.vehicle_class)} pricing`}
                          className="text-muted hover:text-brand-text"
                        >
                          <ChevronRight size={15} aria-hidden="true" />
                        </Link>
                      </td>
                    </tr>
                  ))}
                </RecordsTable>
                <div className="grid divide-y divide-line md:hidden">
                  {visible.map((t) => (
                    <MobilePrice
                      key={`${t.service}/${t.vehicle_class}`}
                      tariff={t}
                      name={className(t.vehicle_class)}
                    />
                  ))}
                </div>
              </>
            )}
            <p className="border-t border-line px-5 py-3 text-[11px] text-muted">{`${visible.length} vehicle ${visible.length === 1 ? "price" : "prices"} · ${me.can("pricing.edit") ? "Changes are saved with an audit reason." : "You have read-only access."}`}</p>
          </section>
          <RoutesCard
            routes={data.fixed_routes}
            classes={tariffs.filter((t) => t.service === "taxi").map((t) => t.vehicle_class)}
            className={className}
            index={2}
            editable={me.can("pricing.edit")}
          />
        </div>
      )}
    </div>
  );
}

function VehicleIcon({ vehicle }: { vehicle: string }) {
  return (
    <span className="grid size-9 shrink-0 place-items-center rounded-lg bg-brand-soft text-brand-text">
      {vehicle === "boda" ? <Bike size={17} aria-hidden="true" /> : <CarFront size={17} aria-hidden="true" />}
    </span>
  );
}
function MobilePrice({ tariff: t, name }: { tariff: Schemas["TariffView"]; name: string }) {
  return (
    <Link href={`/pricing/${t.service}/${t.vehicle_class}`} className="block p-4 hover:bg-surface-2">
      <div className="flex items-center gap-3">
        <VehicleIcon vehicle={t.vehicle_class} />
        <div className="flex-1">
          <p className="text-sm font-semibold">{name}</p>
          <p className="mt-1 text-[11px] text-muted">{`${serviceLabel(t.service)} · ${ENGINE_LABEL[t.engine] ?? t.engine}`}</p>
        </div>
        <ChevronRight size={15} className="text-muted" aria-hidden="true" />
      </div>
      <dl className="mt-4 grid grid-cols-2 gap-3">
        {[
          ["Base fare", t.params.base_fare],
          ["Per km", t.params.per_km],
          ["Per minute", t.params.per_min],
          ["Minimum fare", t.minimum_fare],
          ["Cancellation fee", t.cancellation_fee],
        ].map(([label, value]) => (
          <div key={String(label)}>
            <dt className="text-[10px] text-muted">{String(label)}</dt>
            <dd className="tnum mt-1 text-xs font-semibold">
              {value != null ? money(String(value), t.currency) : "—"}
            </dd>
          </div>
        ))}
        <div>
          <dt className="text-[10px] text-muted">Surge</dt>
          <dd className="mt-1 text-xs font-semibold">
            {t.surge_enabled ? (t.params.surge_max ? `Up to ${t.params.surge_max}×` : "On") : "Off"}
          </dd>
        </div>
      </dl>
    </Link>
  );
}
