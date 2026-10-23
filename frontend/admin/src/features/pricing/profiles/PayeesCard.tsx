"use client";
/** PayeesCard — the third parties a charge can be owed to in this country (a council, an airport …). */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { Field, Input, Select } from "@/components/ui/Field";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { PAYEE_KINDS, type PayeeKind, useCreatePayee, usePayees, useUpdatePayee } from "./hooks";

const KIND_LABEL: Record<PayeeKind, string> = {
  MUNICIPALITY: "City or council",
  AIRPORT: "Airport",
  PARKING_OPERATOR: "Parking operator",
  TAX_AUTHORITY: "Tax authority",
  REGULATOR: "Regulator",
  OTHER: "Other",
};

export function PayeesCard({ country, editable }: { country: string; editable: boolean }) {
  const { data, isLoading } = usePayees(country);
  const create = useCreatePayee();
  const update = useUpdatePayee();
  const [form, setForm] = useState<{ name: string; kind: PayeeKind; reference: string }>({
    name: "",
    kind: "MUNICIPALITY",
    reference: "",
  });
  const [ask, setAsk] = useState<
    { kind: "create" } | { kind: "toggle"; id: string; active: boolean; name: string }
  >();
  return (
    <div className="grid gap-4">
      <ErrorNote error={create.error ?? update.error} />
      {editable ? (
        <div className="grid items-end gap-3 md:grid-cols-[1fr_200px_200px_auto]">
          <Field label="Name">
            <Input
              value={form.name}
              maxLength={80}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
            />
          </Field>
          <Field label="Kind">
            <Select
              value={form.kind}
              onChange={(e) => setForm({ ...form, kind: e.target.value as PayeeKind })}
            >
              {PAYEE_KINDS.map((k) => (
                <option key={k} value={k}>
                  {KIND_LABEL[k]}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Reference" hint="e.g. tax number">
            <Input value={form.reference} onChange={(e) => setForm({ ...form, reference: e.target.value })} />
          </Field>
          <Button disabled={!form.name.trim()} onClick={() => setAsk({ kind: "create" })}>
            Add payee
          </Button>
        </div>
      ) : null}
      {isLoading ? (
        <Skeleton className="h-32" />
      ) : !data?.length ? (
        <Empty title="No payees" body="Add one before a charge can be paid to a third party." />
      ) : (
        <Table head={["Name", "Kind", "Reference", "Status", ""]}>
          {data.map((p) => (
            <Row key={p.id}>
              <Cell className="font-medium">{p.name}</Cell>
              <Cell className="text-muted">{KIND_LABEL[p.kind as PayeeKind] ?? p.kind}</Cell>
              <Cell className="font-mono text-xs text-muted">{p.reference}</Cell>
              <Cell>
                {p.status === "ACTIVE" ? <Chip tone="ok">Active</Chip> : <Chip tone="muted">Off</Chip>}
              </Cell>
              <Cell className="text-right">
                {editable ? (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() =>
                      setAsk({ kind: "toggle", id: p.id, active: p.status === "ACTIVE", name: p.name })
                    }
                  >
                    {p.status === "ACTIVE" ? "Switch off" : "Switch on"}
                  </Button>
                ) : null}
              </Cell>
            </Row>
          ))}
        </Table>
      )}
      <ReasonDialog
        open={!!ask}
        title={
          ask?.kind === "toggle"
            ? `${ask.active ? "Switch off" : "Switch on"} ${ask.name}?`
            : `Add ${form.name}?`
        }
        body={
          ask?.kind === "toggle" && ask.active
            ? "Charges can no longer be paid to it. Not allowed while an active profile charges for it."
            : "Charges can then be paid to it; the platform holds the money until Finance pays it."
        }
        confirm={ask?.kind === "toggle" ? (ask.active ? "Switch off" : "Switch on") : "Add"}
        busy={create.isPending || update.isPending}
        onClose={() => setAsk(undefined)}
        onConfirm={(reason) => {
          if (ask?.kind === "toggle")
            update.mutate(
              { id: ask.id, status: ask.active ? "INACTIVE" : "ACTIVE", reason },
              { onSuccess: () => setAsk(undefined) },
            );
          else
            create.mutate(
              {
                country,
                name: form.name.trim(),
                kind: form.kind,
                reference: form.reference || undefined,
                reason,
              },
              {
                onSuccess: () => {
                  setAsk(undefined);
                  setForm({ name: "", kind: "MUNICIPALITY", reference: "" });
                },
              },
            );
        }}
      />
    </div>
  );
}
