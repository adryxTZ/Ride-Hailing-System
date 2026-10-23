"use client";
/** AssetsEditor — one row per logo or icon slot: what is there, upload a new image, or remove it. */
import { ImageUp, Trash2 } from "lucide-react";
import { useRef, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { ErrorNote } from "@/components/ui/States";
import { apiUrl } from "@/lib/config";
import { type Draft, SLOT_LABEL } from "./draft";
import { useUploadAsset } from "./hooks";

const ACCEPT = "image/png,image/webp,image/jpeg";

export function AssetsEditor({
  draft,
  slots,
  urls,
  onChange,
}: {
  draft: Draft;
  slots: string[];
  /** Public links for keys the saved version already has. */
  urls: Record<string, string>;
  onChange: (d: Draft) => void;
}) {
  const upload = useUploadAsset();
  const [busy, setBusy] = useState<string>();
  const set = (slot: string, key?: string) => {
    const assets = { ...(draft.assets ?? {}) };
    if (key) assets[slot] = key;
    else delete assets[slot];
    onChange({ ...draft, assets });
  };
  return (
    <div className="grid gap-2">
      <ErrorNote error={upload.error} />
      {slots.map((slot) => (
        <SlotRow
          key={slot}
          slot={slot}
          assetKey={draft.assets?.[slot]}
          url={urls[slot]}
          busy={busy === slot}
          onPick={(file) => {
            setBusy(slot);
            upload.mutate(file, { onSuccess: (key) => set(slot, key), onSettled: () => setBusy(undefined) });
          }}
          onRemove={() => set(slot)}
        />
      ))}
      <p className="text-xs text-muted">
        PNG, WebP or JPEG, at most 2 MB each. New images show here once the draft is saved.
      </p>
    </div>
  );
}

function SlotRow({
  slot,
  assetKey,
  url,
  busy,
  onPick,
  onRemove,
}: {
  slot: string;
  assetKey?: string;
  url?: string;
  busy: boolean;
  onPick: (f: File) => void;
  onRemove: () => void;
}) {
  const input = useRef<HTMLInputElement>(null);
  const meta = SLOT_LABEL[slot] ?? { label: slot, hint: "" };
  const saved = assetKey && url?.endsWith(assetKey) ? url : undefined;
  return (
    <div className="flex flex-wrap items-center gap-3 rounded-[8px] bg-surface-2 px-3 py-2">
      <span className="grid size-12 shrink-0 place-items-center overflow-hidden rounded-[8px] bg-surface">
        {saved ? (
          <img src={apiUrl(saved)} alt={meta.label} className="size-full object-contain" />
        ) : (
          <ImageUp size={18} className="text-muted" aria-hidden="true" />
        )}
      </span>
      <div className="min-w-0 flex-1">
        <p className="flex items-center gap-2 text-sm font-medium">
          {meta.label}
          {meta.release ? <Chip tone="warn">Needs an app release</Chip> : null}
          {assetKey && !saved ? <Chip tone="live">New, not saved</Chip> : null}
        </p>
        <p className="truncate text-xs text-muted">{meta.hint}</p>
      </div>
      <input
        ref={input}
        type="file"
        accept={ACCEPT}
        className="hidden"
        onChange={(e) => {
          const f = e.target.files?.[0];
          if (f) onPick(f);
          e.target.value = "";
        }}
      />
      <Button size="sm" variant="soft" loading={busy} onClick={() => input.current?.click()}>
        {assetKey ? "Replace" : "Upload"}
      </Button>
      {assetKey ? (
        <button
          type="button"
          aria-label={`Remove ${meta.label}`}
          onClick={onRemove}
          className="grid size-8 place-items-center rounded-[8px] text-muted hover:bg-surface"
        >
          <Trash2 size={15} aria-hidden="true" />
        </button>
      ) : null}
    </div>
  );
}
