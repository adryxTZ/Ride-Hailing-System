"use client";
/**
 * AccountDetail — one finance account: its balance and the period's movements,
 * its entries (date, trip, description, amount, running balance) with paging,
 * the CSV export and, on accounts the platform owes, "Record payment".
 */
import { ArrowLeft, ArrowUpRight, Download, HandCoins, Landmark, Percent } from "lucide-react";
import Link from "next/link";
import { Suspense, useState } from "react";
import { Button } from "@/components/ui/Button";
import {
  ListPagination,
  MetricTile,
  OperationsHeader,
  OperationsPanel,
  RefreshButton,
  readPage,
} from "@/components/ui/Operations";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { useMe } from "@/features/me";
import type { Schemas } from "@/lib/api";
import { count, dateTime } from "@/lib/format";
import { useRegulator } from "@/lib/scope";
import { useUrlState } from "@/lib/urlState";
import { ENTRIES_PAGE, useAccountEntries, useAccounts, useExportAccount, usePeriod } from "./hooks";
import { ACCOUNT_NOTE, type Account, accountName, amount, ENTRY_KIND, positionLabel, rate } from "./labels";
import { PaymentDialog } from "./PaymentDialog";
import { PeriodPicker } from "./PeriodPicker";

type Entry = Schemas["FinanceAccountEntry"];

/** Accounts kept per person show who each entry belongs to. */
const PER_PERSON = ["DRIVER_WALLET", "RIDER_DUES", "RIDER_REFUNDS"];

export function AccountDetail({ accountKey }: { accountKey: string }) {
  return (
    <Suspense fallback={<Skeleton className="h-96" />}>
      <Detail accountKey={accountKey} />
    </Suspense>
  );
}

function Detail({ accountKey }: { accountKey: string }) {
  const { period, set } = usePeriod();
  const accounts = useAccounts(period);
  const regulator = useRegulator();
  const account = accounts.data?.accounts.find((a) => a.key === accountKey);
  const name = account ? accountName(account, regulator.levy) : "Account";
  return (
    <div>
      <OperationsHeader
        title={name}
        description={account ? (ACCOUNT_NOTE[account.kind] ?? "") : "Ledger entries for this account."}
        actions={
          <>
            <Link
              href={`/finance/accounts?from=${period.from}&to=${period.to}`}
              className="inline-flex items-center gap-1.5 text-xs font-semibold text-muted hover:text-ink"
            >
              <ArrowLeft size={14} aria-hidden="true" /> Accounts
            </Link>
            <RefreshButton busy={accounts.isFetching} onClick={() => void accounts.refetch()} />
            {account ? <Actions account={account} name={name} /> : null}
          </>
        }
      />
      <div className="mb-4 rounded-xl border border-line bg-surface p-2">
        <PeriodPicker value={period} onChange={set} />
      </div>
      <ErrorNote error={accounts.error} />
      {accounts.isLoading ? (
        <Skeleton className="h-40" />
      ) : !account ? (
        <Empty
          title="Account not found"
          body="It may belong to another country. Pick the country in the top bar."
        />
      ) : (
        <div className="grid min-w-0 gap-5">
          <Figures account={account} />
          <Entries account={account} />
        </div>
      )}
    </div>
  );
}

function Actions({ account, name }: { account: Account; name: string }) {
  const me = useMe();
  const { period } = usePeriod();
  const exporter = useExportAccount();
  const [paying, setPaying] = useState(false);
  const canPay = account.settleable && me.can("finance.settle") && Number(account.balance) > 0;
  return (
    <>
      <Button
        variant="soft"
        size="sm"
        icon={<Download size={13} aria-hidden="true" />}
        loading={exporter.isPending}
        onClick={() => exporter.mutate({ key: account.key, period })}
      >
        Export CSV
      </Button>
      {canPay ? (
        <Button
          variant="primary"
          size="sm"
          icon={<HandCoins size={13} aria-hidden="true" />}
          onClick={() => setPaying(true)}
        >
          Record payment
        </Button>
      ) : null}
      <ErrorNote error={exporter.error} />
      <PaymentDialog account={account} name={name} open={paying} onClose={() => setPaying(false)} />
    </>
  );
}

function Figures({ account: a }: { account: Account }) {
  const label = positionLabel(a.position, Number(a.balance));
  const split =
    a.we_owe !== undefined && a.owed_to_us !== undefined
      ? `Owed to us ${amount(a.owed_to_us, a.currency)} · we owe ${amount(a.we_owe, a.currency)} (${count(a.holders)} accounts)`
      : `${count(a.entries)} entries in all`;
  return (
    <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
      <MetricTile
        label={`${label} now`}
        value={amount(Math.abs(Number(a.balance)), a.currency)}
        note={split}
        icon={<Landmark size={17} />}
        accent
      />
      <MetricTile
        label="Added in period"
        value={amount(a.added, a.currency)}
        note="Entries that grew the balance"
        icon={<ArrowUpRight size={17} />}
      />
      <MetricTile
        label="Paid or cleared in period"
        value={amount(a.reduced, a.currency)}
        note="Payments and other reductions"
        icon={<HandCoins size={17} />}
      />
      <MetricTile
        label="Effective rate"
        value={rate(a.rate)}
        note={
          a.fare_base
            ? `${amount(a.on_fares, a.currency)} on fares of ${amount(a.fare_base, a.currency)}`
            : "Not a share of fares"
        }
        icon={<Percent size={17} />}
      />
    </div>
  );
}

function Entries({ account }: { account: Account }) {
  const url = useUrlState();
  const { period } = usePeriod();
  const page = readPage(url.get("page"));
  const offset = page * ENTRIES_PAGE;
  const entries = useAccountEntries(account.key, period, offset);
  const perPerson = PER_PERSON.includes(account.kind);
  const head = ["Date", "Trip", "Description", ...(perPerson ? ["Who"] : []), "Amount", "Balance after"];
  const data = entries.data;
  return (
    <OperationsPanel title="Entries" description="Newest first. + grew the balance, − reduced it.">
      <ErrorNote error={entries.error} />
      {entries.isLoading ? (
        <Skeleton className="m-4 h-64" />
      ) : !data?.entries.length ? (
        <Empty title="No entries in this period" body="Pick a wider period above." />
      ) : (
        <div className="p-4">
          <Table head={head}>
            {data.entries.map((e) => (
              <EntryRow key={e.seq} entry={e} currency={account.currency} perPerson={perPerson} />
            ))}
          </Table>
        </div>
      )}
      <ListPagination
        total={data?.total ?? 0}
        offset={offset}
        pageSize={ENTRIES_PAGE}
        noun="entries"
        busy={entries.isFetching}
        onPage={(p) => url.set({ page: p ? String(p) : undefined })}
      />
    </OperationsPanel>
  );
}

function EntryRow({ entry: e, currency, perPerson }: { entry: Entry; currency: string; perPerson: boolean }) {
  const negative = Number(e.amount) < 0;
  return (
    <Row>
      <Cell className="tnum whitespace-nowrap text-xs text-muted">{dateTime(e.at)}</Cell>
      <Cell className="tnum whitespace-nowrap text-xs">
        {e.trip_id ? (
          <Link href={`/trips/${e.trip_id}`} className="font-semibold hover:text-brand-text">
            {e.trip_code ?? e.trip_id.slice(0, 8)}
          </Link>
        ) : (
          <span className="text-muted">—</span>
        )}
      </Cell>
      <Cell className="text-xs">
        <span className="block font-medium">{ENTRY_KIND[e.kind] ?? e.kind}</span>
        <span className="mt-1 block text-[11px] text-muted">{e.memo}</span>
      </Cell>
      {perPerson ? <Cell className="text-xs">{e.holder ?? "—"}</Cell> : null}
      <Cell className="tnum whitespace-nowrap text-xs font-semibold">
        {`${negative ? "−" : "+"}${amount(Math.abs(Number(e.amount)), currency)}`}
      </Cell>
      <Cell className="tnum whitespace-nowrap text-xs">{amount(e.running, currency)}</Cell>
    </Row>
  );
}
