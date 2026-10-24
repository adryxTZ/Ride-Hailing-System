"use client";
/**
 * HistoryCard — every version of the document: pick one to see what changed
 * against the version before it, or its text, and copy it into the editor
 * (to restore wording from an earlier version as a new draft).
 */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { Cell, Row, Table } from "@/components/ui/Table";
import { dateTime } from "@/lib/format";
import { CompareView } from "./CompareView";
import type { LegalDoc } from "./hooks";
import { STATUS_LABEL, STATUS_TONE } from "./labels";

export function HistoryCard({
  versions,
  lang,
  onCopy,
  index,
}: {
  versions: LegalDoc[];
  lang: string;
  onCopy: (v: LegalDoc) => void;
  index: number;
}) {
  const [picked, setPicked] = useState<string>();
  const at = versions.findIndex((v) => v.id === picked);
  return (
    <Card title="Version history" index={index}>
      <Table head={["Version", "Status", "Published", "Reason", ""]}>
        {versions.map((v) => (
          <Row
            key={v.id}
            selected={v.id === picked}
            onClick={() => setPicked(v.id === picked ? undefined : v.id)}
          >
            <Cell className="tnum font-semibold">{`v${v.version}`}</Cell>
            <Cell>
              <Chip tone={STATUS_TONE[v.status]}>{STATUS_LABEL[v.status]}</Chip>
              {v.source === "STARTER" ? (
                <span className="ml-1.5 text-[11px] text-muted">starter template</span>
              ) : null}
            </Cell>
            <Cell className="tnum whitespace-nowrap text-xs text-muted">
              {v.published_at ? dateTime(v.published_at) : `Saved ${dateTime(v.updated_at)}`}
            </Cell>
            <Cell className="max-w-[320px] truncate text-xs text-muted">{v.change_reason ?? ""}</Cell>
            <Cell className="whitespace-nowrap text-right">
              {v.status !== "DRAFT" ? (
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={(e) => {
                    e.stopPropagation();
                    onCopy(v);
                  }}
                >
                  Copy into editor
                </Button>
              ) : null}
            </Cell>
          </Row>
        ))}
      </Table>
      {at >= 0 ? <CompareView version={versions[at]} previous={versions[at + 1]} lang={lang} /> : null}
      {at < 0 ? <p className="mt-3 text-xs text-muted">Pick a version to see what changed.</p> : null}
    </Card>
  );
}
