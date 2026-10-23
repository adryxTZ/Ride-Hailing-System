"use client";
/**
 * RolesScreen — staff roles as named sets of permissions. Create a role, tick
 * what it may do, and grant it on a staff member's page. Super Admin is built in.
 */
import { Lock, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Field, Input } from "@/components/ui/Field";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import type { Schemas } from "@/lib/api";
import { useDeleteRole, useRoles, useSaveRole } from "./hooks";

type Draft = { id?: string; name: string; description: string; permissions: string[] };

export function RolesScreen() {
  const { data, isLoading, error } = useRoles();
  const save = useSaveRole();
  const del = useDeleteRole();
  const [draft, setDraft] = useState<Draft>();
  const [asking, setAsking] = useState<"save" | "delete">();
  const label = (key: string) =>
    key === "*" ? "Everything" : (data?.permissions.find((p) => p.key === key)?.label ?? key);
  return (
    <div>
      <PageHeader
        title="Roles"
        subtitle="A role is a set of permissions. Changes reach people holding the role at their next sign-in refresh (within 15 minutes)."
        actions={
          !draft ? (
            <Button
              icon={<Plus size={15} aria-hidden="true" />}
              onClick={() => setDraft({ name: "", description: "", permissions: [] })}
            >
              New role
            </Button>
          ) : null
        }
      />
      <ErrorNote error={error ?? save.error ?? del.error} />
      {draft && data ? (
        <RoleForm
          draft={draft}
          permissions={data.permissions}
          onChange={setDraft}
          onCancel={() => setDraft(undefined)}
          onSave={() => setAsking("save")}
          onDelete={draft.id ? () => setAsking("delete") : undefined}
        />
      ) : null}
      {isLoading || !data ? (
        <Skeleton className="h-64" />
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {data.data.map((r, i) => (
            <Card
              key={r.id}
              index={i + 1}
              title={r.name}
              action={
                r.system ? (
                  <span className="inline-flex items-center gap-1 text-xs text-muted">
                    <Lock size={12} aria-hidden="true" /> Built in
                  </span>
                ) : (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() =>
                      setDraft({
                        id: r.id,
                        name: r.name,
                        description: r.description,
                        permissions: [...r.permissions],
                      })
                    }
                  >
                    Edit
                  </Button>
                )
              }
            >
              <p className="text-sm text-muted">{r.description || "No description"}</p>
              <p className="tnum mt-2 text-xs text-muted">{`${r.members} ${r.members === 1 ? "person" : "people"}`}</p>
              <div className="mt-3 flex flex-wrap gap-1.5">
                {r.permissions.map((p) => (
                  <Chip key={p} wrap tone={p === "*" ? "brand" : "muted"}>
                    {label(p)}
                  </Chip>
                ))}
              </div>
            </Card>
          ))}
        </div>
      )}
      <ReasonDialog
        open={asking === "save"}
        title={draft?.id ? `Save changes to ${draft.name}?` : `Create the role ${draft?.name ?? ""}?`}
        body="Recorded in the audit log."
        confirm="Save"
        busy={save.isPending}
        onClose={() => setAsking(undefined)}
        onConfirm={(reason) =>
          draft &&
          save.mutate(
            {
              id: draft.id,
              body: {
                name: draft.name,
                description: draft.description,
                permissions: draft.permissions,
                reason,
              } as Schemas["RoleInput"],
            },
            {
              onSuccess: () => {
                setAsking(undefined);
                setDraft(undefined);
              },
            },
          )
        }
      />
      <ReasonDialog
        open={asking === "delete"}
        title={`Delete the role ${draft?.name ?? ""}?`}
        body="Only roles nobody holds can be deleted."
        confirm="Delete"
        danger
        busy={del.isPending}
        onClose={() => setAsking(undefined)}
        onConfirm={(reason) =>
          draft?.id &&
          del.mutate(
            { id: draft.id, reason },
            {
              onSuccess: () => {
                setAsking(undefined);
                setDraft(undefined);
              },
            },
          )
        }
      />
    </div>
  );
}

function RoleForm({
  draft,
  permissions,
  onChange,
  onCancel,
  onSave,
  onDelete,
}: {
  draft: Draft;
  permissions: Schemas["Permission"][];
  onChange: (d: Draft) => void;
  onCancel: () => void;
  onSave: () => void;
  onDelete?: () => void;
}) {
  const groups = [...new Set(permissions.map((p) => p.group))];
  const toggle = (key: string) =>
    onChange({
      ...draft,
      permissions: draft.permissions.includes(key)
        ? draft.permissions.filter((p) => p !== key)
        : [...draft.permissions, key],
    });
  return (
    <Card index={0} title={draft.id ? `Edit ${draft.name}` : "New role"}>
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Name">
          <Input
            value={draft.name}
            onChange={(e) => onChange({ ...draft, name: e.target.value })}
            placeholder="Regional manager"
          />
        </Field>
        <Field label="What it is for">
          <Input
            value={draft.description}
            onChange={(e) => onChange({ ...draft, description: e.target.value })}
          />
        </Field>
      </div>
      <div className="mt-4 grid gap-4 md:grid-cols-2">
        {groups.map((g) => (
          <fieldset key={g} className="grid gap-2">
            <legend className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted">{g}</legend>
            {permissions
              .filter((p) => p.group === g)
              .map((p) => (
                <label
                  key={p.key}
                  className="flex cursor-pointer items-start gap-2.5 rounded-[8px] px-2 py-1.5 text-sm hover:bg-surface-2"
                >
                  <input
                    type="checkbox"
                    className="mt-0.5 size-4 accent-[var(--brand)]"
                    checked={draft.permissions.includes(p.key)}
                    onChange={() => toggle(p.key)}
                  />
                  <span>{p.label}</span>
                </label>
              ))}
          </fieldset>
        ))}
      </div>
      <div className="mt-4 flex justify-between gap-2">
        {onDelete ? (
          <Button variant="ghost" icon={<Trash2 size={15} aria-hidden="true" />} onClick={onDelete}>
            Delete role
          </Button>
        ) : (
          <span />
        )}
        <div className="flex gap-2">
          <Button variant="ghost" onClick={onCancel}>
            Cancel
          </Button>
          <Button disabled={!draft.name.trim()} onClick={onSave}>
            {draft.id ? "Save role" : "Create role"}
          </Button>
        </div>
      </div>
    </Card>
  );
}
