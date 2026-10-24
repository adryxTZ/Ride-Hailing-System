/** ChangeTable — what a held change does, field by field: before and after. */
import { Empty } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import type { ApprovalRequest } from "./hooks";
import { fieldLabel } from "./labels";

/** How an empty side of a change reads. */
const NOTHING = "—";

export function ChangeTable({ changes }: { changes: ApprovalRequest["summary"]["changes"] }) {
  if (!changes.length) return <Empty title="No field changes" body="The title above says what this does." />;
  return (
    <Table head={["Field", "Now", "After approval"]}>
      {changes.map((c) => (
        <Row key={c.field}>
          <Cell className="text-xs font-medium">{fieldLabel(c.field)}</Cell>
          <Cell className="max-w-[260px] break-words font-mono text-xs text-muted">
            {c.before || NOTHING}
          </Cell>
          <Cell className="max-w-[260px] break-words font-mono text-xs font-semibold">
            {c.after || NOTHING}
          </Cell>
        </Row>
      ))}
    </Table>
  );
}
