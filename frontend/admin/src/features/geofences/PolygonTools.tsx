"use client";
/** PolygonTools — choose how to draw (corners or a rectangle), undo the last corner, or start again. */
import { Eraser, Undo2 } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { FilterBar } from "@/components/ui/Operations";
import type { Draft, PolygonTool } from "./GeofenceMap";

const TOOLS = [
  { value: "corners", label: "Corners" },
  { value: "rectangle", label: "Rectangle" },
] as const;

export function PolygonTools({ draft, onDraft }: { draft: Draft; onDraft: (d: Draft) => void }) {
  const tool: PolygonTool = draft.tool ?? "corners";
  const hint =
    tool === "rectangle"
      ? draft.points.length === 1
        ? "Click the opposite corner."
        : "Click one corner, then the opposite one. Drag a corner to adjust."
      : `${draft.points.length} corners · click the map to add, drag a corner to move it`;
  return (
    <div className="grid gap-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <FilterBar
          label="Drawing tool"
          value={tool}
          options={TOOLS}
          onChange={(t) => onDraft({ ...draft, tool: t, points: [] })}
        />
        <div className="flex gap-1">
          <Button
            size="sm"
            variant="ghost"
            icon={<Undo2 size={14} aria-hidden="true" />}
            disabled={!draft.points.length || tool === "rectangle"}
            onClick={() => onDraft({ ...draft, points: draft.points.slice(0, -1) })}
          >
            Undo
          </Button>
          <Button
            size="sm"
            variant="ghost"
            icon={<Eraser size={14} aria-hidden="true" />}
            disabled={!draft.points.length}
            onClick={() => onDraft({ ...draft, points: [] })}
          >
            Clear
          </Button>
        </div>
      </div>
      <p className="text-xs text-muted">{hint}</p>
    </div>
  );
}
