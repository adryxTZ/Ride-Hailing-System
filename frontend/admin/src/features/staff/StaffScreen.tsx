"use client";
/** StaffScreen — who can use the dashboard and with which roles; only a Super Admin can change roles. */
import { UserPlus } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Field, Input } from "@/components/ui/Field";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { useMe } from "@/features/me";
import type { Schemas } from "@/lib/api";
import { useSetStaff, useStaff } from "./hooks";

type Draft = Schemas["StaffInput"];

export function StaffScreen() {
  const router = useRouter();
  const { data, isLoading, error } = useStaff();
  const save = useSetStaff();
  const me = useMe();
  const admin = me.can("staff.manage");
  const [draft, setDraft] = useState<Draft>();
  const [confirming, setConfirming] = useState(false);
  const toggle = (role: string) =>
    draft &&
    setDraft({
      ...draft,
      roles: draft.roles.includes(role) ? draft.roles.filter((r) => r !== role) : [...draft.roles, role],
    });
  return (
    <div>
      <PageHeader
        title="Staff"
        subtitle="Dashboard accounts and their roles. Removing every role removes access."
        actions={
          admin && !draft ? (
            <Button
              icon={<UserPlus size={15} aria-hidden="true" />}
              onClick={() => setDraft({ phone: "", full_name: "", email: "", password: "", roles: [] })}
            >
              Add staff
            </Button>
          ) : null
        }
      />
      <ErrorNote error={error ?? save.error} />
      {draft ? (
        <Card index={1} title={draft.phone ? `Roles for ${draft.full_name || draft.phone}` : "Add staff"}>
          <div className="grid gap-3 md:grid-cols-2">
            <Field label="Phone">
              <Input
                value={draft.phone}
                onChange={(e) => setDraft({ ...draft, phone: e.target.value })}
                placeholder="+255…"
              />
            </Field>
            <Field label="Full name">
              <Input
                value={draft.full_name ?? ""}
                onChange={(e) => setDraft({ ...draft, full_name: e.target.value })}
              />
            </Field>
            <Field label="Email" hint="Used to sign in to this dashboard">
              <Input
                type="email"
                value={draft.email ?? ""}
                onChange={(e) => setDraft({ ...draft, email: e.target.value })}
              />
            </Field>
            <Field label="Password" hint="At least 6 characters; leave empty to keep the current one">
              <Input
                type="password"
                autoComplete="new-password"
                value={draft.password ?? ""}
                onChange={(e) => setDraft({ ...draft, password: e.target.value })}
              />
            </Field>
          </div>
          <div className="mt-4 flex flex-wrap gap-2" role="group" aria-label="Roles">
            {(data?.roles ?? []).map((r) => (
              <button
                key={r}
                type="button"
                aria-pressed={draft.roles.includes(r)}
                onClick={() => toggle(r)}
                className={`rounded-full px-3.5 py-1.5 text-sm font-medium transition-colors ${draft.roles.includes(r) ? "bg-ink text-canvas" : "bg-surface-2 text-muted hover:text-ink"}`}
              >
                {r}
              </button>
            ))}
          </div>
          <div className="mt-4 flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setDraft(undefined)}>
              Cancel
            </Button>
            <Button disabled={!draft.phone} onClick={() => setConfirming(true)}>
              Save roles
            </Button>
          </div>
        </Card>
      ) : null}
      <div className="mt-4">
        <Card index={2}>
          {isLoading ? (
            <Skeleton className="h-64" />
          ) : !data?.data.length ? (
            <Empty title="No staff yet" body="Add the first account with the staff command on the server." />
          ) : (
            <Table head={["Name", "Phone", "Email", "Roles", ""]}>
              {data.data.map((s) => (
                <Row key={s.id} onClick={() => router.push(`/staff/${s.id}`)}>
                  <Cell className="font-medium">{s.full_name ?? "No name"}</Cell>
                  <Cell className="tnum text-muted">{s.phone}</Cell>
                  <Cell className="text-muted">{s.email ?? "No email yet"}</Cell>
                  <Cell>
                    <div className="flex flex-wrap gap-1.5">
                      {s.staff_roles.map((r) => (
                        <Chip key={r} tone="brand">
                          {r}
                        </Chip>
                      ))}
                    </div>
                  </Cell>
                  <Cell className="text-right">
                    {admin ? (
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() =>
                          setDraft({
                            phone: s.phone,
                            full_name: s.full_name ?? "",
                            email: s.email ?? "",
                            password: "",
                            roles: [...s.staff_roles],
                          })
                        }
                      >
                        Edit roles
                      </Button>
                    ) : null}
                  </Cell>
                </Row>
              ))}
            </Table>
          )}
        </Card>
      </div>
      <ReasonDialog
        open={confirming}
        title={draft?.roles.length ? "Save these roles?" : "Remove dashboard access?"}
        body="The change applies at their next sign-in and is recorded in the audit log."
        confirm="Save"
        danger={!draft?.roles.length}
        busy={save.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() =>
          draft &&
          save.mutate(draft, {
            onSuccess: () => {
              setConfirming(false);
              setDraft(undefined);
            },
          })
        }
      />
    </div>
  );
}
