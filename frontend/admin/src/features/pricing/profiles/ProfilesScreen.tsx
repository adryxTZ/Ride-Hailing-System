"use client";

import {
  ArrowUpRight,
  Calculator,
  CheckCheck,
  ChevronRight,
  Clock3,
  FileStack,
  History,
  MapPin,
  Plus,
  ReceiptText,
  Star,
} from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Suspense, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Field, Input } from "@/components/ui/Field";
import { MetricTile, OperationsHeader, RecordsTable } from "@/components/ui/Operations";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Search } from "@/components/ui/Search";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Tabs } from "@/components/ui/Tabs";
import { useMe } from "@/features/me";
import type { Schemas } from "@/lib/api";
import { count, when } from "@/lib/format";
import { useScope } from "@/lib/scope";
import { useUrlState } from "@/lib/urlState";
import { useScopeLabel } from "../scopeLabel";
import { useCreateProfile, useProfiles } from "./hooks";
import { STATUS_LABEL, STATUS_TONE } from "./model";
import { PayeesCard } from "./PayeesCard";

const TABS = [
  { value: "profiles", label: "Pricing profiles" },
  { value: "payees", label: "Payees" },
] as const;
export function ProfilesScreen() {
  return (
    <Suspense fallback={<Skeleton className="h-96" />}>
      <Profiles />
    </Suspense>
  );
}

function Profiles() {
  const { scope } = useScope();
  const where = useScopeLabel();
  const url = useUrlState();
  const tab = url.get("tab") === "payees" ? "payees" : "profiles";
  const { data, isLoading, error } = useProfiles(scope.country);
  const router = useRouter();
  const create = useCreateProfile();
  const [name, setName] = useState("");
  const [search, setSearch] = useState("");
  const [asking, setAsking] = useState(false);
  const editable = useMe().can("fees.edit");
  const visible = [...(data ?? [])]
    .filter((profile) =>
      `${profile.name} ${profile.scopes.map((s) => s.name).join(" ")}`
        .toLowerCase()
        .includes(search.toLowerCase()),
    )
    .sort((a, b) => Number(b.is_default) - Number(a.is_default));
  const defaultProfile = data?.find((profile) => profile.is_default);
  return (
    <div>
      <OperationsHeader
        title="Fees and profiles"
        description={`Manage pricing rules, extra charges, and their payees in ${where.label.split(" · ")[0] || "the selected country"}.`}
        actions={
          <>
            <Link
              href="/pricing/fees/history"
              className="inline-flex h-9 items-center gap-1.5 rounded-full bg-surface-2 px-4 text-xs font-semibold hover:bg-line"
            >
              <History size={14} aria-hidden="true" /> History
            </Link>
            <Link
              href="/pricing/fees/simulate"
              className="inline-flex h-9 items-center gap-1.5 rounded-full bg-surface-2 px-4 text-xs font-semibold hover:bg-line"
            >
              <Calculator size={14} aria-hidden="true" /> Simulate
            </Link>
            {editable && tab === "profiles" ? (
              <Button icon={<Plus size={15} aria-hidden="true" />} onClick={() => setAsking(true)}>
                New profile
              </Button>
            ) : null}
          </>
        }
      />
      <div className="mb-5 grid grid-cols-2 gap-3 xl:grid-cols-4">
        <MetricTile
          label="Pricing profiles"
          value={data ? count(data.length) : error ? "—" : undefined}
          note="Country and regional pricing rules"
          icon={<FileStack size={17} />}
          accent
        />
        <MetricTile
          label="Active profiles"
          value={data ? count(data.filter((p) => p.status === "ACTIVE").length) : error ? "—" : undefined}
          note="Profiles switched on for pricing"
          icon={<CheckCheck size={17} />}
        />
        <MetricTile
          label="Scheduled versions"
          value={data ? count(data.filter((p) => p.next_version).length) : error ? "—" : undefined}
          note="Published versions starting later"
          icon={<Clock3 size={17} />}
        />
        <MetricTile
          label="Configured charges"
          value={data ? count(data.reduce((sum, p) => sum + p.charges_count, 0)) : error ? "—" : undefined}
          note="Extra charges across pricing profiles"
          icon={<ReceiptText size={17} />}
        />
      </div>
      {defaultProfile ? (
        <Link
          href={`/pricing/fees/${defaultProfile.id}`}
          className="mb-5 flex items-center gap-3 rounded-xl border border-line bg-surface px-4 py-4 transition-colors hover:border-brand/25"
        >
          <span className="grid size-10 shrink-0 place-items-center rounded-lg bg-brand-soft text-brand-text">
            <Star size={18} aria-hidden="true" />
          </span>
          <div className="min-w-0 flex-1">
            <p className="text-xs font-semibold">
              {defaultProfile.name}
              <span className="ml-2 text-[10px] font-normal text-muted">Country default</span>
            </p>
            <p className="mt-1 text-[11px] leading-relaxed text-muted">
              Used wherever no regional profile applies. Each trip keeps the version it was priced under.
            </p>
          </div>
          <ArrowUpRight size={16} className="shrink-0 text-brand-text" aria-hidden="true" />
        </Link>
      ) : null}
      <div className="mb-4">
        <Tabs
          id="fees"
          value={tab}
          options={[...TABS]}
          onChange={(value) => url.set({ tab: value === "profiles" ? undefined : value })}
        />
      </div>
      {error || create.error ? (
        <div className="mb-4">
          <ErrorNote error={error ?? create.error} />
        </div>
      ) : null}
      {tab === "payees" ? (
        <Card title="Charge payees">
          <PayeesCard country={scope.country} editable={editable} />
        </Card>
      ) : (
        <section
          aria-label="Pricing profiles"
          className="min-w-0 overflow-hidden rounded-xl border border-line bg-surface"
        >
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-5 py-4">
            <div>
              <h2 className="text-sm font-semibold">Profiles</h2>
              <p className="mt-1 text-[11px] text-muted">{`${visible.length} profiles · Open one to manage rules and versions`}</p>
            </div>
            <div className="w-full sm:w-64">
              <Search inset value={search} onChange={setSearch} placeholder="Search profiles or regions" />
            </div>
          </div>
          {isLoading ? (
            <div className="p-5">
              <Skeleton className="h-64" />
            </div>
          ) : !visible.length ? (
            <Empty
              title={error ? "Profiles unavailable" : "No profiles in this view"}
              body={
                search
                  ? "Try another profile name or region."
                  : "Pricing profiles appear here when configured for this country."
              }
            />
          ) : (
            <>
              <RecordsTable
                head={[
                  "Profile",
                  "Where it applies",
                  "Status",
                  "Version",
                  "Charges",
                  "In force since",
                  "Changed",
                  "",
                ]}
              >
                {visible.map((p) => (
                  <tr
                    key={p.id}
                    className="cursor-pointer transition-colors hover:bg-surface-2"
                    onClick={(event) => {
                      if (!(event.target as HTMLElement).closest("a")) router.push(`/pricing/fees/${p.id}`);
                    }}
                  >
                    <td className="px-5 py-4">
                      <Link
                        href={`/pricing/fees/${p.id}`}
                        className="flex items-center gap-2 text-xs font-semibold hover:text-brand-text"
                      >
                        {p.name}
                        {p.is_default ? <Chip tone="brand">Default</Chip> : null}
                      </Link>
                    </td>
                    <td className="max-w-48 px-5 py-4 text-xs text-muted">{scopeText(p)}</td>
                    <td className="px-5 py-4">
                      <Chip tone={STATUS_TONE[p.status]}>{STATUS_LABEL[p.status]}</Chip>
                    </td>
                    <td className="tnum px-5 py-4 text-xs">
                      {p.current_version ? `v${p.current_version.version}` : "Unpublished"}
                      {p.next_version ? (
                        <p className="mt-1 text-[10px] text-muted">{`v${p.next_version.version} scheduled`}</p>
                      ) : null}
                    </td>
                    <td className="tnum px-5 py-4 text-xs">{p.charges_count}</td>
                    <td className="tnum whitespace-nowrap px-5 py-4 text-[11px] text-muted">
                      {p.current_version ? when(p.current_version.effective_from) : "—"}
                    </td>
                    <td className="tnum whitespace-nowrap px-5 py-4 text-[11px] text-muted">
                      {when(p.updated_at)}
                    </td>
                    <td className="pr-4">
                      <Link
                        href={`/pricing/fees/${p.id}`}
                        aria-label={`Open profile ${p.name}`}
                        className="text-muted hover:text-brand-text"
                      >
                        <ChevronRight size={15} aria-hidden="true" />
                      </Link>
                    </td>
                  </tr>
                ))}
              </RecordsTable>
              <ul className="divide-y divide-line md:hidden">
                {visible.map((p) => (
                  <li key={p.id}>
                    <Link href={`/pricing/fees/${p.id}`} className="block p-4 hover:bg-surface-2">
                      <div className="flex items-start justify-between gap-2">
                        <p className="text-sm font-semibold">{p.name}</p>
                        <Chip tone={STATUS_TONE[p.status]}>{STATUS_LABEL[p.status]}</Chip>
                      </div>
                      {p.is_default ? (
                        <p className="mt-2 text-[10px] font-semibold text-brand-text">Country default</p>
                      ) : null}
                      <p className="mt-3 flex items-start gap-1.5 text-[11px] text-muted">
                        <MapPin size={12} className="shrink-0" aria-hidden="true" />
                        {scopeText(p)}
                      </p>
                      <div className="mt-3 flex items-center justify-between border-t border-line pt-3 text-[11px] text-muted">
                        <span>
                          {p.current_version ? `Version ${p.current_version.version}` : "Unpublished"}
                        </span>
                        <span>{`${p.charges_count} charges`}</span>
                      </div>
                    </Link>
                  </li>
                ))}
              </ul>
            </>
          )}
        </section>
      )}
      <ReasonDialog
        open={asking}
        title="New pricing profile"
        body="It starts as a draft: set its rules and charges, publish, switch it on and apply it to a region."
        confirm="Create"
        busy={create.isPending}
        onClose={() => setAsking(false)}
        onConfirm={(reason) =>
          name.trim() &&
          create.mutate(
            { country: scope.country, name: name.trim(), reason },
            { onSuccess: (p) => router.push(`/pricing/fees/${p.id}`) },
          )
        }
      >
        <Field label="Name">
          <Input
            value={name}
            maxLength={80}
            onChange={(event) => setName(event.target.value)}
            placeholder="e.g. Airport and city centre"
          />
        </Field>
      </ReasonDialog>
    </div>
  );
}
function scopeText(profile: Schemas["PricingProfile"]) {
  return profile.is_default
    ? "Whole country"
    : profile.scopes.length
      ? profile.scopes.map((scope) => scope.name).join(", ")
      : "Not applied to a region";
}
