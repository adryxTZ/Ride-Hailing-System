/**
 * diff — a line diff between two versions of a text (longest common
 * subsequence). Legal texts are a few hundred lines, so the quadratic table
 * stays small; beyond MAX_LINES the texts are compared as whole blocks.
 */
export type DiffLine = { kind: "same" | "added" | "removed"; text: string };

/** Above this many lines on a side, show a removed block and an added block. */
const MAX_LINES = 2000;

export function lineDiff(before: string, after: string): DiffLine[] {
  const a = before.split("\n");
  const b = after.split("\n");
  if (a.length > MAX_LINES || b.length > MAX_LINES) {
    return [
      ...a.map((text) => ({ kind: "removed" as const, text })),
      ...b.map((text) => ({ kind: "added" as const, text })),
    ];
  }
  return walk(a, b, lcsTable(a, b));
}

/** table[i][j] = length of the common subsequence of a[i:] and b[j:]. */
function lcsTable(a: string[], b: string[]): number[][] {
  const table = Array.from({ length: a.length + 1 }, () => new Array<number>(b.length + 1).fill(0));
  for (let i = a.length - 1; i >= 0; i--) {
    for (let j = b.length - 1; j >= 0; j--) {
      table[i][j] = a[i] === b[j] ? table[i + 1][j + 1] + 1 : Math.max(table[i + 1][j], table[i][j + 1]);
    }
  }
  return table;
}

function walk(a: string[], b: string[], table: number[][]): DiffLine[] {
  const out: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) {
      out.push({ kind: "same", text: a[i] });
      i++;
      j++;
    } else if (table[i + 1][j] >= table[i][j + 1]) {
      out.push({ kind: "removed", text: a[i++] });
    } else {
      out.push({ kind: "added", text: b[j++] });
    }
  }
  while (i < a.length) out.push({ kind: "removed", text: a[i++] });
  while (j < b.length) out.push({ kind: "added", text: b[j++] });
  return out;
}

/** A run of unchanged lines hidden from the changes view. */
export type FoldLine = { kind: "fold"; count: number };

/** Unchanged lines kept around each change so it can be read in place. */
const CONTEXT_LINES = 2;

/** Keeps the changes and CONTEXT_LINES around each; longer unchanged runs become one fold. */
export function foldUnchanged(lines: DiffLine[]): (DiffLine | FoldLine)[] {
  const near = lines.map((_, i) =>
    lines.slice(Math.max(0, i - CONTEXT_LINES), i + CONTEXT_LINES + 1).some((l) => l.kind !== "same"),
  );
  const out: (DiffLine | FoldLine)[] = [];
  lines.forEach((l, i) => {
    if (near[i]) {
      out.push(l);
      return;
    }
    const last = out[out.length - 1];
    if (last?.kind === "fold") last.count++;
    else out.push({ kind: "fold", count: 1 });
  });
  return out;
}

/** How many lines were added and removed. */
export function diffCounts(lines: DiffLine[]): { added: number; removed: number } {
  return {
    added: lines.filter((l) => l.kind === "added").length,
    removed: lines.filter((l) => l.kind === "removed").length,
  };
}
