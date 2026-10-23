"use client";

import {
  ArrowUpRight,
  Banknote,
  HandCoins,
  Landmark,
  ReceiptText,
  Send,
  ShieldCheck,
  Wallet,
} from "lucide-react";
import Link from "next/link";
import { Suspense, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import {
  FilterBar,
  Initials,
  MetricTile,
  OperationsHeader,
  OperationsPanel,
  RefreshButton,
} from "@/components/ui/Operations";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { useMe } from "@/features/me";
import { cents, count, money, when } from "@/lib/format";
import { useRegulator } from "@/lib/scope";
import { useUrlState } from "@/lib/urlState";
import { HoldsCard } from "./HoldsCard";
import { useFinance, useHoldAction, useVerifyLedger } from "./hooks";
import { JournalCard } from "./JournalCard";
import { PayinsCard } from "./PayinsCard";
import { PayoutsCard } from "./PayoutsCard";

const VIEWS = [
  { value: "overview", label: "Overview" },
  { value: "payouts", label: "Cash-outs" },
  { value: "payins", label: "Mobile money" },
  { value: "holds", label: "Driver holds" },
  { value: "journal", label: "Journal" },
] as const;
export function FinanceScreen() {
  return (
    <Suspense fallback={<Skeleton className="h-96" />}>
      <Finance />
    </Suspense>
  );
}

function Finance() {
  const regulator = useRegulator();
  const finance = useFinance();
  const { data, isLoading, error } = finance;
  const verify = useVerifyLedger();
  const me = useMe();
  const canHold = me.can("holds.manage");
  const hold = useHoldAction();
  const url = useUrlState();
  const requestedView =
    url.get("view") ??
    (url.get("payouts") !== undefined
      ? "payouts"
      : url.get("payins") !== undefined || url.get("payin_kind") !== undefined
        ? "payins"
        : "overview");
  const view = VIEWS.find((item) => item.value === requestedView)?.value ?? "overview";
  const [holdTarget, setHoldTarget] = useState<{ driverId: string; name: string }>();
  const summary = data?.summary;
  const currency = summary?.currency ?? "TZS";
  return (
    <div>
      <OperationsHeader
        title="Finance"
        description="Revenue, balances and payments in one place. All figures use the selected country’s ledger and currency."
        actions={
          <>
            <RefreshButton busy={finance.isFetching} onClick={() => void finance.refetch()} />
            <Button
              variant="primary"
              icon={<ShieldCheck size={15} aria-hidden="true" />}
              loading={verify.isPending}
              onClick={() => verify.mutate()}
            >
              Verify ledger
            </Button>
          </>
        }
      />
      {error || verify.error || hold.error ? (
        <div className="mb-4">
          <ErrorNote error={error ?? verify.error ?? hold.error} />
        </div>
      ) : null}
      {verify.data ? (
        <div
          className={`mb-5 flex items-center gap-3 rounded-xl border px-4 py-3 text-sm ${verify.data.ok ? "border-good/20 bg-good/10 text-good" : "border-critical/20 bg-critical/10 text-critical"}`}
          role="status"
        >
          <ShieldCheck size={18} className="shrink-0" aria-hidden="true" />
          <span>
            {verify.data.ok
              ? `Ledger intact: ${count(verify.data.checked)} entries checked`
              : `Ledger chain broken at entry ${verify.data.broken_at ?? "unknown"}`}
          </span>
        </div>
      ) : null}
      {isLoading ? (
        <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
          {[0, 1, 2, 3].map((index) => (
            <Skeleton key={index} className="h-40" />
          ))}
        </div>
      ) : !summary || !data ? (
        <Empty title="Finance unavailable" body="Use Refresh to load ledger balances." />
      ) : (
        <div className="grid min-w-0 grid-cols-1 gap-5">
          <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
            <MetricTile
              label="Platform revenue"
              value={money(summary.platform_revenue, currency)}
              note="Commission after tax"
              icon={<Landmark size={17} />}
              accent
            />
            <MetricTile
              label="Drivers owe"
              value={money(summary.drivers_owe, currency)}
              note="Commission due from cash trips"
              icon={<Wallet size={17} />}
            />
            <MetricTile
              label="Owed to drivers"
              value={money(summary.owed_to_drivers, currency)}
              note="Driver balances payable by the platform"
              icon={<HandCoins size={17} />}
            />
            <MetricTile
              label="Gateway clearing"
              value={money(summary.gateway_clearing, currency)}
              note="Collected mobile money awaiting the bank"
              icon={<Banknote size={17} />}
            />
          </div>
          <div className="flex min-w-0 flex-wrap items-center justify-between gap-3 rounded-xl border border-line bg-surface p-2">
            <div className="min-w-0 max-w-full">
              <FilterBar
                label="Finance views"
                value={view}
                options={VIEWS}
                onChange={(value) => url.set({ view: value })}
              />
            </div>
            <span className="tnum px-2 text-[11px] text-muted">{`${count(summary.ledger_entries)} ledger entries · ${currency}`}</span>
          </div>
          {view === "overview" ? (
            <>
              <OperationsPanel
                title="Amounts payable"
                description="Tax, customer obligations, third-party charges and outgoing cash-outs"
              >
                <dl className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3">
                  <Figure
                    label="VAT payable"
                    value={`${currency} ${cents(summary.vat_payable)}`}
                    note="Due to the tax authority"
                  />
                  <Figure
                    label={`${regulator.levy} payable`}
                    value={`${currency} ${cents(summary.levy_payable)}`}
                    note="Due to the transport regulator"
                  />
                  <Figure
                    label="Owed to riders"
                    value={money(summary.owed_to_riders ?? 0, currency)}
                    note="Late fare payments awaiting refund"
                  />
                  <Figure
                    label="Owed to third parties"
                    value={money(
                      (summary.charges_payable ?? []).reduce((sum, payee) => sum + Number(payee.amount), 0),
                      currency,
                    )}
                    note={
                      summary.charges_payable?.length
                        ? summary.charges_payable
                            .map((payee) => `${payee.name}: ${money(payee.amount, currency)}`)
                            .join(" · ")
                        : "Extra charges held for charge payees"
                    }
                  />
                  <Figure
                    label="Cash-outs in progress"
                    value={money(data.payouts?.payouts_in_progress ?? 0, currency)}
                    note={`Paid today ${money(data.payouts?.payouts_paid_today ?? 0, currency)}`}
                  />
                  <Figure
                    label="Rider cancellation dues"
                    value={money(summary.rider_dues, currency)}
                    note="Outstanding cancellation charges"
                  />
                </dl>
              </OperationsPanel>
              <div className="grid gap-3 sm:grid-cols-3">
                {[
                  {
                    view: "payouts",
                    title: "Manage cash-outs",
                    text: "Review requests and follow outgoing payments",
                    icon: <Send size={17} />,
                  },
                  {
                    view: "payins",
                    title: "Incoming payments",
                    text: "Follow mobile-money collections and failures",
                    icon: <Banknote size={17} />,
                  },
                  {
                    view: "journal",
                    title: "Explore the journal",
                    text: "Inspect transactions and their ledger legs",
                    icon: <ReceiptText size={17} />,
                  },
                ].map((item) => (
                  <button
                    key={item.view}
                    type="button"
                    onClick={() => url.set({ view: item.view })}
                    className="group flex items-start gap-3 rounded-xl border border-line bg-surface p-4 text-left transition-colors hover:border-brand/25"
                  >
                    <span
                      className="grid size-9 shrink-0 place-items-center rounded-lg bg-brand-soft text-brand-text"
                      aria-hidden="true"
                    >
                      {item.icon}
                    </span>
                    <div className="min-w-0 flex-1">
                      <p className="text-xs font-semibold">{item.title}</p>
                      <p className="mt-1.5 text-[11px] leading-relaxed text-muted">{item.text}</p>
                    </div>
                    <ArrowUpRight
                      size={14}
                      className="shrink-0 text-muted group-hover:text-brand-text"
                      aria-hidden="true"
                    />
                  </button>
                ))}
              </div>
              <Card
                title="Driver balances"
                action={
                  <button
                    type="button"
                    onClick={() => url.set({ view: "holds" })}
                    className="inline-flex items-center gap-1 text-xs font-medium text-brand-text"
                  >
                    View holds <ArrowUpRight size={13} aria-hidden="true" />
                  </button>
                }
              >
                <p className="mb-4 text-xs leading-relaxed text-muted">
                  Negative balances are commission owed on cash trips. Positive balances are owed to the
                  driver.
                </p>
                {!data.drivers.length ? (
                  <Empty title="No driver balances" body="Balances appear after a driver’s first trip." />
                ) : (
                  <Table head={["Driver", "Balance", "Last movement", "Account hold"]}>
                    {data.drivers.map((driver) => (
                      <Row key={driver.driver_id}>
                        <Cell>
                          <Link
                            href={`/drivers/${driver.driver_id}`}
                            className="group flex items-center gap-2.5"
                          >
                            <Initials name={driver.name} />
                            <span>
                              <span className="block text-xs font-semibold group-hover:text-brand-text">
                                {driver.name ?? "Name unavailable"}
                              </span>
                              <span className="tnum mt-1 block text-[11px] text-muted">{driver.phone}</span>
                            </span>
                          </Link>
                        </Cell>
                        <Cell
                          className={`tnum whitespace-nowrap text-xs font-semibold ${Number(driver.balance) < 0 ? "text-critical" : ""}`}
                        >
                          {money(driver.balance, currency)}
                        </Cell>
                        <Cell className="tnum whitespace-nowrap text-xs text-muted">
                          {when(driver.last_at)}
                        </Cell>
                        <Cell>
                          {driver.on_hold ? (
                            <Chip tone={driver.hold_kind === "MANUAL" ? "bad" : "warn"}>
                              {driver.hold_kind === "MANUAL" ? "Staff hold" : "Debt hold"}
                            </Chip>
                          ) : canHold ? (
                            <Button
                              size="sm"
                              variant="soft"
                              onClick={() =>
                                setHoldTarget({
                                  driverId: driver.driver_id,
                                  name: driver.name ?? driver.phone,
                                })
                              }
                            >
                              Place hold
                            </Button>
                          ) : (
                            <span className="text-xs text-muted">No hold</span>
                          )}
                        </Cell>
                      </Row>
                    ))}
                  </Table>
                )}
              </Card>
            </>
          ) : view === "payouts" ? (
            <PayoutsCard index={0} editable={me.can("payouts.approve")} />
          ) : view === "payins" ? (
            <PayinsCard index={0} />
          ) : view === "holds" ? (
            <HoldsCard index={0} currency={currency} editable={canHold} />
          ) : (
            <JournalCard index={0} currency={currency} />
          )}
        </div>
      )}
      <ReasonDialog
        open={!!holdTarget}
        title={`Put ${holdTarget?.name ?? ""} on hold?`}
        body="They are taken offline (after any trip in progress) and cannot go online until a staff member releases the hold."
        confirm="Place hold"
        danger
        busy={hold.isPending}
        onClose={() => setHoldTarget(undefined)}
        onConfirm={(reason) =>
          holdTarget &&
          hold.mutate(
            { driverId: holdTarget.driverId, release: false, reason },
            { onSuccess: () => setHoldTarget(undefined) },
          )
        }
      />
    </div>
  );
}
function Figure({ label, value, note }: { label: string; value: string; note: string }) {
  return (
    <div className="min-w-0 border-b border-line px-5 py-5">
      <dt className="text-[11px] text-muted">{label}</dt>
      <dd className="tnum mt-2 break-words text-lg font-semibold">{value}</dd>
      <p className="mt-1.5 text-[10px] leading-relaxed text-muted">{note}</p>
    </div>
  );
}
