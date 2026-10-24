"use client";
/** NewGeofence — kind, code and name, then draw on the map (corners, or a centre and radius) and save with a reason. */
import { Undo2 } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Field, Input, Select } from "@/components/ui/Field";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { ErrorNote } from "@/components/ui/States";
import type { Draft } from "./GeofenceMap";
import { type Kind, useCreateGeofence } from "./hooks";
import { KIND_HINT, KIND_LABEL } from "./labels";
import { PolygonTools } from "./PolygonTools";
import { polygonFrom } from "./shapes";

const RADIUS = { min: 50, max: 5000, step: 50 };

export function NewGeofence({
  country,
  draft,
  onDraft,
  kind,
  onKind,
  onDone,
}: {
  country: string;
  draft: Draft;
  onDraft: (d: Draft) => void;
  kind: Kind;
  onKind: (k: Kind) => void;
  onDone: (id: string) => void;
}) {
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [asking, setAsking] = useState(false);
  const create = useCreateGeofence();
  const shapeReady = kind === "PLACE" ? !!draft.center : draft.points.length >= 3;
  return (
    <div className="grid gap-3">
      <ErrorNote error={create.error} />
      <Field label="Kind" hint={KIND_HINT[kind]}>
        <Select value={kind} onChange={(e) => onKind(e.target.value as Kind)}>
          {(["ZONE", "PLACE", "DISTRICT"] as const).map((k) => (
            <option key={k} value={k}>
              {KIND_LABEL[k]}
            </option>
          ))}
        </Select>
      </Field>
      <Field label="Name">
        <Input
          value={name}
          maxLength={80}
          onChange={(e) => {
            setName(e.target.value);
            if (!code || code === slug(name)) setCode(slug(e.target.value));
          }}
        />
      </Field>
      <Field label="Code" hint="Lowercase letters, numbers and dashes; unique in the country">
        <Input value={code} onChange={(e) => setCode(slug(e.target.value))} />
      </Field>
      {kind === "PLACE" ? (
        <label className="grid gap-1.5 text-sm">
          <span className="flex justify-between font-medium">
            Radius <span className="tnum text-muted">{`${draft.radiusM} m`}</span>
          </span>
          <input
            type="range"
            min={RADIUS.min}
            max={RADIUS.max}
            step={RADIUS.step}
            value={draft.radiusM}
            onChange={(e) => onDraft({ ...draft, radiusM: Number(e.target.value) })}
            className="accent-[var(--brand)]"
          />
          <span className="text-xs text-muted">
            {draft.center ? "Click the map to move the centre." : "Click the map to place the centre."}
          </span>
        </label>
      ) : (
        <PolygonTools draft={draft} onDraft={onDraft} />
      )}
      <Button disabled={!shapeReady || !name.trim() || code.length < 2} onClick={() => setAsking(true)}>
        Save geofence
      </Button>
      <ReasonDialog
        open={asking}
        title={`Save ${name}?`}
        body="The shape is checked (valid outline, size, country). A profile can then be applied to it."
        confirm="Save"
        busy={create.isPending}
        onClose={() => setAsking(false)}
        onConfirm={(reason) => {
          const shape =
            kind === "PLACE"
              ? { center: draft.center, radius_m: draft.radiusM }
              : { geometry: polygonFrom(draft.points) ?? undefined };
          create.mutate(
            { country, kind, code, name: name.trim(), reason, ...shape },
            {
              onSuccess: (f) => {
                setAsking(false);
                onDone(f.id);
              },
            },
          );
        }}
      />
    </div>
  );
}

/** "Julius Nyerere Airport" → "julius-nyerere-airport". */
function slug(s: string): string {
  return s
    .toLowerCase()
    .normalize("NFKD")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 48);
}
