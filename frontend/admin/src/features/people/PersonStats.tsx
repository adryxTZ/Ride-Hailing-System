"use client";
/** PersonStats — the headline tiles shared by the rider and driver pages. */
import { Stat } from "@/components/ui/Stat";
import type { Schemas } from "@/lib/api";
import { money, percent } from "@/lib/format";

export function PersonStats({
  trips,
  rating,
  role,
}: {
  trips: Schemas["PersonTripStats"];
  rating: Schemas["RatingSummary"];
  role: "rider" | "driver";
}) {
  const ended =
    trips.completed +
    trips.cancelled_by_rider +
    trips.cancelled_by_driver +
    trips.cancelled_by_staff +
    trips.no_driver;
  const ownCancels = role === "rider" ? trips.cancelled_by_rider : trips.cancelled_by_driver;
  const currency = trips.currency || "TZS";
  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-3">
      <Stat
        index={1}
        label="Trips"
        value={trips.total}
        note={trips.active ? `${trips.active} in progress now` : undefined}
      />
      <Stat
        index={2}
        label="Completed"
        value={trips.completed}
        note={ended ? `${percent(trips.completed / ended)} of ended trips` : undefined}
      />
      <Stat
        index={3}
        label={role === "rider" ? "Cancelled by them" : "Cancelled by driver"}
        value={ownCancels}
        note={`${role === "rider" ? trips.cancelled_by_driver : trips.cancelled_by_rider} by the ${role === "rider" ? "driver" : "rider"} · ${trips.no_driver} no driver`}
      />
      <Stat
        index={4}
        label={role === "rider" ? "Spent" : "Fares driven"}
        value={Number(trips.fares_total)}
        format={(n) => money(n, currency)}
      />
      <Stat
        index={5}
        label="Average fare"
        value={Number(trips.avg_fare)}
        format={(n) => money(n, currency)}
      />
      <Stat
        index={6}
        label="Rating"
        value={rating.average}
        format={(n) => (rating.count ? n.toFixed(2) : "None")}
        note={`${rating.count} ratings`}
      />
    </div>
  );
}
