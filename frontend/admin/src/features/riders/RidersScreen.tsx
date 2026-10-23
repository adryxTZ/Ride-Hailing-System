"use client";
/** RidersScreen — riders by name or phone; a row opens the rider's page; block and unblock (blocking ends their sessions). */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Suspense, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/Chip";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { Search } from "@/components/ui/Search";
import { Empty, ErrorNote, Skeleton } from "@/components/ui/States";
import { Cell, Row, Table } from "@/components/ui/Table";
import { useMe } from "@/features/me";
import { api, unwrap } from "@/lib/api";
import { when } from "@/lib/format";
import { useScope } from "@/lib/scope";
import { useUrlState } from "@/lib/urlState";

const PAGE = 50;

export function RidersScreen() {
  return (
    <Suspense>
      <Riders />
    </Suspense>
  );
}

function Riders() {
  const url = useUrlState();
  const q = url.get("q");
  const qc = useQueryClient();
  const me = useMe();
  const scope = useScope();
  const router = useRouter();
  const [target, setTarget] = useState<{ id: string; block: boolean; name: string }>();
  const { data, isLoading, error } = useQuery({
    queryKey: ["admin", "riders", q, scope.query],
    queryFn: async () =>
      unwrap(
        await api.GET("/admin/riders", {
          params: { query: { ...scope.query, q: q || undefined, limit: PAGE } },
        }),
      ),
    placeholderData: (prev) => prev,
  });
  const setStatus = useMutation({
    mutationFn: async (a: { id: string; block: boolean; reason: string }) => {
      const opts = { params: { path: { id: a.id } }, body: { reason: a.reason } };
      return unwrap(
        a.block
          ? await api.POST("/admin/riders/{id}/block", opts)
          : await api.POST("/admin/riders/{id}/unblock", opts),
      );
    },
    onSuccess: () => {
      setTarget(undefined);
      void qc.invalidateQueries({ queryKey: ["admin", "riders"] });
    },
  });
  return (
    <div>
      <PageHeader
        title="Riders"
        subtitle="Everyone who books rides and packages."
        actions={
          <Search
            value={q}
            onChange={(v) => url.set({ q: v || undefined })}
            placeholder="Name, phone or email"
          />
        }
      />
      <ErrorNote error={error ?? setStatus.error} />
      <Card index={1}>
        {isLoading ? (
          <Skeleton className="h-64" />
        ) : !data?.data.length ? (
          <Empty title="No riders found" body="Try another name or phone number." />
        ) : (
          <Table head={["Name", "Phone", "Joined", "Status", "Details", ""]}>
            {data.data.map((r) => (
              <Row key={r.id} onClick={() => router.push(`/riders/${r.id}`)}>
                <Cell className="font-medium">{r.full_name ?? "No name"}</Cell>
                <Cell className="tnum text-muted">{r.phone}</Cell>
                <Cell className="tnum text-muted">{when(r.created_at)}</Cell>
                <Cell>
                  {r.status === "ACTIVE" ? <Chip tone="ok">Active</Chip> : <Chip tone="bad">Blocked</Chip>}
                </Cell>
                <Cell>
                  <Link
                    className="text-sm font-semibold text-brand-text"
                    href={`/riders/${r.id}`}
                    onClick={(e) => e.stopPropagation()}
                  >
                    Open
                  </Link>
                </Cell>
                <Cell className="text-right">
                  {me.can("riders.block") ? (
                    <Button
                      size="sm"
                      variant={r.status === "ACTIVE" ? "ghost" : "soft"}
                      onClick={(e) => {
                        e.stopPropagation();
                        setTarget({ id: r.id, block: r.status === "ACTIVE", name: r.full_name ?? r.phone });
                      }}
                    >
                      {r.status === "ACTIVE" ? "Block" : "Unblock"}
                    </Button>
                  ) : null}
                </Cell>
              </Row>
            ))}
          </Table>
        )}
        <p className="tnum mt-4 text-sm text-muted">{`${data?.total ?? 0} riders`}</p>
      </Card>
      <ReasonDialog
        open={!!target}
        title={target?.block ? `Block ${target.name}?` : `Unblock ${target?.name ?? ""}?`}
        body={
          target?.block
            ? "They are signed out and cannot sign in or book until unblocked."
            : "They can sign in and book again."
        }
        confirm={target?.block ? "Block" : "Unblock"}
        danger={target?.block}
        busy={setStatus.isPending}
        onClose={() => setTarget(undefined)}
        onConfirm={(reason) => target && setStatus.mutate({ id: target.id, block: target.block, reason })}
      />
    </div>
  );
}
