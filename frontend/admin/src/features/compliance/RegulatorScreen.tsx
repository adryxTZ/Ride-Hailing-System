"use client";
/**
 * RegulatorScreen — each country's transport regulator: whether submissions are
 * sent, whether unregistered drivers may go online (fail-closed), what is
 * waiting or stuck, and the outbox with a manual retry. Every switch is audited.
 */
import { useQueryClient } from "@tanstack/react-query";
import { CheckCheck, Clock3, Globe2, RefreshCw, ShieldAlert, ShieldCheck } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { MetricTile, OperationsHeader, RefreshButton, UpdateIndicator } from "@/components/ui/Operations";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Search } from "@/components/ui/Search";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { Tabs } from "@/components/ui/Tabs";
import { useMe } from "@/features/me";
import type { Schemas } from "@/lib/api";
import { count, when } from "@/lib/format";
import { useCountries } from "@/lib/scope";
import {
  type SubmissionFilter,
  useRegulatorSettings,
  useRegulatorStatus,
  useRetrySubmission,
  useSubmissions,
} from "./hooks";

const VIEWS = [
  { value: "waiting", label: "Waiting" },
  { value: "stuck", label: "Stuck" },
  { value: "accepted", label: "Accepted" },
  { value: "all", label: "All" },
];
const KIND_LABEL: Record<string, string> = {
  DRIVER: "Driver",
  VEHICLE: "Vehicle",
  TRIP: "Trip",
  DAILY_REPORT: "Daily report",
};

type Switch = { country: string; field: "enabled" | "fail_closed"; on: boolean };

export function RegulatorScreen() {
  const regulator = useRegulatorStatus();
  const { data: statuses, isLoading, error } = regulator;
  const queryClient = useQueryClient();
  const settings = useRegulatorSettings();
  const editable = useMe().can("compliance.manage");
  const [ask, setAsk] = useState<Switch>();
  return (
    <div>
      <OperationsHeader
        title="Regulator"
        description="Monitor transport registrations, trip reports and submission issues across regulated countries."
        actions={
          <>
            <UpdateIndicator
              at={regulator.dataUpdatedAt}
              error={error}
              fetching={regulator.isFetching}
              interval="15s"
            />
            <RefreshButton
              busy={regulator.isFetching}
              onClick={() => void queryClient.invalidateQueries({ queryKey: ["admin", "compliance"] })}
            />
          </>
        }
      />
      <div className="mb-5 grid grid-cols-2 gap-3 xl:grid-cols-4">
        <MetricTile
          label="Regulated countries"
          value={statuses ? count(statuses.length) : error ? "—" : undefined}
          note="Countries with regulator reporting"
          icon={<Globe2 size={17} />}
          accent
        />
        <MetricTile
          label="Waiting to send"
          value={
            statuses
              ? count(statuses.reduce((sum, status) => sum + status.pending, 0))
              : error
                ? "—"
                : undefined
          }
          note="Pending across regulated countries"
          icon={<Clock3 size={17} />}
        />
        <MetricTile
          label="Stuck submissions"
          value={
            statuses
              ? count(statuses.reduce((sum, status) => sum + status.stuck, 0))
              : error
                ? "—"
                : undefined
          }
          note="Submissions needing closer attention"
          icon={<ShieldAlert size={17} />}
        />
        <MetricTile
          label="Accepted today"
          value={
            statuses
              ? count(statuses.reduce((sum, status) => sum + status.accepted_today, 0))
              : error
                ? "—"
                : undefined
          }
          note="Successfully received by regulators"
          icon={<CheckCheck size={17} />}
        />
      </div>
      <ErrorNote error={error ?? settings.error} />
      {isLoading ? (
        <Skeleton className="h-40" />
      ) : !statuses?.length ? (
        <Empty
          title={error ? "Regulator status unavailable" : "No regulated countries"}
          body={
            error
              ? "Use Refresh to try again."
              : "Regulated countries appear here when reporting is configured."
          }
        />
      ) : (
        <div className={`mb-5 grid gap-5 ${statuses.length > 1 ? "xl:grid-cols-2" : ""}`}>
          {statuses.map((s, i) => (
            <CountryCard key={s.country} index={i + 1} status={s} editable={editable} onToggle={setAsk} />
          ))}
        </div>
      )}
      <Submissions />
      <ReasonDialog
        open={!!ask}
        title={ask ? switchTitle(ask) : ""}
        body={
          ask?.field === "fail_closed" && ask.on
            ? "Drivers whose registration is not accepted yet will not be able to go online."
            : "Recorded in the audit log."
        }
        confirm={ask?.on ? "Turn on" : "Turn off"}
        danger={!!ask && ask.field === "fail_closed" && ask.on}
        busy={settings.isPending}
        onClose={() => setAsk(undefined)}
        onConfirm={(reason) =>
          ask &&
          settings.mutate(
            { country: ask.country, [ask.field]: ask.on, reason },
            { onSuccess: () => setAsk(undefined) },
          )
        }
      />
    </div>
  );
}

function switchTitle(s: Switch): string {
  const what = s.field === "enabled" ? "sending to the regulator" : "fail-closed";
  return `Turn ${s.on ? "on" : "off"} ${what} in ${s.country}?`;
}

function CountryCard({
  status: s,
  index,
  editable,
  onToggle,
}: {
  status: Schemas["RegulatorStatus"];
  index: number;
  editable: boolean;
  onToggle: (v: Switch) => void;
}) {
  const { data: countries } = useCountries();
  const countryName = countries?.find((country) => country.code === s.country)?.name?.en ?? s.country;
  return (
    <Card
      index={index}
      title={`${countryName} · ${s.regulator.toUpperCase()}`}
      action={
        <Chip tone={s.adapter === "pending" ? "warn" : s.adapter === "fake" ? "muted" : "ok"}>
          {adapterLabel(s.adapter)}
        </Chip>
      }
    >
      <div className="grid grid-cols-2 gap-4 rounded-lg bg-surface-2/60 p-4 text-sm sm:grid-cols-4">
        <Figure label="Waiting" value={count(s.pending)} />
        <Figure label="Stuck" value={count(s.stuck)} warn={s.stuck > 0} />
        <Figure label="Accepted today" value={count(s.accepted_today)} />
        <Figure label="Nothing to send" value={count(s.skipped_today)} />
      </div>
      {s.last_error ? (
        <p className="mt-3 truncate font-mono text-xs text-critical" title={s.last_error}>
          {s.last_error}
        </p>
      ) : null}
      <p className="mt-4 flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-muted">
        <ShieldCheck size={12} aria-hidden="true" />
        Reporting controls
      </p>
      <div className="mt-3 grid gap-2">
        <Toggle
          label="Send to the regulator"
          on={s.enabled}
          disabled={!editable}
          onChange={(on) => onToggle({ country: s.country, field: "enabled", on })}
        />
        <Toggle
          label="Only registered drivers may go online"
          on={s.fail_closed}
          disabled={!editable || (!s.configured && !s.fail_closed)}
          onChange={(on) => onToggle({ country: s.country, field: "fail_closed", on })}
        />
      </div>
      {!s.configured && !s.fail_closed ? (
        <p className="mt-2 text-[11px] leading-relaxed text-muted">
          Connect the regulator before requiring accepted driver registrations.
        </p>
      ) : null}
      {s.last_accepted_at ? (
        <p className="mt-3 text-xs text-muted">{`Last accepted ${when(s.last_accepted_at)}`}</p>
      ) : null}
    </Card>
  );
}

function adapterLabel(a: string): string {
  return a === "pending" ? "Not connected: queued only" : a === "fake" ? "Test adapter" : "Connected";
}

function Figure({ label, value, warn }: { label: string; value: string; warn?: boolean }) {
  return (
    <div>
      <div className="text-xs text-muted">{label}</div>
      <div className={`tnum text-lg font-semibold ${warn ? "text-critical" : ""}`}>{value}</div>
    </div>
  );
}

function Toggle({
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
    <label
      className={`flex items-center justify-between gap-3 rounded-[8px] bg-surface-2 px-3 py-2 text-sm ${disabled ? "opacity-60" : "cursor-pointer"}`}
    >
      <span>{label}</span>
      <input
        type="checkbox"
        role="switch"
        className="size-4 accent-[var(--brand)]"
        checked={on}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
      />
    </label>
  );
}

function Submissions() {
  const editable = useMe().can("compliance.manage");
  const [view, setView] = useState("waiting");
  const [q, setQ] = useState("");
  const [retrying, setRetrying] = useState<Schemas["RegulatorSubmission"]>();
  const retry = useRetrySubmission();
  const filter: SubmissionFilter =
    view === "waiting"
      ? { status: "PENDING" }
      : view === "stuck"
        ? { status: "PENDING", stuck: true }
        : view === "accepted"
          ? { status: "ACCEPTED" }
          : {};
  const { data, isLoading, error } = useSubmissions({ ...filter, q });
  return (
    <Card
      index={9}
      title="Submissions"
      action={<Tabs id="regulator-view" value={view} options={VIEWS} onChange={setView} />}
    >
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <p className="max-w-md text-xs leading-relaxed text-muted">
          Submissions for the selected country. Failed sends retry automatically; use the filters to find
          items needing attention.
        </p>
        <div className="w-full sm:w-64">
          <Search inset placeholder="Search code, plate or driver" value={q} onChange={setQ} />
        </div>
      </div>
      <ErrorNote error={error ?? retry.error} />
      {isLoading ? (
        <Skeleton className="h-40" />
      ) : !data?.length ? (
        <Empty
          title="Nothing here"
          body="Submissions appear when drivers are approved and trips complete in a regulated country."
        />
      ) : (
        <Table head={["Created", "What", "Status", "Tries", "Next try", "Last answer", ""]}>
          {data.map((s) => (
            <Row key={s.id}>
              <Cell className="tnum whitespace-nowrap text-muted">{when(s.created_at)}</Cell>
              <Cell>
                <div className="font-medium">{s.subject_label}</div>
                <div className="text-xs text-muted">{`${KIND_LABEL[s.kind] ?? s.kind} · ${s.country}`}</div>
              </Cell>
              <Cell>
                {s.status === "ACCEPTED" ? (
                  <span title={s.reference ?? ""}>
                    <Chip tone="ok">Accepted</Chip>
                  </span>
                ) : s.status === "SKIPPED" ? (
                  <Chip tone="muted">Nothing to send</Chip>
                ) : s.stuck ? (
                  <Chip tone="bad">Stuck</Chip>
                ) : (
                  <Chip tone="live">Waiting</Chip>
                )}
              </Cell>
              <Cell className="tnum">{s.attempts}</Cell>
              <Cell className="tnum whitespace-nowrap text-muted">
                {s.status === "PENDING" ? when(s.next_attempt_at) : ""}
              </Cell>
              <Cell className="max-w-[18rem] font-mono text-xs text-muted">
                <span className="block truncate" title={s.last_error ?? s.reference ?? ""}>
                  {s.status === "ACCEPTED"
                    ? (s.reference ?? s.last_code ?? "")
                    : [s.last_code, s.last_error].filter(Boolean).join(" · ")}
                </span>
              </Cell>
              <Cell className="text-right">
                {s.status === "PENDING" && editable ? (
                  <Button
                    size="sm"
                    variant="ghost"
                    icon={<RefreshCw size={14} aria-hidden="true" />}
                    onClick={() => setRetrying(s)}
                  >
                    Retry now
                  </Button>
                ) : null}
              </Cell>
            </Row>
          ))}
        </Table>
      )}
      <ReasonDialog
        open={!!retrying}
        title={`Send ${retrying?.subject_label ?? ""} again now?`}
        body="It goes out on the next sweep (within a minute)."
        confirm="Retry"
        busy={retry.isPending}
        onClose={() => setRetrying(undefined)}
        onConfirm={(reason) =>
          retrying && retry.mutate({ id: retrying.id, reason }, { onSuccess: () => setRetrying(undefined) })
        }
      />
    </Card>
  );
}
