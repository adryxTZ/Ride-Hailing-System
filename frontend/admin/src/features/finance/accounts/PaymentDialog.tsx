"use client";
/**
 * PaymentDialog — "Record payment" on an account the platform owes (VAT, levy,
 * a charge payee, rider refunds): amount, date, method and reference, with a
 * reason for the audit log. Rider refunds are paid oldest first, by choosing
 * refunds, or as one rider's balance. One Idempotency-Key per opening.
 */
import { useEffect, useState } from "react";
import { Field, Input, Select } from "@/components/ui/Field";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { ErrorNote } from "@/components/ui/States";
import { dateTime, money } from "@/lib/format";
import { day, type PaymentDraft, useOwedRefunds, useRecordPayment } from "./hooks";
import { type Account, amount, METHODS, type Method } from "./labels";

type RefundMode = "oldest" | "chosen" | "rider";

export function PaymentDialog({
  account,
  name,
  open,
  onClose,
}: {
  account: Account;
  name: string;
  open: boolean;
  onClose: () => void;
}) {
  const record = useRecordPayment();
  const refunds = account.kind === "RIDER_REFUNDS";
  const [draft, setDraft] = useState<PaymentDraft>(() => blank(account.key));
  const [mode, setMode] = useState<RefundMode>("oldest");
  useEffect(() => {
    if (open) {
      setDraft(blank(account.key));
      setMode("oldest");
      record.reset();
    }
    // a new key and a clean form every time the dialog opens
  }, [open, account.key]);
  const set = (patch: Partial<PaymentDraft>) => setDraft((d) => ({ ...d, ...patch }));
  const owed = Number(account.balance);
  return (
    <ReasonDialog
      open={open}
      title={`Record a payment to ${name}`}
      body={`We owe ${amount(account.balance, account.currency)}. The payment is posted from the platform bank and lowers what we owe.`}
      confirm="Record payment"
      busy={record.isPending}
      onClose={onClose}
      onConfirm={(reason) => {
        const amount = Number(draft.amount);
        if (!(amount > 0) || amount > owed || !draft.reference.trim()) return;
        record.mutate(payload(draft, mode, reason), { onSuccess: onClose });
      }}
    >
      {refunds ? <RefundTargets mode={mode} setMode={setMode} draft={draft} set={set} open={open} /> : null}
      <Field
        label={`Amount (${account.currency})`}
        hint={`Up to ${amount(account.balance, account.currency)}.`}
      >
        <Input
          inputMode="decimal"
          value={draft.amount}
          readOnly={refunds && mode === "chosen"}
          onChange={(e) => set({ amount: e.target.value.replace(/[^0-9.]/g, "") })}
        />
      </Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Paid on">
          <Input
            type="date"
            value={draft.paid_on}
            max={day(new Date())}
            onChange={(e) => set({ paid_on: e.target.value })}
          />
        </Field>
        <Field label="Method">
          <Select value={draft.method} onChange={(e) => set({ method: e.target.value as Method })}>
            {METHODS.map((m) => (
              <option key={m.value} value={m.value}>
                {m.label}
              </option>
            ))}
          </Select>
        </Field>
      </div>
      <Field label="Reference" hint="Bank or receipt reference, so the payment can be traced.">
        <Input value={draft.reference} maxLength={120} onChange={(e) => set({ reference: e.target.value })} />
      </Field>
      {Number(draft.amount) > owed ? (
        <p className="text-xs text-critical">That is more than we owe on this account.</p>
      ) : null}
      <ErrorNote error={record.error} />
    </ReasonDialog>
  );
}

function blank(key: string): PaymentDraft {
  return {
    key,
    idem: crypto.randomUUID(),
    amount: "",
    paid_on: day(new Date()),
    method: "BANK_TRANSFER",
    reference: "",
    reason: "",
  };
}

/** The request body for the chosen rider-refunds mode. */
function payload(d: PaymentDraft, mode: RefundMode, reason: string): PaymentDraft {
  const base = { ...d, reason, reference: d.reference.trim(), refund_ids: undefined, rider_id: undefined };
  if (mode === "chosen") return { ...base, refund_ids: d.refund_ids };
  if (mode === "rider") return { ...base, rider_id: d.rider_id };
  return base;
}

function RefundTargets({
  mode,
  setMode,
  draft,
  set,
  open,
}: {
  mode: RefundMode;
  setMode: (m: RefundMode) => void;
  draft: PaymentDraft;
  set: (p: Partial<PaymentDraft>) => void;
  open: boolean;
}) {
  const owed = useOwedRefunds(open);
  const list = owed.data?.refunds ?? [];
  const chosen = new Set(draft.refund_ids ?? []);
  const toggle = (id: string) => {
    const next = new Set(chosen);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    const ids = [...next];
    const sum = list.filter((r) => next.has(r.id)).reduce((s, r) => s + Number(r.amount), 0);
    set({ refund_ids: ids, amount: sum ? String(sum) : "" });
  };
  return (
    <div className="grid gap-3">
      <Field label="What are you paying?">
        <Select value={mode} onChange={(e) => setMode(e.target.value as RefundMode)}>
          <option value="oldest">Refunds, oldest first (whole refunds)</option>
          <option value="chosen">Refunds I choose</option>
          <option value="rider">One rider's balance (refunds and late fare payments)</option>
        </Select>
      </Field>
      {mode === "chosen" ? (
        <div className="max-h-48 overflow-auto rounded-[8px] border border-line">
          {list.length ? (
            list.map((r) => (
              <label
                key={r.id}
                className="flex items-center gap-3 border-b border-line px-3 py-2 text-xs last:border-b-0"
              >
                <input type="checkbox" checked={chosen.has(r.id)} onChange={() => toggle(r.id)} />
                <span className="tnum font-semibold">{money(r.amount, owed.data?.currency)}</span>
                <span className="min-w-0 flex-1 truncate text-muted">{`${r.rider} · trip ${r.trip_code} · ${dateTime(r.created_at)}`}</span>
              </label>
            ))
          ) : (
            <p className="px-3 py-2 text-xs text-muted">No unpaid refunds.</p>
          )}
        </div>
      ) : null}
      {mode === "rider" ? (
        <Field label="Rider">
          <Select
            value={draft.rider_id ?? ""}
            onChange={(e) => set({ rider_id: e.target.value || undefined })}
          >
            <option value="">Choose a rider</option>
            {(owed.data?.riders ?? []).map((r) => (
              <option key={r.rider_id} value={r.rider_id}>
                {`${r.rider} · owed ${money(r.owed, owed.data?.currency)}`}
              </option>
            ))}
          </Select>
        </Field>
      ) : null}
      <ErrorNote error={owed.error} />
    </div>
  );
}
