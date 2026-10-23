"use client";
/** RatingsCard — average received, a bar per star, what they give others, and the latest comments. */
import { Star } from "lucide-react";
import { Card } from "@/components/ui/Card";
import { Empty } from "@/components/ui/States";
import type { Schemas } from "@/lib/api";
import { when } from "@/lib/format";

const STARS = [5, 4, 3, 2, 1] as const;
const RECENT_SHOWN = 6;

export function RatingsCard({
  ratings,
  index,
  who,
}: {
  ratings: Schemas["PersonRatings"];
  index: number;
  who: string;
}) {
  const total = ratings.received.count;
  const commented = ratings.recent.filter((r) => r.comment);
  return (
    <Card index={index} title="Ratings">
      {total === 0 ? (
        <Empty title="Not rated yet" body={`Ratings appear when ${who} finishes trips.`} />
      ) : (
        <div className="grid gap-5 sm:grid-cols-[150px_1fr]">
          <div>
            <p className="tnum flex items-center gap-1.5 text-[30px] font-semibold tracking-tight">
              {ratings.received.average.toFixed(2)}
              <Star size={20} className="fill-current text-warn" aria-hidden="true" />
            </p>
            <p className="tnum text-sm text-muted">{`${total} ratings`}</p>
            <p className="tnum mt-2 text-xs text-muted">
              {ratings.given.count
                ? `Gives others ${ratings.given.average.toFixed(1)} on average (${ratings.given.count})`
                : "Has not rated anyone"}
            </p>
          </div>
          <ul className="grid gap-1.5" aria-label="Ratings by stars">
            {STARS.map((s) => {
              const n = ratings.stars[s - 1] ?? 0;
              return (
                <li key={s} className="grid grid-cols-[28px_1fr_36px] items-center gap-2 text-xs">
                  <span className="tnum text-muted">{`${s} ★`}</span>
                  <span className="h-2 overflow-hidden rounded-full bg-surface-2">
                    <span
                      className={`block h-full rounded-full ${s <= 2 ? "bg-critical" : "bg-brand"}`}
                      style={{ width: `${(n / total) * 100}%` }}
                    />
                  </span>
                  <span className="tnum text-right text-muted">{n}</span>
                </li>
              );
            })}
          </ul>
        </div>
      )}
      {commented.length ? (
        <ul className="mt-5 grid gap-2 text-sm">
          {commented.slice(0, RECENT_SHOWN).map((r) => (
            <li key={`${r.trip_id}${r.rater_role}`} className="rounded-[8px] bg-surface-2 px-3 py-2">
              <p className="tnum text-xs text-muted">{`${r.score} ★ · ${r.other_name ?? "Someone"} · trip ${r.trip_code} · ${when(r.created_at)}`}</p>
              <p>{r.comment}</p>
            </li>
          ))}
        </ul>
      ) : null}
    </Card>
  );
}
