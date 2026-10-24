"use client";
/**
 * DetailsForm — the values the legal texts' placeholders need that are not
 * brand data (registered address, data protection contact ...). Saving asks
 * for a reason (audited); the public pages show the new values within minutes.
 */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Field, Input } from "@/components/ui/Field";
import { Portal } from "@/components/ui/Portal";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { ErrorNote } from "@/components/ui/States";
import type { Schemas } from "@/lib/api";
import { when } from "@/lib/format";
import { type Placeholder, type Texts, useSaveDetails } from "./hooks";

/** Keys whose field takes an e-mail or a number (the browser's own checks). */
const INPUT_TYPE: Record<string, string> = { dpo_email: "email", minimum_age: "number" };

export function DetailsForm({
  details,
  placeholders,
}: {
  details: Schemas["LegalDetails"];
  placeholders: Placeholder[];
}) {
  const editable = placeholders.filter((p) => p.source === "DETAILS");
  const [form, setForm] = useState<Texts>(details.fields);
  const [asking, setAsking] = useState(false);
  const save = useSaveDetails();
  const changed = editable.some((p) => (form[p.key] ?? "") !== (details.fields[p.key] ?? ""));
  return (
    <div className="grid gap-4">
      <p className="text-xs text-muted">
        {details.updated_at ? `Last changed ${when(details.updated_at)}. ` : ""}
        The brand name and support contacts come from the Branding page.
      </p>
      <ErrorNote error={save.error} />
      <div className="grid gap-3 md:grid-cols-2">
        {editable.map((p) => (
          <Field key={p.key} label={p.label} hint={hintFor(p, form[p.key])}>
            <Input
              type={INPUT_TYPE[p.key] ?? "text"}
              value={form[p.key] ?? ""}
              onChange={(e) => setForm({ ...form, [p.key]: e.target.value })}
            />
          </Field>
        ))}
      </div>
      <div className="flex justify-end gap-2">
        <Button variant="ghost" disabled={!changed} onClick={() => setForm(details.fields)}>
          Undo changes
        </Button>
        <Button disabled={!changed} onClick={() => setAsking(true)}>
          Save details
        </Button>
      </div>
      <Portal>
        <ReasonDialog
          open={asking}
          title="Save the legal details?"
          body="The published texts show the new values within a few minutes. Nobody is asked to accept again."
          confirm="Save"
          busy={save.isPending}
          onClose={() => setAsking(false)}
          onConfirm={(reason) => save.mutate({ fields: form, reason }, { onSuccess: () => setAsking(false) })}
        />
      </Portal>
    </div>
  );
}

/** The field's hint, and what the texts show while it is empty. */
function hintFor(p: Placeholder, typed?: string): string {
  const fallback = !typed && p.value ? ` Now shows: ${p.value}.` : "";
  return `{{${p.key}}}. ${p.hint ?? ""}${fallback}`.trim();
}
