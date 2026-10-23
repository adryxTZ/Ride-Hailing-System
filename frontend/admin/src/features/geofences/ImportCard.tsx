"use client";
/** ImportCard — paste or pick a GeoJSON FeatureCollection, check it (dry run), then import all or nothing. */
import { FileUp } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { Field, Select } from "@/components/ui/Field";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { ErrorNote } from "@/components/ui/States";
import { type Kind, useImportGeofences } from "./hooks";
import { KIND_LABEL } from "./labels";

type Result = { code: string; outcome: string; error?: string };

export function ImportCard({ country }: { country: string }) {
  const [kind, setKind] = useState<Kind>("DISTRICT");
  const [text, setText] = useState("");
  const [results, setResults] = useState<Result[]>();
  const [asking, setAsking] = useState(false);
  const run = useImportGeofences();
  const parsed = (() => {
    try {
      return JSON.parse(text) as Record<string, unknown>;
    } catch {
      return null;
    }
  })();
  const check = () =>
    parsed &&
    run.mutate(
      { country, kind, features: parsed, dry_run: true },
      { onSuccess: (r) => setResults(r.results) },
    );
  const valid = !!results?.length && results.every((r) => r.outcome !== "INVALID");
  return (
    <div className="grid gap-3">
      <ErrorNote error={run.error} />
      <Field label="Kind of every feature">
        <Select
          value={kind}
          onChange={(e) => {
            setKind(e.target.value as Kind);
            setResults(undefined);
          }}
        >
          {(["DISTRICT", "ZONE", "PLACE"] as const).map((k) => (
            <option key={k} value={k}>
              {KIND_LABEL[k]}
            </option>
          ))}
        </Select>
      </Field>
      <label className="inline-flex w-fit cursor-pointer items-center gap-2 rounded-[8px] bg-surface-2 px-3 py-2 text-sm font-semibold">
        <FileUp size={15} aria-hidden="true" /> Choose a .geojson file
        <input
          type="file"
          accept=".geojson,.json,application/geo+json,application/json"
          className="hidden"
          onChange={async (e) => {
            const f = e.target.files?.[0];
            if (f) {
              setText(await f.text());
              setResults(undefined);
            }
          }}
        />
      </label>
      <textarea
        aria-label="GeoJSON"
        value={text}
        onChange={(e) => {
          setText(e.target.value);
          setResults(undefined);
        }}
        rows={6}
        placeholder='{"type":"FeatureCollection","features":[{"properties":{"name":"…","code":"…"},"geometry":{…}}]}'
        className="w-full rounded-[8px] bg-surface-2 px-3 py-2 font-mono text-xs outline-none focus:ring-2 focus:ring-brand"
      />
      <p className="text-xs text-muted">
        Each feature needs properties.name and properties.code; places need a Point and properties.radius_m.
        An existing code gets a new shape version.
      </p>
      <div className="flex gap-2">
        <Button variant="soft" disabled={!parsed} loading={run.isPending && !asking} onClick={check}>
          Check
        </Button>
        <Button disabled={!valid} onClick={() => setAsking(true)}>
          Import
        </Button>
      </div>
      {results ? (
        <ul className="grid gap-1 text-sm">
          {results.map((r) => (
            <li key={r.code} className="flex items-center gap-2">
              <Chip tone={r.outcome === "INVALID" ? "bad" : r.outcome === "CREATE" ? "ok" : "live"}>
                {r.outcome.toLowerCase()}
              </Chip>
              <span className="font-mono text-xs">{r.code}</span>
              {r.error ? <span className="text-xs text-critical">{r.error}</span> : null}
            </li>
          ))}
        </ul>
      ) : null}
      <ReasonDialog
        open={asking}
        title={`Import ${results?.length ?? 0} ${KIND_LABEL[kind].toLowerCase()}s?`}
        body="All of them are saved together, or none if one fails."
        confirm="Import"
        busy={run.isPending}
        onClose={() => setAsking(false)}
        onConfirm={(reason) =>
          parsed &&
          run.mutate(
            { country, kind, features: parsed, dry_run: false, reason },
            {
              onSuccess: (r) => {
                setResults(r.results);
                setAsking(false);
                setText("");
              },
            },
          )
        }
      />
    </div>
  );
}
