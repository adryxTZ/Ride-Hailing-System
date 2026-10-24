"use client";
/** PlaceholdersCard — every {{key}} a text may use, with today's value; Insert puts it at the cursor. */
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Cell, Row, Table } from "@/components/ui/Table";
import type { Placeholder } from "./hooks";

const SOURCE_LABEL: Record<Placeholder["source"], string> = {
  BRAND: "Branding",
  COUNTRY: "Country",
  DETAILS: "Legal details",
  DOCUMENT: "This version",
};

export function PlaceholdersCard({
  placeholders,
  onInsert,
  index,
}: {
  placeholders: Placeholder[];
  onInsert: (text: string) => void;
  index: number;
}) {
  return (
    <Card title="Placeholders" index={index}>
      <p className="mb-3 text-xs text-muted">
        Filled in when the text is shown, so the same text works for every brand and country.
      </p>
      <Table head={["Placeholder", "Value today", "From", ""]}>
        {placeholders.map((p) => (
          <Row key={p.key}>
            <Cell className="font-mono text-xs">{`{{${p.key}}}`}</Cell>
            <Cell className={`text-xs ${p.value ? "" : "text-warn"}`}>{p.value || "Not set"}</Cell>
            <Cell className="text-xs text-muted">{SOURCE_LABEL[p.source]}</Cell>
            <Cell className="text-right">
              <Button size="sm" variant="ghost" onClick={() => onInsert(`{{${p.key}}}`)}>
                Insert
              </Button>
            </Cell>
          </Row>
        ))}
      </Table>
    </Card>
  );
}
