"use client";
/**
 * ChargeDefaultsCard — who receives each kind of extra charge unless a charge
 * says otherwise. Built in: money the driver pays at a gate (tolls, parking,
 * airport and entry fees) goes back to the driver. New charges start with these.
 */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { Field, Select } from "@/components/ui/Field";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import type { Schemas } from "@/lib/api";
import { useChargeDefaults, usePayees, useSetChargeDefault } from "./hooks";
import { CATEGORY_LABEL, RECIPIENT_LABEL } from "./model";

type Default = Schemas["ChargeDefault"];

export function ChargeDefaultsCard({ country, editable }: { country: string; editable: boolean }) {
  const { data, isLoading, error } = useChargeDefaults(country);
  const { data: payees } = usePayees(country);
  const set = useSetChargeDefault();
  const [edit, setEdit] = useState<{
    category: Default["category"];
    recipient: Default["recipient"];
    payee_id?: string;
  }>();
  if (isLoading) return <Skeleton className="h-48" />;
  return (
    <div className="grid gap-3">
      <ErrorNote error={error ?? set.error} />
      <Table head={["Kind of charge", "Goes to", "", ""]}>
        {(data ?? []).map((d) => (
          <Row key={d.category}>
            <Cell className="text-xs font-semibold">{CATEGORY_LABEL[d.category]}</Cell>
            <Cell className="text-xs">
              {RECIPIENT_LABEL[d.recipient]}
              {d.payee_name ? <span className="ml-1.5 text-muted">{d.payee_name}</span> : null}
            </Cell>
            <Cell>{d.custom ? <Chip tone="brand">Changed</Chip> : <Chip>Built-in</Chip>}</Cell>
            <Cell className="text-right">
              {editable ? (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() =>
                    setEdit({
                      category: d.category,
                      recipient: d.recipient,
                      payee_id: d.payee_id ?? undefined,
                    })
                  }
                >
                  Change
                </Button>
              ) : null}
            </Cell>
          </Row>
        ))}
      </Table>
      <ReasonDialog
        open={!!edit}
        title={`Who receives ${edit ? CATEGORY_LABEL[edit.category].toLowerCase() : ""}`}
        body="New charges of this kind start with this. Charges already published keep what they say."
        confirm="Save"
        busy={set.isPending}
        onClose={() => setEdit(undefined)}
        onConfirm={(reason) =>
          edit &&
          (edit.recipient !== "THIRD_PARTY" || edit.payee_id) &&
          set.mutate({ country, ...edit, reason }, { onSuccess: () => setEdit(undefined) })
        }
      >
        {edit ? (
          <>
            <Field label="Goes to">
              <Select
                value={edit.recipient}
                onChange={(e) =>
                  setEdit({ ...edit, recipient: e.target.value as Default["recipient"], payee_id: undefined })
                }
              >
                {Object.entries(RECIPIENT_LABEL).map(([k, v]) => (
                  <option key={k} value={k}>
                    {v}
                  </option>
                ))}
              </Select>
            </Field>
            {edit.recipient === "THIRD_PARTY" ? (
              <Field label="Payee">
                <Select
                  value={edit.payee_id ?? ""}
                  onChange={(e) => setEdit({ ...edit, payee_id: e.target.value || undefined })}
                >
                  <option value="">Choose a payee</option>
                  {(payees ?? [])
                    .filter((p) => p.status === "ACTIVE")
                    .map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name}
                      </option>
                    ))}
                </Select>
              </Field>
            ) : null}
          </>
        ) : null}
      </ReasonDialog>
    </div>
  );
}
