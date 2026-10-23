"use client";
import { Button } from "@/components/ui/Button";
/** VersionsTable — every theme version: open one to edit (as a new draft), roll back, or delete a draft. */
import { Chip } from "@/components/ui/Chip";
import { Cell, Row, Table } from "@/components/ui/Table";
import type { Schemas } from "@/lib/api";
import { when } from "@/lib/format";

const TONE = { PUBLISHED: "ok", DRAFT: "live", ARCHIVED: "muted" } as const;
const LABEL = { PUBLISHED: "Live", DRAFT: "Draft", ARCHIVED: "Earlier" } as const;

/** "Preset · Deep navy", "Color Hunt", "Custom". */
function sourceText(source: string): string {
  if (source.startsWith("PRESET:")) return "Preset";
  if (source.startsWith("IMPORTED:")) return "Color Hunt";
  return "Custom";
}

export function VersionsTable({
  versions,
  onOpen,
  onRollback,
  onDelete,
}: {
  versions: Schemas["ThemeVersion"][];
  onOpen: (v: Schemas["ThemeVersion"]) => void;
  onRollback: (v: Schemas["ThemeVersion"]) => void;
  onDelete: (v: Schemas["ThemeVersion"]) => void;
}) {
  return (
    <Table head={["Version", "Colours", "Name", "From", "Status", "Published", "Reason", ""]}>
      {versions.map((v) => (
        <Row key={v.id}>
          <Cell className="tnum font-semibold">{v.version}</Cell>
          <Cell>
            <span className="flex h-5 w-20 overflow-hidden rounded-[4px]">
              {[v.palette.primary, v.palette.secondary, v.palette.accent, v.palette.background].map(
                (c, i) => (
                  <span key={`${c}${i}`} className="flex-1" style={{ background: c }} />
                ),
              )}
            </span>
          </Cell>
          <Cell>{v.name || `Version ${v.version}`}</Cell>
          <Cell className="text-muted">{sourceText(v.source)}</Cell>
          <Cell>
            <Chip tone={TONE[v.status]}>{LABEL[v.status]}</Chip>
            {v.report.pass ? null : <span className="ml-1 text-xs text-critical">contrast fails</span>}
          </Cell>
          <Cell className="tnum whitespace-nowrap text-muted">
            {v.published_at ? when(v.published_at) : ""}
          </Cell>
          <Cell className="max-w-[200px] truncate text-muted">{v.change_reason ?? ""}</Cell>
          <Cell className="whitespace-nowrap text-right">
            <Button size="sm" variant="ghost" onClick={() => onOpen(v)}>
              {v.status === "DRAFT" ? "Edit" : "Copy to draft"}
            </Button>
            {v.status === "ARCHIVED" ? (
              <Button size="sm" variant="soft" onClick={() => onRollback(v)}>
                Roll back
              </Button>
            ) : null}
            {v.status === "DRAFT" ? (
              <Button size="sm" variant="ghost" onClick={() => onDelete(v)}>
                Delete
              </Button>
            ) : null}
          </Cell>
        </Row>
      ))}
    </Table>
  );
}
