"use client";
/**
 * DocumentTypesCard — the documents the selected country asks drivers for:
 * staff choose Mandatory (blocks sign-up), Optional or Off for each, whether its
 * expiry date is tracked and required, and whether an expired copy holds the
 * driver. New kinds (e.g. a medical certificate) can be added. Every change asks for a reason.
 */
import { Plus } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Field, Input, Select } from "@/components/ui/Field";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { useScope } from "@/lib/scope";
import { type DocumentType, useDocumentTypes, useSaveDocumentType } from "./hooks";

const REQUIREMENT_LABEL = { MANDATORY: "Mandatory", OPTIONAL: "Optional", OFF: "Not asked" } as const;

export function DocumentTypesCard({ index, editable }: { index: number; editable: boolean }) {
  const { scope } = useScope();
  const { data, isLoading, error } = useDocumentTypes();
  const save = useSaveDocumentType();
  const [pending, setPending] = useState<DocumentType>();
  const [adding, setAdding] = useState(false);
  const change = (t: DocumentType, patch: Partial<DocumentType>) => {
    const next = { ...t, ...patch };
    if (!next.tracks_expiry) Object.assign(next, { expiry_required: false, blocks_when_expired: false });
    setPending(next);
  };
  return (
    <Card
      index={index}
      title={`Documents asked for in ${scope.country}`}
      action={
        editable && !adding ? (
          <Button
            size="sm"
            variant="soft"
            icon={<Plus size={14} aria-hidden="true" />}
            onClick={() => setAdding(true)}
          >
            Add a document
          </Button>
        ) : null
      }
    >
      <ErrorNote error={error ?? save.error} />
      {adding ? (
        <NewType
          country={scope.country}
          onCancel={() => setAdding(false)}
          onSave={(t) => {
            setPending(t);
            setAdding(false);
          }}
        />
      ) : null}
      {isLoading ? (
        <Skeleton className="h-48" />
      ) : !data?.length ? (
        <Empty
          title="No documents configured"
          body="Add the documents drivers must provide in this country."
        />
      ) : (
        <Table
          head={["Document", "For", "Asked", "Track expiry", "Expiry date required", "Hold when expired"]}
        >
          {data.map((t) => (
            <Row key={t.kind}>
              <Cell>
                <div className="font-medium">{t.label.en}</div>
                <div className="font-mono text-xs text-muted">{t.kind}</div>
              </Cell>
              <Cell className="text-muted">{t.owner === "DRIVER" ? "Driver" : "Vehicle"}</Cell>
              <Cell>
                <Select
                  aria-label={`${t.label.en}: asked`}
                  value={t.requirement}
                  disabled={!editable}
                  className="h-9"
                  onChange={(e) => change(t, { requirement: e.target.value as DocumentType["requirement"] })}
                >
                  {Object.entries(REQUIREMENT_LABEL).map(([v, l]) => (
                    <option key={v} value={v}>
                      {l}
                    </option>
                  ))}
                </Select>
              </Cell>
              <Cell>
                <Check
                  label="Track expiry"
                  on={t.tracks_expiry}
                  disabled={!editable}
                  onChange={(on) => change(t, { tracks_expiry: on })}
                />
              </Cell>
              <Cell>
                <Check
                  label="Expiry date required"
                  on={t.expiry_required}
                  disabled={!editable || !t.tracks_expiry}
                  onChange={(on) => change(t, { expiry_required: on })}
                />
              </Cell>
              <Cell>
                <Check
                  label="Hold when expired"
                  on={t.blocks_when_expired}
                  disabled={!editable || !t.tracks_expiry}
                  onChange={(on) => change(t, { blocks_when_expired: on })}
                />
              </Cell>
            </Row>
          ))}
        </Table>
      )}
      <p className="mt-3 text-xs text-muted">
        Mandatory documents block sign-up. Making a document mandatory does not suspend drivers already
        approved; they appear under Missing.
      </p>
      <ReasonDialog
        open={!!pending}
        title={pending ? `Save "${pending.label.en}" for ${pending.country}?` : ""}
        body={pending ? summary(pending) : ""}
        confirm="Save"
        busy={save.isPending}
        onClose={() => setPending(undefined)}
        onConfirm={(reason) =>
          pending && save.mutate({ ...pending, reason }, { onSuccess: () => setPending(undefined) })
        }
      />
    </Card>
  );
}

function summary(t: DocumentType): string {
  const parts: string[] = [REQUIREMENT_LABEL[t.requirement]];
  if (t.tracks_expiry) parts.push(t.expiry_required ? "expiry date required" : "expiry date optional");
  if (t.blocks_when_expired) parts.push("an expired copy holds the driver");
  return `${parts.join(", ")}. Recorded in the audit log.`;
}

function Check({
  label,
  on,
  disabled,
  onChange,
}: {
  label: string;
  on: boolean;
  disabled: boolean;
  onChange: (on: boolean) => void;
}) {
  return (
    <input
      type="checkbox"
      aria-label={label}
      className="size-4 accent-[var(--brand)]"
      checked={on}
      disabled={disabled}
      onChange={(e) => onChange(e.target.checked)}
    />
  );
}

/** A new document kind for the country. */
function NewType({
  country,
  onCancel,
  onSave,
}: {
  country: string;
  onCancel: () => void;
  onSave: (t: DocumentType) => void;
}) {
  const [t, setT] = useState<DocumentType>({
    country,
    kind: "",
    owner: "DRIVER",
    label: { en: "", sw: "" },
    requirement: "OPTIONAL",
    tracks_expiry: false,
    expiry_required: false,
    blocks_when_expired: false,
    sort_order: 100,
  });
  const kindOk = /^[A-Z][A-Z0-9_]{1,31}$/.test(t.kind);
  return (
    <div className="mb-4 grid gap-3 rounded-[10px] bg-surface-2 p-4 md:grid-cols-4">
      <Field label="Name (English)">
        <Input
          value={t.label.en}
          onChange={(e) => setT({ ...t, label: { ...t.label, en: e.target.value }, kind: t.kind || "" })}
          placeholder="Medical certificate"
        />
      </Field>
      <Field label="Name (Swahili)">
        <Input
          value={t.label.sw ?? ""}
          onChange={(e) => setT({ ...t, label: { ...t.label, sw: e.target.value } })}
          placeholder="Cheti cha afya"
        />
      </Field>
      <Field label="Code" hint="Capitals and _, e.g. MEDICAL">
        <Input
          value={t.kind}
          onChange={(e) => setT({ ...t, kind: e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, "") })}
        />
      </Field>
      <Field label="For">
        <Select
          value={t.owner}
          onChange={(e) => setT({ ...t, owner: e.target.value as DocumentType["owner"] })}
        >
          <option value="DRIVER">Driver</option>
          <option value="VEHICLE">Vehicle</option>
        </Select>
      </Field>
      <div className="flex justify-end gap-2 md:col-span-4">
        <Button variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
        <Button disabled={!kindOk || !t.label.en.trim()} onClick={() => onSave(t)}>
          Add document
        </Button>
      </div>
    </div>
  );
}
