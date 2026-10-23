"use client";
/**
 * ProfileScreen — one pricing profile. Rules and extra charges are edited as a
 * draft and published together as the next version (now or from a date);
 * "Where it applies" assigns it to regions; Versions compares any two;
 * Activity is its history. Switching off and making it the default ask for a reason.
 */
import { ArrowLeft, FileStack, MapPin, Power, ReceiptText, Rocket, Star } from "lucide-react";
import Link from "next/link";
import { Suspense, useEffect, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Field, Input } from "@/components/ui/Field";
import { MetricTile, OperationsHeader } from "@/components/ui/Operations";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Tabs } from "@/components/ui/Tabs";
import { useMe } from "@/features/me";
import type { Schemas } from "@/lib/api";
import { when } from "@/lib/format";
import { useCountries } from "@/lib/scope";
import { useUrlState } from "@/lib/urlState";
import { ChargesEditor } from "./ChargesEditor";
import { EventsCard } from "./EventsCard";
import { useMakeDefault, usePayees, useProfile, useProfiles, usePublishVersion, useSetStatus } from "./hooks";
import { type Charge, STATUS_LABEL, STATUS_TONE, type Terms } from "./model";
import { ScopesCard } from "./ScopesCard";
import { TermsForm } from "./TermsForm";
import { VersionsCard } from "./VersionsCard";

const TABS = [
  { value: "rules", label: "Rules" },
  { value: "charges", label: "Extra charges" },
  { value: "where", label: "Where it applies" },
  { value: "versions", label: "Versions" },
  { value: "activity", label: "Activity" },
] as const;
type Tab = (typeof TABS)[number]["value"];
type Ask = "publish" | "status" | "default";

/** The version in force now (versions come newest first), else the newest. */
function current(versions: Schemas["ProfileVersion"][] | undefined) {
  const now = Date.now();
  return versions?.find((v) => new Date(v.effective_from).getTime() <= now) ?? versions?.[0];
}

export function ProfileScreen({ id }: { id: string }) {
  return (
    <Suspense>
      <Profile key={id} id={id} />
    </Suspense>
  );
}

function Profile({ id }: { id: string }) {
  const { data: p, isLoading, error } = useProfile(id);
  const list = useProfiles(p?.country ?? "");
  const defaultId = list.data?.find((x) => x.is_default)?.id;
  const def = useProfile(p && !p.is_default ? defaultId : undefined);
  const payees = usePayees(p?.country ?? "");
  const { data: countries } = useCountries();
  const currency = countries?.find((c) => c.code === p?.country)?.currency ?? "";
  const me = useMe();
  const editable = me.can("fees.edit");
  const url = useUrlState();
  const tab: Tab = TABS.find((item) => item.value === url.get("tab"))?.value ?? "rules";
  const [terms, setTerms] = useState<Terms>();
  const [charges, setCharges] = useState<Charge[]>();
  /** The version the draft was started from; a newer one (after publishing) restarts the draft. */
  const [from, setFrom] = useState<string>();
  const [startsAt, setStartsAt] = useState("");
  const [ask, setAsk] = useState<Ask>();
  const publish = usePublishVersion();
  const status = useSetStatus();
  const makeDefault = useMakeDefault();
  const base = current(p?.versions);
  const baseKey = p ? (base?.id ?? "none") : undefined;
  useEffect(() => {
    if (baseKey !== undefined && baseKey !== from) {
      setTerms(base?.terms ?? {});
      setCharges(base?.charges ?? []);
      setFrom(baseKey);
    }
  }, [baseKey, from, base]);
  const changed =
    JSON.stringify(terms) !== JSON.stringify(base?.terms ?? {}) ||
    JSON.stringify(charges) !== JSON.stringify(base?.charges ?? []);
  const defaults = p?.is_default ? undefined : current(def.data?.versions)?.terms;
  return (
    <div>
      <OperationsHeader
        title={p?.name ?? "Pricing profile"}
        description={
          p
            ? p.description ||
              (p.is_default ? "The country's default rules" : "Rules for the places it is applied to")
            : "Manage this profile’s rules, charges and published versions."
        }
        actions={
          <Link
            href="/pricing/fees"
            className="inline-flex items-center gap-1.5 text-sm font-semibold text-brand-text"
          >
            <ArrowLeft size={15} aria-hidden="true" /> All profiles
          </Link>
        }
      />
      <ErrorNote error={error ?? publish.error ?? status.error ?? makeDefault.error} />
      {isLoading ? (
        <Skeleton className="h-96" />
      ) : !p ? (
        <Empty
          title="Pricing profile unavailable"
          body="Return to all profiles to find the pricing rules you need."
        />
      ) : !terms || !charges ? (
        <Skeleton className="h-96" />
      ) : (
        <div className="grid gap-4">
          <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
            <MetricTile
              label="Current version"
              value={p.current_version ? `v${p.current_version.version}` : "Unpublished"}
              note={
                p.current_version
                  ? `In force since ${when(p.current_version.effective_from)}`
                  : "Publish rules before activating"
              }
              icon={<FileStack size={17} />}
              accent
            />
            <MetricTile
              label="Extra charges"
              value={String(base?.charges.length ?? 0)}
              note="Charges in the loaded published version"
              icon={<ReceiptText size={17} />}
            />
            <MetricTile
              label="Applies to"
              value={p.is_default ? "Country" : `${p.scopes.length} regions`}
              note={
                p.is_default
                  ? "Used when no regional profile applies"
                  : "Places currently assigned this profile"
              }
              icon={<MapPin size={17} />}
            />
            <MetricTile
              label="Version history"
              value={String(p.versions?.length ?? 0)}
              note={
                p.next_version
                  ? `v${p.next_version.version} scheduled next`
                  : "Published versions available to compare"
              }
              icon={<FileStack size={17} />}
            />
          </div>
          <Card index={0} title="Publication status">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex flex-wrap items-center gap-2 text-sm">
                {p.is_default ? <Chip tone="brand">Default</Chip> : null}
                <Chip tone={STATUS_TONE[p.status]}>{STATUS_LABEL[p.status]}</Chip>
                <span className="text-muted">
                  {p.current_version
                    ? `Version ${p.current_version.version}, in force since ${when(p.current_version.effective_from)}`
                    : "Not published yet"}
                </span>
                {p.next_version ? (
                  <Chip tone="live">{`v${p.next_version.version} from ${when(p.next_version.effective_from)}`}</Chip>
                ) : null}
              </div>
              {editable ? (
                <div className="flex flex-wrap gap-2">
                  {!p.is_default ? (
                    <Button
                      variant={p.status === "ACTIVE" ? "danger" : "primary"}
                      icon={<Power size={15} aria-hidden="true" />}
                      disabled={!p.current_version}
                      onClick={() => setAsk("status")}
                    >
                      {p.status === "ACTIVE" ? "Switch off" : "Switch on"}
                    </Button>
                  ) : null}
                  {!p.is_default && p.status === "ACTIVE" ? (
                    <Button
                      variant="soft"
                      icon={<Star size={15} aria-hidden="true" />}
                      onClick={() => setAsk("default")}
                    >
                      Make default
                    </Button>
                  ) : null}
                </div>
              ) : null}
            </div>
          </Card>
          <Tabs
            id="profile"
            value={tab}
            options={[...TABS]}
            onChange={(v) => url.set({ tab: v === "rules" ? undefined : v })}
          />
          <Card
            index={1}
            title={tab === "rules" ? "Pricing rules" : TABS.find((item) => item.value === tab)?.label}
            action={!editable ? <Chip tone="muted">Read only</Chip> : undefined}
          >
            {tab === "rules" ? (
              <TermsForm
                terms={terms}
                defaults={defaults}
                editable={editable}
                currency={currency}
                onChange={setTerms}
              />
            ) : tab === "charges" ? (
              <ChargesEditor
                charges={charges}
                payees={payees.data ?? []}
                editable={editable}
                currency={currency}
                onChange={setCharges}
              />
            ) : tab === "where" ? (
              <ScopesCard profile={p} editable={editable} />
            ) : tab === "versions" ? (
              <VersionsCard versions={p.versions ?? []} currency={currency} />
            ) : (
              <EventsCard events={p.events ?? []} />
            )}
          </Card>
          {editable && changed && (tab === "rules" || tab === "charges") ? (
            <Card index={2} className="sticky bottom-4 z-20 border-brand/20 shadow-lg">
              <div className="flex flex-wrap items-end justify-between gap-3">
                <div>
                  <p className="text-sm font-semibold text-brand-text">Unpublished changes</p>
                  <p className="mt-1 text-xs leading-relaxed text-muted">
                    Publish a new version to apply these rules. Existing trips keep their original version.
                  </p>
                </div>
                <div className="flex flex-wrap items-end gap-2">
                  <Field label="Starts" hint="Empty = now">
                    <Input
                      type="datetime-local"
                      value={startsAt}
                      onChange={(e) => setStartsAt(e.target.value)}
                      className="h-10 w-56"
                    />
                  </Field>
                  <Button
                    variant="ghost"
                    onClick={() => {
                      setTerms(base?.terms ?? {});
                      setCharges(base?.charges ?? []);
                    }}
                  >
                    Discard
                  </Button>
                  <Button icon={<Rocket size={15} aria-hidden="true" />} onClick={() => setAsk("publish")}>
                    Publish changes
                  </Button>
                </div>
              </div>
            </Card>
          ) : null}
        </div>
      )}
      <ReasonDialog
        open={!!ask}
        title={
          ask === "publish"
            ? "Publish a new version?"
            : ask === "default"
              ? `Make ${p?.name ?? "this profile"} the country default?`
              : p?.status === "ACTIVE"
                ? "Switch this profile off?"
                : "Switch this profile on?"
        }
        body={
          ask === "publish"
            ? "Quotes from the start time use it. Trips already priced keep their version."
            : ask === "default"
              ? "It prices every trip no region profile covers. The current default stops being the default at the same moment."
              : p?.status === "ACTIVE"
                ? "New trips stop using this profile. Trips already priced keep it."
                : "It starts pricing trips where it is applied."
        }
        confirm={
          ask === "publish"
            ? "Publish"
            : ask === "default"
              ? "Make default"
              : p?.status === "ACTIVE"
                ? "Switch off"
                : "Switch on"
        }
        danger={ask === "status" && p?.status === "ACTIVE"}
        busy={publish.isPending || status.isPending || makeDefault.isPending}
        onClose={() => setAsk(undefined)}
        onConfirm={(reason) => {
          if (!p) return;
          const done = { onSuccess: () => setAsk(undefined) };
          if (ask === "publish") {
            const effective_from = startsAt ? new Date(startsAt).toISOString() : undefined;
            publish.mutate(
              { id: p.id, terms: terms ?? {}, charges: charges ?? [], effective_from, reason },
              {
                onSuccess: () => {
                  setAsk(undefined);
                  setStartsAt("");
                },
              },
            );
          } else if (ask === "default") makeDefault.mutate({ id: p.id, reason }, done);
          else
            status.mutate({ id: p.id, status: p.status === "ACTIVE" ? "INACTIVE" : "ACTIVE", reason }, done);
        }}
      />
    </div>
  );
}
