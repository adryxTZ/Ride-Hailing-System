"use client";
/** VersionsCard — every published version; pick two to compare what changed in the rules and charges. */
import { useState } from "react";
import { Chip } from "@/components/ui/Chip";
import { Empty } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import type { Schemas } from "@/lib/api";
import { when } from "@/lib/format";
import { showTerm, TERM_FIELDS } from "./model";

type Version = Schemas["ProfileVersion"];

export function VersionsCard({ versions, currency }: { versions: Version[]; currency: string }) {
  const [picked, setPicked] = useState<number[]>(
    versions.length > 1 ? [versions[1].version, versions[0].version] : [],
  );
  if (!versions.length)
    return <Empty title="Nothing published yet" body="Publish the rules and charges to create version 1." />;
  const now = Date.now();
  const live = versions.find((v) => new Date(v.effective_from).getTime() <= now);
  const toggle = (n: number) =>
    setPicked((p) => (p.includes(n) ? p.filter((x) => x !== n) : [...p.slice(-1), n]));
  const [a, b] = [...picked].sort((x, y) => x - y).map((n) => versions.find((v) => v.version === n));
  return (
    <div className="grid gap-4">
      <Table head={["Compare", "Version", "In force from", "Charges", "Reason", ""]}>
        {versions.map((v) => (
          <Row key={v.id}>
            <Cell>
              <input
                type="checkbox"
                aria-label={`Compare version ${v.version}`}
                checked={picked.includes(v.version)}
                onChange={() => toggle(v.version)}
                className="accent-[var(--brand)]"
              />
            </Cell>
            <Cell className="tnum font-semibold">{`v${v.version}`}</Cell>
            <Cell className="tnum whitespace-nowrap text-muted">{when(v.effective_from)}</Cell>
            <Cell className="tnum">{v.charges.length}</Cell>
            <Cell className="max-w-[260px] truncate text-muted">{v.reason}</Cell>
            <Cell>
              {v.id === live?.id ? (
                <Chip tone="ok">In force</Chip>
              ) : new Date(v.effective_from).getTime() > now ? (
                <Chip tone="live">Scheduled</Chip>
              ) : null}
            </Cell>
          </Row>
        ))}
      </Table>
      {a && b ? (
        <Diff a={a} b={b} currency={currency} />
      ) : (
        <p className="text-sm text-muted">Tick two versions to compare them.</p>
      )}
    </div>
  );
}

/** What changed from a to b: rules that differ, and charges added, removed or changed. */
function Diff({ a, b, currency }: { a: Version; b: Version; currency: string }) {
  const rules = TERM_FIELDS.filter(
    (f) => JSON.stringify(a.terms[f.key] ?? null) !== JSON.stringify(b.terms[f.key] ?? null),
  );
  const byCode = (v: Version) => new Map(v.charges.map((c) => [c.code, c]));
  const ca = byCode(a),
    cb = byCode(b);
  const codes = [...new Set([...ca.keys(), ...cb.keys()])].filter(
    (c) => JSON.stringify(ca.get(c)) !== JSON.stringify(cb.get(c)),
  );
  if (!rules.length && !codes.length)
    return <p className="text-sm text-muted">{`v${a.version} and v${b.version} are the same.`}</p>;
  return (
    <div className="rounded-[8px] bg-surface-2 p-3 text-sm">
      <p className="mb-2 font-semibold">{`From v${a.version} to v${b.version}`}</p>
      <ul className="grid gap-1">
        {rules.map((f) => (
          <li key={f.key} className="tnum">
            {`${f.label}: ${showTerm(f, a.terms[f.key], currency)} → ${showTerm(f, b.terms[f.key], currency)}`}
          </li>
        ))}
        {codes.map((code) => {
          const before = ca.get(code),
            after = cb.get(code);
          const text = !before ? "added" : !after ? "removed" : "changed";
          return <li key={code}>{`Charge ${(after ?? before)?.label.en ?? code} ${text}`}</li>;
        })}
      </ul>
    </div>
  );
}
