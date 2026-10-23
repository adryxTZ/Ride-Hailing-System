"use client";
/**
 * ChartsView — the optional Charts tab of a person page: trips and money per
 * day (lines), how trips ended, how they paid, vehicle types and ratings
 * (pies), busiest hours (bars), and for drivers how they answer offers.
 * Pies marked "All time" read the page's totals; the rest follow the window.
 */
import { Chart } from "@/components/charts/Chart";
import { Donut } from "@/components/charts/Donut";
import { Card } from "@/components/ui/Card";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import type { Schemas } from "@/lib/api";
import { useClassName } from "@/lib/catalog";
import { money, percent } from "@/lib/format";
import { type Role, usePersonCharts } from "./hooks";

const WINDOWS = [
  { days: 30, label: "30 days" },
  { days: 90, label: "90 days" },
  { days: 365, label: "1 year" },
] as const;
const HOURS = Array.from({ length: 24 }, (_, h) => `${String(h).padStart(2, "0")}:00`);
const LOW_STARS = 2;
/** Lines that pass through every point without overshooting (no dips below zero between days). */
const LINE = { stroke: { curve: "monotoneCubic" as const, width: 2 } };

/** "21 Oct" from a YYYY-MM-DD local day (no timezone shift). */
function dayLabel(day: string): string {
  const [y, m, d] = day.split("-").map(Number);
  return new Date(y, m - 1, d).toLocaleString("en-US", { day: "numeric", month: "short" });
}

export function ChartsView({
  role,
  id,
  days,
  onDays,
  trips,
  ratings,
}: {
  role: Role;
  id: string;
  days: number;
  onDays: (d: number) => void;
  trips: Schemas["PersonTripStats"];
  ratings: Schemas["PersonRatings"];
}) {
  const { data: c, isLoading, error } = usePersonCharts(role, id, days, true);
  const className = useClassName();
  const currency = trips.currency || "TZS";
  return (
    <div className="grid gap-4">
      <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Chart window">
        <span className="text-sm text-muted">Show the last</span>
        {WINDOWS.map((w) => (
          <button
            key={w.days}
            type="button"
            aria-pressed={days === w.days}
            onClick={() => onDays(w.days)}
            className={`rounded-[8px] px-3 py-1.5 text-sm font-medium transition-colors duration-150 ${days === w.days ? "bg-nav text-nav-ink" : "bg-surface text-muted hover:text-ink"}`}
          >
            {w.label}
          </button>
        ))}
      </div>
      <ErrorNote error={error} />
      {isLoading || !c ? (
        <Skeleton className="h-96" />
      ) : (
        <>
          <Card index={1} title="Trips per day">
            <Chart
              type="line"
              series={[
                { name: "Requested", data: c.daily.map((d) => ({ x: dayLabel(d.day), y: d.requested })) },
                { name: "Completed", data: c.daily.map((d) => ({ x: dayLabel(d.day), y: d.completed })) },
                { name: "Cancelled", data: c.daily.map((d) => ({ x: dayLabel(d.day), y: d.cancelled })) },
              ]}
              options={{ ...LINE, xaxis: { tickAmount: 8 } }}
              table={{
                head: ["Day", "Requested", "Completed", "Cancelled"],
                rows: c.daily.map((d) => [d.day, d.requested, d.completed, d.cancelled]),
              }}
            />
          </Card>
          <Card index={2} title={role === "rider" ? "Spent per day" : "Fares driven per day"}>
            <Chart
              type="area"
              series={[
                {
                  name: role === "rider" ? "Spent" : "Fares",
                  data: c.daily.map((d) => ({ x: dayLabel(d.day), y: Number(d.fares) })),
                },
              ]}
              options={{
                ...LINE,
                xaxis: { tickAmount: 8 },
                yaxis: { labels: { formatter: (v: number) => money(v, currency) } },
              }}
              table={{ head: ["Day", currency], rows: c.daily.map((d) => [d.day, money(d.fares, currency)]) }}
            />
          </Card>
          <div className="grid gap-4 md:grid-cols-2">
            <Card index={3} title="How trips ended · all time">
              <Donut
                total="trips"
                slices={[
                  { label: "Completed", value: trips.completed },
                  { label: "Cancelled by rider", value: trips.cancelled_by_rider },
                  { label: "Cancelled by driver", value: trips.cancelled_by_driver },
                  { label: "No driver or staff", value: trips.no_driver + trips.cancelled_by_staff },
                ]}
              />
            </Card>
            <Card index={4} title="How they paid · all time">
              <Donut
                total="paid trips"
                slices={[
                  { label: "Cash", value: trips.paid_cash },
                  { label: "Mobile money", value: trips.paid_mobile },
                ]}
              />
            </Card>
            <Card index={5} title="Vehicle types">
              <Donut
                total="completed"
                slices={c.by_class.map((x) => ({ label: className(x.vehicle_class), value: x.trips }))}
              />
            </Card>
            <Card index={6} title="Ratings received · all time">
              <Donut
                total="ratings"
                slices={[
                  { label: "5 stars", value: ratings.stars[4] ?? 0 },
                  { label: "4 stars", value: ratings.stars[3] ?? 0 },
                  { label: "3 stars", value: ratings.stars[2] ?? 0 },
                  {
                    label: `${LOW_STARS} stars or less`,
                    value: (ratings.stars[0] ?? 0) + (ratings.stars[1] ?? 0),
                  },
                ]}
              />
            </Card>
          </div>
          <Card index={7} title="Busiest hours">
            <Chart
              type="bar"
              series={[{ name: "Trips", data: c.by_hour.map((n, h) => ({ x: HOURS[h], y: n })) }]}
              options={{
                plotOptions: { bar: { borderRadius: 4, columnWidth: "70%" } },
                xaxis: { tickAmount: 12 },
              }}
              table={{ head: ["Hour", "Trips"], rows: c.by_hour.map((n, h) => [HOURS[h], n]) }}
            />
          </Card>
          {role === "driver" && c.offers ? <OfferCharts offers={c.offers} /> : null}
        </>
      )}
    </div>
  );
}

/** A driver's offer answers: the split (pie), and acceptance and response time per day (two lines, never one dual axis). */
function OfferCharts({ offers }: { offers: NonNullable<Schemas["PersonCharts"]["offers"]> }) {
  const sum = (k: "accepted" | "declined" | "timed_out") => offers.reduce((n, d) => n + d[k], 0);
  const answered = offers.filter((d) => d.offered > 0);
  return (
    <>
      <div className="grid gap-4 md:grid-cols-2">
        <Card index={8} title="Offer answers">
          <Donut
            total="offers"
            slices={[
              { label: "Accepted", value: sum("accepted") },
              { label: "Declined", value: sum("declined") },
              { label: "Timed out", value: sum("timed_out") },
            ]}
          />
        </Card>
        <Card index={9} title="Acceptance rate per day">
          <Chart
            type="line"
            height={240}
            series={[
              {
                name: "Acceptance",
                data: answered.map((d) => ({
                  x: dayLabel(d.day),
                  y: Math.round((d.accepted / d.offered) * 100),
                })),
              },
            ]}
            options={{
              ...LINE,
              yaxis: { max: 100, labels: { formatter: (v: number) => `${Math.round(v)}%` } },
            }}
            table={{
              head: ["Day", "Offers", "Accepted"],
              rows: answered.map((d) => [d.day, d.offered, percent(d.accepted / d.offered)]),
            }}
          />
        </Card>
      </div>
      <Card index={10} title="Average response time per day">
        <Chart
          type="line"
          series={[
            {
              name: "Seconds",
              data: answered
                .filter((d) => d.avg_response_s != null)
                .map((d) => ({ x: dayLabel(d.day), y: Number((d.avg_response_s ?? 0).toFixed(1)) })),
            },
          ]}
          options={{ ...LINE, yaxis: { labels: { formatter: (v: number) => `${v.toFixed(1)} s` } } }}
          table={{
            head: ["Day", "Seconds"],
            rows: answered
              .filter((d) => d.avg_response_s != null)
              .map((d) => [d.day, (d.avg_response_s ?? 0).toFixed(1)]),
          }}
        />
      </Card>
    </>
  );
}
