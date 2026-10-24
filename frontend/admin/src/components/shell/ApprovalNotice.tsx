"use client";
/**
 * ApprovalNotice — the one "Sent for approval" message: when a gated action's
 * workflow holds a change (202), it says so, names the change and links to
 * the request. It closes by itself after a few seconds, or with the button.
 */
import { useQueryClient } from "@tanstack/react-query";
import { ClipboardCheck, X } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { EASE } from "@/components/ui/motion";
import { type ApprovalRequest, onSentForApproval } from "@/lib/approval";

/** How long the notice stays when nobody closes it. */
const SHOW_MS = 9000;

export function ApprovalNotice() {
  const qc = useQueryClient();
  const [request, setRequest] = useState<ApprovalRequest>();
  useEffect(
    () =>
      onSentForApproval((r) => {
        setRequest(r);
        void qc.invalidateQueries({ queryKey: ["admin", "approvals"] });
      }),
    [qc],
  );
  useEffect(() => {
    if (!request) return;
    const t = setTimeout(() => setRequest(undefined), SHOW_MS);
    return () => clearTimeout(t);
  }, [request]);
  return (
    <AnimatePresence>
      {request ? (
        <motion.div
          role="status"
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: 12 }}
          transition={{ duration: 0.26, ease: EASE }}
          className="fixed bottom-4 right-4 z-50 flex w-[min(380px,calc(100vw-32px))] items-start gap-3 rounded-[10px] border border-line bg-surface p-4 shadow-lg"
        >
          <ClipboardCheck size={18} className="mt-0.5 shrink-0 text-brand-text" aria-hidden="true" />
          <div className="grid min-w-0 flex-1 gap-1 text-sm">
            <span className="font-semibold">Sent for approval</span>
            <span className="text-xs text-muted">
              {`${request.summary.title || request.action_label}. It applies once approved.`}
            </span>
            <Link
              href={`/approvals/${request.id}`}
              onClick={() => setRequest(undefined)}
              className="text-xs font-semibold text-brand-text"
            >
              View request
            </Link>
          </div>
          <button
            type="button"
            aria-label="Close"
            onClick={() => setRequest(undefined)}
            className="grid size-7 shrink-0 place-items-center rounded-[8px] text-muted hover:bg-surface-2"
          >
            <X size={14} aria-hidden="true" />
          </button>
        </motion.div>
      ) : null}
    </AnimatePresence>
  );
}
