"use client";
/** NotesCard — staff notes on a person (support history), newest first, with a box to add one. */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Empty, ErrorNote } from "@/components/ui/States";
import type { Schemas } from "@/lib/api";
import { when } from "@/lib/format";
import { useAddNote } from "./hooks";

const NOTE_MAX = 2000;

export function NotesCard({
  id,
  notes,
  index,
}: {
  id: string;
  notes: Schemas["PersonNote"][];
  index: number;
}) {
  const [draft, setDraft] = useState("");
  const add = useAddNote();
  const save = () => add.mutate({ id, body: draft.trim() }, { onSuccess: () => setDraft("") });
  return (
    <Card index={index} title="Staff notes">
      <ErrorNote error={add.error} />
      <label className="grid gap-2 text-sm">
        <span className="sr-only">New note</span>
        <textarea
          value={draft}
          maxLength={NOTE_MAX}
          rows={3}
          onChange={(e) => setDraft(e.target.value)}
          placeholder="What happened, what you agreed, what to watch for"
          className="w-full resize-y rounded-[8px] bg-surface-2 px-3 py-2 text-sm outline-none placeholder:text-muted focus:ring-2 focus:ring-brand"
        />
      </label>
      <div className="mt-2 flex items-center justify-between">
        <span className="tnum text-xs text-muted">{`${draft.length} / ${NOTE_MAX}`}</span>
        <Button size="sm" disabled={!draft.trim()} loading={add.isPending} onClick={save}>
          Add note
        </Button>
      </div>
      {notes.length === 0 ? (
        <div className="mt-4">
          <Empty title="No notes yet" body="Notes are visible to every staff member who opens this page." />
        </div>
      ) : (
        <ul className="mt-4 grid gap-2">
          {notes.map((n) => (
            <li key={n.id} className="rounded-[8px] bg-surface-2 px-3 py-2 text-sm">
              <p className="whitespace-pre-wrap">{n.body}</p>
              <p className="tnum mt-1 text-xs text-muted">{`${n.author_name ?? "Staff"} · ${when(n.created_at)}`}</p>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
