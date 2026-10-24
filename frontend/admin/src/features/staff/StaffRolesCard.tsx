"use client";
/**
 * StaffRolesCard — on a rider's or driver's page: the dashboard roles the
 * same person holds (any number), and for staff managers a way to give or
 * change them. Their rider or driver account stays as it is.
 */
import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import { useMe } from "@/features/me";
import { useSetStaff, useStaff } from "./hooks";
import { RolePicker } from "./RolePicker";

export function StaffRolesCard({
  phone,
  name,
  index,
}: {
  phone: string;
  name?: string | null;
  index: number;
}) {
  if (!useMe().can("staff.manage")) return null;
  return <RolesCard phone={phone} name={name} index={index} />;
}

function RolesCard({ phone, name, index }: { phone: string; name?: string | null; index: number }) {
  const { data, isLoading } = useStaff();
  const save = useSetStaff();
  const [draft, setDraft] = useState<string[]>();
  const [asking, setAsking] = useState(false);
  const row = data?.data.find((s) => s.phone === phone);
  const current = row?.staff_roles ?? [];
  return (
    <Card
      index={index}
      title="Dashboard roles"
      action={
        draft ? null : (
          <Button size="sm" variant="ghost" onClick={() => setDraft([...current])}>
            {current.length ? "Change roles" : "Give dashboard roles"}
          </Button>
        )
      }
    >
      {isLoading ? (
        <Skeleton className="h-10" />
      ) : draft ? (
        <div className="grid gap-3">
          <RolePicker roles={data?.roles ?? []} value={draft} onChange={setDraft} />
          <p className="text-xs text-muted">
            They sign in to this dashboard with an email and password set on the Staff page.
          </p>
          <ErrorNote error={save.error} />
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setDraft(undefined)}>
              Cancel
            </Button>
            <Button onClick={() => setAsking(true)}>Save roles</Button>
          </div>
        </div>
      ) : current.length ? (
        <div className="flex flex-wrap items-center gap-1.5">
          {current.map((r) => (
            <Chip key={r} tone="brand">
              {r}
            </Chip>
          ))}
          {row ? (
            <Link href={`/staff/${row.id}`} className="ml-2 text-xs font-semibold text-brand-text">
              Staff page
            </Link>
          ) : null}
        </div>
      ) : (
        <p className="text-sm text-muted">No dashboard access.</p>
      )}
      <ReasonDialog
        open={asking}
        title={draft?.length ? `Save roles for ${name ?? phone}?` : "Remove dashboard access?"}
        body="They get the new permissions at their next sign-in refresh. Recorded in the audit log."
        confirm="Save"
        danger={!draft?.length}
        busy={save.isPending}
        onClose={() => setAsking(false)}
        onConfirm={(reason) =>
          draft &&
          save.mutate(
            { phone, full_name: name ?? "", roles: draft, reason },
            {
              onSuccess: () => {
                setAsking(false);
                setDraft(undefined);
              },
            },
          )
        }
      />
    </Card>
  );
}
