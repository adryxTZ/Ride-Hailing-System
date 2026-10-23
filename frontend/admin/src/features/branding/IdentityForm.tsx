"use client";
/** IdentityForm — the brand's name and contacts; saving asks for a reason (audited). */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { Field, Input } from "@/components/ui/Field";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { ErrorNote } from "@/components/ui/States";
import type { Schemas } from "@/lib/api";
import { when } from "@/lib/format";
import { useSaveIdentity } from "./hooks";

type Identity = Schemas["BrandIdentity"];
const FIELDS: { key: keyof Omit<Identity, "edited_at">; label: string; hint?: string; type?: string }[] = [
  { key: "name", label: "Brand name", hint: "Shown in the apps, receipts, messages and the dashboard." },
  { key: "legal_entity", label: "Legal entity", hint: "The company name printed on receipts." },
  { key: "support_phone", label: "Support phone", type: "tel" },
  { key: "support_email", label: "Support email", type: "email" },
  { key: "emergency_phone", label: "Emergency number", hint: "Offered on the Safety screen.", type: "tel" },
];

export function IdentityForm({ identity }: { identity: Identity }) {
  const [form, setForm] = useState<Identity>(identity);
  const [asking, setAsking] = useState(false);
  const save = useSaveIdentity();
  const changed = FIELDS.some((f) => (form[f.key] ?? "") !== (identity[f.key] ?? ""));
  return (
    <div className="grid gap-4">
      <ErrorNote error={save.error} />
      <div className="flex items-center gap-2 text-sm text-muted">
        {identity.edited_at ? (
          <span>{`Last changed ${when(identity.edited_at)}`}</span>
        ) : (
          <Chip tone="muted">From the brand file</Chip>
        )}
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        {FIELDS.map((f) => (
          <Field key={f.key} label={f.label} hint={f.hint}>
            <Input
              type={f.type ?? "text"}
              value={form[f.key] ?? ""}
              onChange={(e) => setForm({ ...form, [f.key]: e.target.value })}
            />
          </Field>
        ))}
      </div>
      <p className="text-xs text-muted">
        The name under the app icon on phones changes with the next app release.
      </p>
      <div className="flex justify-end gap-2">
        <Button variant="ghost" disabled={!changed} onClick={() => setForm(identity)}>
          Undo changes
        </Button>
        <Button disabled={!changed || !form.name.trim()} onClick={() => setAsking(true)}>
          Save identity
        </Button>
      </div>
      <ReasonDialog
        open={asking}
        title="Save the brand identity?"
        body="Riders and drivers see the new name and contacts the next time their app loads its settings."
        confirm="Save"
        busy={save.isPending}
        onClose={() => setAsking(false)}
        onConfirm={(reason) => save.mutate({ ...form, reason }, { onSuccess: () => setAsking(false) })}
      />
    </div>
  );
}
