"use client";
/** JournalCard — recent ledger transactions, filtered by kind and shown a page at a time; each row opens to show its legs. */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Select } from "@/components/ui/Field";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { money, when } from "@/lib/format";
import { useJournal } from "./hooks";
import { ACCOUNT_LABEL, humanize } from "./labels";

const PAGE = 25;

export function JournalCard({ index, currency }: { index: number; currency: string }) {
  const { data, isLoading, error } = useJournal();
  const [open, setOpen] = useState<string>();
  const [kind, setKind] = useState("");
  const [shown, setShown] = useState(PAGE);
  const kinds = [...new Set(data?.data.map((tx) => tx.kind) ?? [])].sort();
  const rows = (data?.data ?? []).filter((tx) => !kind || tx.kind === kind);
  return (
    <Card
      index={index}
      title="Journal"
      action={
        <Select
          aria-label="Kind"
          value={kind}
          onChange={(e) => {
            setKind(e.target.value);
            setShown(PAGE);
          }}
          className="h-8 w-auto text-xs"
        >
          <option value="">All kinds</option>
          {kinds.map((k) => (
            <option key={k} value={k}>
              {humanize(k)}
            </option>
          ))}
        </Select>
      }
    >
      <ErrorNote error={error} />
      {isLoading ? (
        <Skeleton className="h-64" />
      ) : !rows.length ? (
        <Empty
          title="No ledger entries yet"
          body="Entries appear when trips complete, fees are charged or debts are paid."
        />
      ) : (
        <>
          <Table head={["When", "Kind", "Memo", "Amount"]}>
            {rows.slice(0, shown).map((tx) => (
              <Row key={tx.id} onClick={() => setOpen(open === tx.id ? undefined : tx.id)}>
                <Cell className="tnum whitespace-nowrap text-muted">{when(tx.at)}</Cell>
                <Cell className="whitespace-nowrap font-medium">{humanize(tx.kind)}</Cell>
                <Cell>
                  <span className="text-muted">{tx.memo || tx.reference}</span>
                  {open === tx.id ? (
                    <ul className="mt-2 grid gap-1 text-xs">
                      {tx.legs.map((l, i) => (
                        <li key={`${l.account}-${i}`} className="flex justify-between gap-4">
                          <span>{ACCOUNT_LABEL[l.account] ?? humanize(l.account)}</span>
                          <span className="tnum font-mono">{money(l.amount, currency)}</span>
                        </li>
                      ))}
                    </ul>
                  ) : null}
                </Cell>
                <Cell className="tnum whitespace-nowrap text-right">{money(tx.amount, currency)}</Cell>
              </Row>
            ))}
          </Table>
          {rows.length > shown ? (
            <div className="mt-4 flex justify-center">
              <Button variant="soft" size="sm" onClick={() => setShown(shown + PAGE)}>
                {`Show ${Math.min(PAGE, rows.length - shown)} more`}
              </Button>
            </div>
          ) : null}
        </>
      )}
    </Card>
  );
}
