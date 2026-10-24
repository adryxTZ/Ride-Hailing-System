"use client";
/**
 * AccountsScreen — Finance › Accounts: one line per party paid from fares
 * (platform revenue, VAT, levy, each charge payee, drivers, riders, the gateway
 * and the bank) with its balance now, the period's movements and, for fare
 * shares, the fare base and effective rate. All in the selected country's currency.
 */
import { ArrowUpRight, HandCoins, Landmark, ReceiptText, Wallet } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Suspense, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { MetricTile, OperationsHeader, OperationsPanel, RefreshButton } from "@/components/ui/Operations";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { useMe } from "@/features/me";
import { count, when } from "@/lib/format";
import { useRegulator } from "@/lib/scope";
import { type Period, useAccounts, usePeriod } from "./hooks";
import {
  ACCOUNT_NOTE,
  type Account,
  accountName,
  amount,
  PAYEE_KIND,
  positionLabel,
  positionTone,
  rate,
} from "./labels";
import { PaymentDialog } from "./PaymentDialog";
import { PeriodPicker } from "./PeriodPicker";

const HEAD = [
  "Account",
  "Position",
  "Balance now",
  "Added in period",
  "Paid or cleared",
  "Fare base",
  "Rate",
  "Last entry",
  "",
];

export function AccountsScreen() {
  return (
    <Suspense fallback={<Skeleton className="h-96" />}>
      <Accounts />
    </Suspense>
  );
}

function Accounts() {
  const { period, set } = usePeriod();
  const accounts = useAccounts(period);
  const { data, isLoading, error } = accounts;
  return (
    <div>
      <OperationsHeader
        title="Accounts"
        description="Every party paid from fares: what we earned, what we owe and what is owed to us. Balances are now; movements are for the period."
        actions={<RefreshButton busy={accounts.isFetching} onClick={() => void accounts.refetch()} />}
      />
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-line bg-surface p-2">
        <PeriodPicker value={period} onChange={set} />
        {data ? (
          <span className="tnum px-2 text-[11px] text-muted">{`${count(data.trips)} trips · fares ${amount(data.fares, data.currency)}`}</span>
        ) : null}
      </div>
      <ErrorNote error={error} />
      {isLoading ? (
        <Skeleton className="h-96" />
      ) : !data ? (
        <Empty title="Accounts unavailable" body="Use Refresh to load the ledger." />
      ) : (
        <div className="grid min-w-0 gap-5">
          <Totals accounts={data.accounts} currency={data.currency} />
          <AccountsTable accounts={data.accounts} period={period} />
        </div>
      )}
    </div>
  );
}

/** Earned, we owe, owed to us: sums of lines in one currency. */
function Totals({ accounts, currency }: { accounts: Account[]; currency: string }) {
  const earned = accounts.filter((a) => a.position === "EARNED").reduce((s, a) => s + Number(a.balance), 0);
  const weOwe = accounts.reduce((s, a) => s + weOweOf(a), 0);
  const owedToUs = accounts.reduce((s, a) => s + owedToUsOf(a), 0);
  const held = accounts.filter((a) => a.position === "HELD").reduce((s, a) => s + Number(a.balance), 0);
  return (
    <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
      <MetricTile
        label="Earned"
        value={amount(earned, currency)}
        note="Platform revenue after tax, all time"
        icon={<Landmark size={17} />}
        accent
      />
      <MetricTile
        label="We owe"
        value={amount(weOwe, currency)}
        note="Tax, levy, payees, riders and drivers"
        icon={<ReceiptText size={17} />}
      />
      <MetricTile
        label="Owed to us"
        value={amount(owedToUs, currency)}
        note="Driver commission and rider dues"
        icon={<Wallet size={17} />}
      />
      <MetricTile
        label="Held at the gateway"
        value={amount(held, currency)}
        note="Collected, not yet in the bank"
        icon={<HandCoins size={17} />}
      />
    </div>
  );
}

/** What a line says we owe (per-person lines count only those we owe). */
function weOweOf(a: Account): number {
  if (a.we_owe !== undefined) return Number(a.we_owe);
  return a.position === "WE_OWE" ? Math.max(0, Number(a.balance)) : 0;
}

function owedToUsOf(a: Account): number {
  if (a.owed_to_us !== undefined) return Number(a.owed_to_us);
  return a.position === "OWED_TO_US" ? Math.max(0, Number(a.balance)) : 0;
}

function AccountsTable({ accounts, period }: { accounts: Account[]; period: Period }) {
  const router = useRouter();
  const me = useMe();
  const regulator = useRegulator();
  const [paying, setPaying] = useState<Account>();
  const href = (a: Account) =>
    `/finance/accounts/${encodeURIComponent(a.key)}?from=${period.from}&to=${period.to}`;
  return (
    <OperationsPanel
      title="Accounts"
      description="Open an account for its entries, the CSV export and payments."
    >
      <div className="p-4">
        <Table head={HEAD}>
          {accounts.map((a) => {
            const label = positionLabel(a.position, Number(a.balance));
            return (
              <Row key={a.key} onClick={() => router.push(href(a))}>
                <Cell>
                  <Link
                    href={href(a)}
                    className="block text-xs font-semibold hover:text-brand-text"
                    onClick={(e) => e.stopPropagation()}
                  >
                    {accountName(a, regulator.levy)}
                  </Link>
                  <span className="mt-1 block text-[11px] text-muted">
                    {a.payee_kind ? (PAYEE_KIND[a.payee_kind] ?? a.payee_kind) : ACCOUNT_NOTE[a.kind]}
                  </span>
                </Cell>
                <Cell>
                  <Chip tone={positionTone(label)}>{label}</Chip>
                </Cell>
                <Cell className="tnum whitespace-nowrap text-xs font-semibold">
                  <Balance account={a} />
                </Cell>
                <Cell className="tnum whitespace-nowrap text-xs">{amount(a.added, a.currency)}</Cell>
                <Cell className="tnum whitespace-nowrap text-xs">{amount(a.reduced, a.currency)}</Cell>
                <Cell className="tnum whitespace-nowrap text-xs text-muted">
                  {a.fare_base ? amount(a.fare_base, a.currency) : "—"}
                </Cell>
                <Cell className="tnum whitespace-nowrap text-xs">{rate(a.rate)}</Cell>
                <Cell className="tnum whitespace-nowrap text-xs text-muted">
                  {a.last_at ? when(a.last_at) : "No entries"}
                </Cell>
                <Cell>
                  <LineAction account={a} canSettle={me.can("finance.settle")} onPay={() => setPaying(a)} />
                </Cell>
              </Row>
            );
          })}
        </Table>
      </div>
      {paying ? (
        <PaymentDialog
          account={paying}
          name={accountName(paying, regulator.levy)}
          open={!!paying}
          onClose={() => setPaying(undefined)}
        />
      ) : null}
    </OperationsPanel>
  );
}

/** The balance; per-person lines also show both sides. */
function Balance({ account: a }: { account: Account }) {
  const value = amount(Math.abs(Number(a.balance)), a.currency);
  if (a.we_owe === undefined || a.owed_to_us === undefined) return <>{value}</>;
  return (
    <span>
      {value}
      <span className="mt-1 block text-[11px] font-normal text-muted">
        {`Owed to us ${amount(a.owed_to_us, a.currency)} · we owe ${amount(a.we_owe, a.currency)}`}
      </span>
    </span>
  );
}

/** Record payment on what we owe; drivers link to their balances. */
function LineAction({
  account: a,
  canSettle,
  onPay,
}: {
  account: Account;
  canSettle: boolean;
  onPay: () => void;
}) {
  if (a.kind === "DRIVER_WALLET") {
    return (
      <Link
        href="/finance"
        onClick={(e) => e.stopPropagation()}
        className="inline-flex items-center gap-1 whitespace-nowrap text-xs font-medium text-brand-text"
      >
        Driver balances <ArrowUpRight size={13} aria-hidden="true" />
      </Link>
    );
  }
  if (!a.settleable || !canSettle || Number(a.balance) <= 0) return null;
  return (
    <span
      className="whitespace-nowrap"
      onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => e.stopPropagation()}
      role="presentation"
    >
      <Button size="sm" variant="soft" onClick={onPay}>
        Record payment
      </Button>
    </span>
  );
}
