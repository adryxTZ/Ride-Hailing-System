"use client";
/**
 * StaffDetail — one staff member: who they are, their roles and what those let
 * them do, the countries and regions they are limited to, their sign-in sessions
 * (with a map), what they did, and the controls to end sessions or deactivate.
 */
import {
  ArrowLeft,
  Check,
  Globe2,
  KeyRound,
  LogOut,
  Monitor,
  Plus,
  Power,
  ShieldCheck,
  Trash2,
} from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Select } from "@/components/ui/Field";
import { Initials, MetricTile, OperationsHeader } from "@/components/ui/Operations";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { Tabs } from "@/components/ui/Tabs";
import { SignInMap } from "@/features/audit/SignInMap";
import { placeText, toSignIn } from "@/features/audit/signIns";
import { SessionsTable } from "@/features/people/SessionsTable";
import type { Schemas } from "@/lib/api";
import { when } from "@/lib/format";
import { useAreas, useCountries } from "@/lib/scope";
import { claims } from "@/lib/session";
import {
  useEndSessions,
  usePersonAudit,
  useRoles,
  useSetAccess,
  useSetActive,
  useSetStaff,
  useStaffMember,
} from "./hooks";
import { RolePicker } from "./RolePicker";

const TABS = [
  { value: "sessions", label: "Sessions" },
  { value: "activity", label: "Activity" },
  { value: "signins", label: "Sign-ins" },
];

type Ask = { kind: "end"; sessionId?: string } | { kind: "status" } | { kind: "roles" } | { kind: "access" };

export function StaffDetail({ id }: { id: string }) {
  const { data: p, isLoading, error } = useStaffMember(id);
  const { data: roles } = useRoles();
  const end = useEndSessions();
  const active = useSetActive();
  const setStaff = useSetStaff();
  const setAccess = useSetAccess();
  const [tab, setTab] = useState("sessions");
  const [ask, setAsk] = useState<Ask>();
  const [roleDraft, setRoleDraft] = useState<string[]>();
  const [accessDraft, setAccessDraft] = useState<Schemas["StaffAccess"][]>();
  const isMe = claims().sub === id;
  const label = (key: string) =>
    key === "*" ? "Everything" : (roles?.permissions.find((x) => x.key === key)?.label ?? key);
  const disabled = p?.status !== "ACTIVE";
  return (
    <div>
      <OperationsHeader
        title={p?.full_name ?? "Staff member"}
        description={
          p
            ? [p.email, p.phone].filter(Boolean).join(" · ")
            : "Account details, permissions and sign-in history."
        }
        actions={
          <Link
            href="/staff"
            className="inline-flex items-center gap-1.5 text-sm font-semibold text-brand-text"
          >
            <ArrowLeft size={15} aria-hidden="true" /> All staff
          </Link>
        }
      />
      <ErrorNote error={error ?? end.error ?? active.error ?? setStaff.error ?? setAccess.error} />
      {isLoading ? (
        <Skeleton className="h-96" />
      ) : !p ? (
        <Empty title="Staff profile unavailable" body="Return to all staff to find this account." />
      ) : (
        <div className="grid gap-5">
          <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
            <MetricTile
              label="Assigned roles"
              value={String(p.staff_roles.length)}
              note="Roles controlling dashboard access"
              icon={<ShieldCheck size={17} />}
              accent
            />
            <MetricTile
              label="Permissions"
              value={p.permissions.includes("*") ? "All" : String(p.permissions.length)}
              note="Actions granted by assigned roles"
              icon={<KeyRound size={17} />}
            />
            <MetricTile
              label="Open sessions"
              value={String(p.sessions.filter((session) => session.active).length)}
              note="Currently active sign-in sessions"
              icon={<Monitor size={17} />}
            />
            <MetricTile
              label="Work locations"
              value={p.access.length ? "Restricted" : "All regions"}
              note={
                p.access.length
                  ? `${p.access.length} country or region access rules`
                  : "Access across every country and region"
              }
              icon={<Globe2 size={17} />}
            />
          </div>
          <Card index={1} title="Account status">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex flex-wrap items-center gap-2 text-sm">
                <Initials name={p.full_name} />
                {disabled ? <Chip tone="bad">Deactivated</Chip> : <Chip tone="ok">Active</Chip>}
                {isMe ? <Chip tone="muted">Your account</Chip> : null}
                <span className="text-muted">
                  {p.last_sign_in ? `Last sign-in ${when(p.last_sign_in)}` : "Never signed in"}
                </span>
                <span className="text-muted">{`· ${p.sessions.filter((s) => s.active).length} open sessions`}</span>
              </div>
              <div className="flex gap-2">
                <Button
                  variant="soft"
                  icon={<LogOut size={15} aria-hidden="true" />}
                  disabled={!p.sessions.some((s) => s.active)}
                  onClick={() => setAsk({ kind: "end" })}
                >
                  End all sessions
                </Button>
                {!isMe ? (
                  <Button
                    variant={disabled ? "primary" : "danger"}
                    icon={<Power size={15} aria-hidden="true" />}
                    onClick={() => setAsk({ kind: "status" })}
                  >
                    {disabled ? "Reactivate" : "Deactivate"}
                  </Button>
                ) : null}
              </div>
            </div>
          </Card>
          <div className="grid items-start gap-5 xl:grid-cols-2">
            <Card
              index={2}
              title="Roles and permissions"
              action={
                roleDraft ? null : (
                  <Button size="sm" variant="ghost" onClick={() => setRoleDraft([...p.staff_roles])}>
                    Change roles
                  </Button>
                )
              }
            >
              {roleDraft ? (
                <div className="grid gap-3">
                  <RolePicker
                    roles={(roles?.data ?? []).map((r) => r.name)}
                    value={roleDraft}
                    onChange={setRoleDraft}
                  />
                  <div className="flex justify-end gap-2">
                    <Button variant="ghost" onClick={() => setRoleDraft(undefined)}>
                      Cancel
                    </Button>
                    <Button onClick={() => setAsk({ kind: "roles" })}>Save roles</Button>
                  </div>
                </div>
              ) : (
                <>
                  <div className="flex flex-wrap gap-1.5">
                    {p.staff_roles.length ? (
                      p.staff_roles.map((r) => (
                        <Chip key={r} tone="brand">
                          {r}
                        </Chip>
                      ))
                    ) : (
                      <span className="text-sm text-muted">No roles: no dashboard access</span>
                    )}
                  </div>
                  <p className="mb-2 mt-4 text-xs font-semibold uppercase tracking-wide text-muted">Can do</p>
                  <ul className="grid gap-2 text-xs sm:grid-cols-2">
                    {p.permissions.map((x) => (
                      <li key={x} className="flex items-start gap-2 rounded-lg bg-surface-2 px-3 py-2.5">
                        <Check size={13} className="mt-0.5 shrink-0 text-brand-text" aria-hidden="true" />
                        <span className="min-w-0 break-words">{label(x)}</span>
                      </li>
                    ))}
                  </ul>
                </>
              )}
            </Card>
            <AccessCard
              access={accessDraft ?? p.access}
              editing={!!accessDraft}
              onEdit={() => setAccessDraft([...p.access])}
              onChange={setAccessDraft}
              onCancel={() => setAccessDraft(undefined)}
              onSave={() => setAsk({ kind: "access" })}
            />
          </div>
          <Card
            index={4}
            action={<Tabs id="staff-tab" value={tab} options={TABS} onChange={setTab} />}
            title="History"
          >
            <p className="mb-4 text-xs leading-relaxed text-muted">
              Review active sessions, account activity and sign-in history.
            </p>
            {tab === "sessions" ? (
              <SessionsTable
                sessions={p.sessions}
                onEnd={(sessionId) => setAsk({ kind: "end", sessionId })}
              />
            ) : tab === "activity" ? (
              <ActivityView id={id} />
            ) : (
              <SignInsView id={id} />
            )}
          </Card>
        </div>
      )}
      <ReasonDialog
        open={ask?.kind === "end"}
        title={
          ask?.kind === "end" && ask.sessionId
            ? "End this session?"
            : `End every session of ${p?.full_name ?? "this person"}?`
        }
        body="They are signed out on their next action and must sign in again."
        confirm="End"
        danger
        busy={end.isPending}
        onClose={() => setAsk(undefined)}
        onConfirm={(reason) =>
          ask?.kind === "end" &&
          end.mutate({ id, sessionId: ask.sessionId, reason }, { onSuccess: () => setAsk(undefined) })
        }
      />
      <ReasonDialog
        open={ask?.kind === "status"}
        title={disabled ? `Reactivate ${p?.full_name ?? ""}?` : `Deactivate ${p?.full_name ?? ""}?`}
        body={
          disabled
            ? "They can sign in again."
            : "They are signed out everywhere at once and cannot sign in until reactivated."
        }
        confirm={disabled ? "Reactivate" : "Deactivate"}
        danger={!disabled}
        busy={active.isPending}
        onClose={() => setAsk(undefined)}
        onConfirm={(reason) =>
          active.mutate({ id, active: disabled, reason }, { onSuccess: () => setAsk(undefined) })
        }
      />
      <ReasonDialog
        open={ask?.kind === "roles"}
        title="Save these roles?"
        body="They get the new permissions at their next sign-in refresh (within 15 minutes)."
        confirm="Save"
        busy={setStaff.isPending}
        onClose={() => setAsk(undefined)}
        onConfirm={(reason) =>
          p &&
          roleDraft &&
          setStaff.mutate(
            { phone: p.phone, full_name: p.full_name ?? "", roles: roleDraft, reason },
            {
              onSuccess: () => {
                setAsk(undefined);
                setRoleDraft(undefined);
              },
            },
          )
        }
      />
      <ReasonDialog
        open={ask?.kind === "access"}
        title="Save where this person may work?"
        body="Applies at their next sign-in refresh (within 15 minutes). No limits means every country and region."
        confirm="Save"
        busy={setAccess.isPending}
        onClose={() => setAsk(undefined)}
        onConfirm={(reason) =>
          accessDraft &&
          setAccess.mutate(
            { id, access: accessDraft, reason },
            {
              onSuccess: () => {
                setAsk(undefined);
                setAccessDraft(undefined);
              },
            },
          )
        }
      />
    </div>
  );
}

/** Countries and regions the person is limited to (none = everywhere). */
function AccessCard({
  access,
  editing,
  onEdit,
  onChange,
  onCancel,
  onSave,
}: {
  access: Schemas["StaffAccess"][];
  editing: boolean;
  onEdit: () => void;
  onChange: (a: Schemas["StaffAccess"][]) => void;
  onCancel: () => void;
  onSave: () => void;
}) {
  const { data: countries } = useCountries();
  const name = (code: string) => countries?.find((c) => c.code === code)?.name?.en ?? code;
  return (
    <Card
      index={3}
      title="Where they may work"
      action={
        editing ? null : (
          <Button size="sm" variant="ghost" onClick={onEdit}>
            Change
          </Button>
        )
      }
    >
      {!editing ? (
        access.length ? (
          <ul className="grid gap-2 text-sm">
            {access.map((a) => (
              <li key={`${a.country}/${a.area_id ?? ""}`}>
                <AccessLabel a={a} countryName={name(a.country)} />
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-sm text-muted">Every country and region.</p>
        )
      ) : (
        <div className="grid gap-2">
          {access.map((a, i) => (
            <AccessRow
              key={i}
              value={a}
              onChange={(v) => onChange(access.map((x, j) => (j === i ? v : x)))}
              onRemove={() => onChange(access.filter((_, j) => j !== i))}
            />
          ))}
          <Button
            size="sm"
            variant="soft"
            icon={<Plus size={14} aria-hidden="true" />}
            onClick={() => onChange([...access, { country: countries?.[0]?.code ?? "TZ" }])}
          >
            Add a country or region
          </Button>
          <p className="text-xs text-muted">Leave the list empty for every country and region.</p>
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={onCancel}>
              Cancel
            </Button>
            <Button onClick={onSave}>Save</Button>
          </div>
        </div>
      )}
    </Card>
  );
}

function AccessLabel({ a, countryName }: { a: Schemas["StaffAccess"]; countryName: string }) {
  const { data: areas } = useAreas(a.country);
  const area = areas?.find((x) => x.id === a.area_id);
  return (
    <span>
      {a.area_id ? `${countryName} · ${area?.name ?? "a region"}` : `${countryName} (whole country)`}
    </span>
  );
}

function AccessRow({
  value,
  onChange,
  onRemove,
}: {
  value: Schemas["StaffAccess"];
  onChange: (v: Schemas["StaffAccess"]) => void;
  onRemove: () => void;
}) {
  const { data: countries } = useCountries();
  const { data: areas } = useAreas(value.country);
  return (
    <div className="flex min-w-0 items-center gap-2">
      <Select
        aria-label="Country"
        value={value.country}
        onChange={(e) => onChange({ country: e.target.value })}
        className="h-9 min-w-0 flex-1"
      >
        {(countries ?? []).map((c) => (
          <option key={c.code} value={c.code}>
            {c.name?.en ?? c.code}
          </option>
        ))}
      </Select>
      <Select
        aria-label="Region"
        value={value.area_id ?? ""}
        onChange={(e) => onChange({ country: value.country, area_id: e.target.value || undefined })}
        className="h-9 min-w-0 flex-1"
      >
        <option value="">Whole country</option>
        {(areas ?? []).map((a) => (
          <option key={a.id} value={a.id}>
            {a.name}
          </option>
        ))}
      </Select>
      <button
        type="button"
        aria-label="Remove"
        onClick={onRemove}
        className="grid size-9 shrink-0 place-items-center rounded-[8px] text-muted hover:bg-surface-2"
      >
        <Trash2 size={15} aria-hidden="true" />
      </button>
    </div>
  );
}

function ActivityView({ id }: { id: string }) {
  const { data, isLoading } = usePersonAudit(id, "actor");
  if (isLoading) return <Skeleton className="h-40" />;
  if (!data?.length) return <Empty title="No activity yet" body="Decisions this person makes appear here." />;
  return (
    <Table head={["When", "What", "Record", "Reason"]}>
      {data.map((e) => (
        <Row key={e.id}>
          <Cell className="tnum whitespace-nowrap text-muted">{when(e.at)}</Cell>
          <Cell>{e.action.replace(/[._]/g, " ")}</Cell>
          <Cell className="max-w-[260px] truncate font-mono text-xs text-muted">{e.target}</Cell>
          <Cell className="text-muted">{e.reason ?? ""}</Cell>
        </Row>
      ))}
    </Table>
  );
}

function SignInsView({ id }: { id: string }) {
  const { data, isLoading } = usePersonAudit(id, "signins");
  const [selected, setSelected] = useState<number>();
  if (isLoading) return <Skeleton className="h-40" />;
  const signIns = (data ?? []).map(toSignIn);
  if (!signIns.length) return <Empty title="No sign-ins recorded" />;
  return (
    <div className="grid gap-4">
      {signIns.some((s) => s.location) ? (
        <SignInMap signIns={signIns} selectedId={selected} onSelect={setSelected} />
      ) : null}
      <Table head={["When", "Outcome", "Device", "Network address", "Location"]}>
        {signIns.map((s) => (
          <Row key={s.id} onClick={() => setSelected(s.id)} selected={selected === s.id}>
            <Cell className="tnum whitespace-nowrap text-muted">{s.when}</Cell>
            <Cell>{s.failed ? <Chip tone="bad">Failed</Chip> : <Chip tone="ok">Signed in</Chip>}</Cell>
            <Cell>{s.device}</Cell>
            <Cell className="font-mono text-xs text-muted">{s.ip}</Cell>
            <Cell className="font-mono text-xs text-muted">{placeText(s.location)}</Cell>
          </Row>
        ))}
      </Table>
    </div>
  );
}
