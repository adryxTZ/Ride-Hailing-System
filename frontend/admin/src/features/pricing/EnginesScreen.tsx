"use client";
/** EnginesScreen — the pricing engines the Platform can use and which vehicle types use each one. */
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { PageHeader } from "@/components/ui/PageHeader";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { serviceLabel, useClassName } from "@/lib/catalog";
import { usePricing } from "./hooks";

/** The settings an engine asks for, from its JSON schema. */
function settingNames(schema: Record<string, unknown>): string[] {
  const props = (schema.properties ?? {}) as Record<string, { title?: string }>;
  return Object.entries(props).map(([k, v]) => v.title ?? k.replace(/_/g, " "));
}

export function EnginesScreen() {
  const { data, isLoading, error } = usePricing();
  const className = useClassName();
  return (
    <div>
      <PageHeader
        title="Engines"
        subtitle="How a fare is worked out. Each vehicle type picks one engine; new engines are added in code and appear here."
      />
      <ErrorNote error={error} />
      {isLoading || !data ? (
        <Skeleton className="h-64" />
      ) : (
        <Card index={1}>
          <Table head={["Engine", "Key", "Settings", "Used by"]}>
            {data.engines.map((e) => {
              const users = data.tariffs.filter((t) => t.engine === e.key);
              return (
                <Row key={e.key}>
                  <Cell className="font-medium">{e.name}</Cell>
                  <Cell className="font-mono text-xs text-muted">{e.key}</Cell>
                  <Cell className="text-sm text-muted">
                    {settingNames(e.settings_schema).join(", ") || "None"}
                  </Cell>
                  <Cell>
                    <div className="flex flex-wrap gap-1.5">
                      {users.length ? (
                        users.map((t) => (
                          <Chip
                            key={`${t.service}/${t.vehicle_class}`}
                            tone="brand"
                          >{`${serviceLabel(t.service)} · ${className(t.vehicle_class)}`}</Chip>
                        ))
                      ) : (
                        <span className="text-xs text-muted">Not in use</span>
                      )}
                    </div>
                  </Cell>
                </Row>
              );
            })}
          </Table>
        </Card>
      )}
    </div>
  );
}
