"use client";
/** RoutesCard — fixed fares between two areas (e.g. town to the airport): list, add, retire. */
import { Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Field, Input, Select } from "@/components/ui/Field";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { ErrorNote } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import type { Schemas } from "@/lib/api";
import { money } from "@/lib/format";
import { useDeleteRoute, useSaveRoute } from "./hooks";

const DEFAULT_RADIUS_M = 1500;

const blank = (cls: string): Schemas["FixedRoute"] => ({
  id: "",
  service: "taxi",
  name: "",
  origin: { lat: 0, lng: 0 },
  origin_radius_m: DEFAULT_RADIUS_M,
  destination: { lat: 0, lng: 0 },
  destination_radius_m: DEFAULT_RADIUS_M,
  bidirectional: true,
  vehicle_classes: [cls],
  price: "",
  currency: "TZS",
  surge_applies: false,
  priority: 0,
});

export function RoutesCard({
  routes,
  classes,
  className,
  index,
  editable,
}: {
  routes: Schemas["FixedRoute"][];
  classes: string[];
  className: (k: string) => string;
  index: number;
  editable: boolean;
}) {
  const [draft, setDraft] = useState<Schemas["FixedRoute"]>();
  const [retire, setRetire] = useState<string>();
  const save = useSaveRoute();
  const del = useDeleteRoute();
  const coord = (v: string) => Number(v.replace(/[^\d.-]/g, "")) || 0;
  return (
    <Card
      index={index}
      title="Fixed fares"
      action={
        editable && !draft ? (
          <Button
            size="sm"
            variant="soft"
            icon={<Plus size={14} aria-hidden="true" />}
            onClick={() => setDraft(blank(classes[0] ?? "standard"))}
          >
            Add fixed fare
          </Button>
        ) : null
      }
    >
      <Table head={["Route", "Ride type", "Price", "Both ways", ""]}>
        {routes.map((r) => (
          <Row key={r.id}>
            <Cell className="font-medium">{r.name}</Cell>
            <Cell>{r.vehicle_classes.map(className).join(", ") || "All"}</Cell>
            <Cell className="tnum">{money(r.price, r.currency)}</Cell>
            <Cell>{r.bidirectional ? "Yes" : "No"}</Cell>
            <Cell className="text-right">
              {editable ? (
                <button
                  type="button"
                  aria-label={`Retire ${r.name}`}
                  onClick={() => setRetire(r.id)}
                  className="grid size-8 place-items-center rounded-full text-muted hover:bg-surface-2"
                >
                  <Trash2 size={15} aria-hidden="true" />
                </button>
              ) : null}
            </Cell>
          </Row>
        ))}
      </Table>
      {draft ? (
        <div className="mt-4 grid grid-cols-2 gap-3 rounded-[10px] bg-surface-2 p-4 xl:grid-cols-4">
          <Field label="Name">
            <Input
              value={draft.name}
              onChange={(e) => setDraft({ ...draft, name: e.target.value })}
              placeholder="Town to airport"
            />
          </Field>
          <Field label="Ride type">
            <Select
              value={draft.vehicle_classes[0]}
              onChange={(e) => setDraft({ ...draft, vehicle_classes: [e.target.value] })}
            >
              {classes.map((c) => (
                <option key={c} value={c}>
                  {className(c)}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Price">
            <Input
              value={draft.price}
              onChange={(e) => setDraft({ ...draft, price: e.target.value.replace(/[^\d.]/g, "") })}
            />
          </Field>
          <Field label="Radius at each end (m)">
            <Input
              value={String(draft.origin_radius_m)}
              onChange={(e) => {
                const m = Number(e.target.value.replace(/\D/g, "") || 0);
                setDraft({ ...draft, origin_radius_m: m, destination_radius_m: m });
              }}
            />
          </Field>
          <Field label="From latitude">
            <Input
              value={String(draft.origin.lat)}
              onChange={(e) =>
                setDraft({ ...draft, origin: { ...draft.origin, lat: coord(e.target.value) } })
              }
            />
          </Field>
          <Field label="From longitude">
            <Input
              value={String(draft.origin.lng)}
              onChange={(e) =>
                setDraft({ ...draft, origin: { ...draft.origin, lng: coord(e.target.value) } })
              }
            />
          </Field>
          <Field label="To latitude">
            <Input
              value={String(draft.destination.lat)}
              onChange={(e) =>
                setDraft({ ...draft, destination: { ...draft.destination, lat: coord(e.target.value) } })
              }
            />
          </Field>
          <Field label="To longitude">
            <Input
              value={String(draft.destination.lng)}
              onChange={(e) =>
                setDraft({ ...draft, destination: { ...draft.destination, lng: coord(e.target.value) } })
              }
            />
          </Field>
          <div className="col-span-full flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setDraft(undefined)}>
              Cancel
            </Button>
            <Button
              disabled={!draft.name || !draft.price || !draft.origin.lat || !draft.destination.lat}
              onClick={() =>
                save.mutate({ ...draft, reason: "New fixed fare" }, { onSuccess: () => setDraft(undefined) })
              }
              loading={save.isPending}
            >
              Save fixed fare
            </Button>
          </div>
        </div>
      ) : null}
      <ErrorNote error={save.error ?? del.error} />
      <ReasonDialog
        open={!!retire}
        title="Retire this fixed fare?"
        body="New quotes stop using it; past trips keep their price."
        confirm="Retire"
        danger
        busy={del.isPending}
        onClose={() => setRetire(undefined)}
        onConfirm={(reason) =>
          retire && del.mutate({ id: retire, reason }, { onSuccess: () => setRetire(undefined) })
        }
      />
    </Card>
  );
}
