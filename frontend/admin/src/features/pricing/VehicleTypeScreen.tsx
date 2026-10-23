"use client";
/** VehicleTypeScreen — one ride type's settings in the selected country or region: its engine, prices and surge. */
import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { useMe } from "@/features/me";
import { serviceLabel, useClassName } from "@/lib/catalog";
import { useDeleteTariffOverride, usePricing } from "./hooks";
import { useScopeLabel } from "./scopeLabel";
import { TariffCard } from "./TariffCard";

export function VehicleTypeScreen({ service, cls }: { service: string; cls: string }) {
  const { data, isLoading, error } = usePricing();
  const me = useMe();
  const className = useClassName();
  const where = useScopeLabel();
  const remove = useDeleteTariffOverride();
  const [asking, setAsking] = useState(false);
  const t = data?.tariffs.find((x) => x.service === service && x.vehicle_class === cls);
  const title = `${serviceLabel(service)} · ${className(cls)}`;
  const engine = data?.engines.find((e) => e.key === t?.engine);
  return (
    <div>
      <PageHeader
        title={title}
        subtitle={`Settings in ${where.label}${t?.inherited ? ` (using the country price until you save one for this ${where.term.toLowerCase()})` : ""}.`}
        actions={
          <Link
            href="/pricing"
            className="inline-flex items-center gap-1.5 text-sm font-semibold text-brand-text"
          >
            <ArrowLeft size={15} aria-hidden="true" /> All vehicle types
          </Link>
        }
      />
      <ErrorNote error={error ?? remove.error} />
      {isLoading || !data ? (
        <Skeleton className="h-96" />
      ) : !t ? (
        <Empty
          title="This vehicle type has no price here"
          body="It is not set up in the selected country yet."
        />
      ) : (
        <div className="grid max-w-4xl gap-4">
          <TariffCard
            key={JSON.stringify(t)}
            t={t}
            index={1}
            title="Prices"
            editable={me.can("pricing.edit")}
          />
          <Card index={2} title="Pricing engine">
            <p className="text-sm">{engine?.name ?? t.engine}</p>
            <p className="mt-1 text-xs text-muted">
              The engine decides how a fare is worked out from distance and time. Engines are listed under
              Pricing → Engines.
            </p>
          </Card>
          {where.inArea && !t.inherited && me.can("pricing.edit") ? (
            <Card index={3} title={`${where.areaName ?? "Region"} price`}>
              <p className="text-sm text-muted">
                This region has its own price. Remove it to use the country price again.
              </p>
              <div className="mt-3">
                <Button variant="ghost" onClick={() => setAsking(true)}>
                  Use country price
                </Button>
              </div>
            </Card>
          ) : null}
        </div>
      )}
      <ReasonDialog
        open={asking}
        title={`Use the country price for ${title} in ${where.areaName ?? "this region"}?`}
        body="New quotes here use the national price within seconds."
        confirm="Use country price"
        busy={remove.isPending}
        onClose={() => setAsking(false)}
        onConfirm={(reason) => remove.mutate({ service, cls, reason }, { onSuccess: () => setAsking(false) })}
      />
    </div>
  );
}
