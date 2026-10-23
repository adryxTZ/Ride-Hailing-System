"use client";
/** ReasonDialog — every audited action asks why; the reason lands in the audit log. */
import { AnimatePresence, motion } from "motion/react";
import { useState } from "react";
import { Button } from "./Button";
import { Field, Input } from "./Field";
import { EASE } from "./motion";

interface Props {
  open: boolean;
  title: string;
  body?: string;
  confirm: string;
  danger?: boolean;
  busy?: boolean;
  onConfirm: (reason: string) => void;
  onClose: () => void;
  /** Extra fields shown above the reason (e.g. a new record's name). */
  children?: React.ReactNode;
}

export function ReasonDialog({
  open,
  title,
  body,
  confirm,
  danger,
  busy,
  onConfirm,
  onClose,
  children,
}: Props) {
  const [reason, setReason] = useState("");
  return (
    <AnimatePresence>
      {open ? (
        <motion.div
          className="fixed inset-0 z-50 grid place-items-center p-4"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
        >
          <button
            type="button"
            aria-label="Close"
            className="absolute inset-0 bg-black/30"
            onClick={onClose}
          />
          <motion.form
            role="dialog"
            aria-label={title}
            initial={{ y: 12, opacity: 0, scale: 0.98 }}
            animate={{ y: 0, opacity: 1, scale: 1 }}
            transition={{ duration: 0.35, ease: EASE }}
            onSubmit={(e) => {
              e.preventDefault();
              if (reason.trim()) onConfirm(reason.trim());
            }}
            className="relative grid w-full max-w-md gap-4 rounded-[10px] bg-surface p-6"
          >
            <h2 className="text-lg font-semibold">{title}</h2>
            {body ? <p className="text-sm text-muted">{body}</p> : null}
            {children}
            <Field label="Reason" hint="Saved in the audit log with your name.">
              <Input
                autoFocus={!children}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="Why are you doing this?"
              />
            </Field>
            <div className="flex justify-end gap-2">
              <Button variant="ghost" onClick={onClose}>
                Cancel
              </Button>
              <Button
                type="submit"
                variant={danger ? "danger" : "primary"}
                loading={busy}
                disabled={!reason.trim()}
              >
                {confirm}
              </Button>
            </div>
          </motion.form>
        </motion.div>
      ) : null}
    </AnimatePresence>
  );
}
