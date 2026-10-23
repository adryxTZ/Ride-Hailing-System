"use client";
/**
 * DocumentAttachments — the driver's documents as a compact attachment list
 * (file type, size, uploaded, expiry, review state) instead of large previews;
 * Open shows the file in a new tab, and staff approve or reject in the row.
 */
import { Check, ExternalLink, FileImage, FileText, FileX, X } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import type { Schemas } from "@/lib/api";
import { apiUrl } from "@/lib/config";
import { fileSize, when } from "@/lib/format";
import { DOC_LABEL, DOC_STATUS } from "./labels";

const EXT: Record<string, string> = {
  "image/jpeg": "JPG",
  "image/png": "PNG",
  "image/webp": "WEBP",
  "application/pdf": "PDF",
};

type Doc = Schemas["Document"];

export function DocumentAttachments({
  requirements,
  documents,
  canReview,
  onApprove,
  onReject,
}: {
  requirements: Schemas["Requirement"][];
  documents: Doc[];
  canReview: boolean;
  onApprove: (docId: string) => void;
  onReject: (docId: string) => void;
}) {
  return (
    <ul className="grid gap-1.5">
      {requirements.map((r) => {
        const doc = documents.find((x) => x.kind === r.kind);
        const name = r.label?.en ?? DOC_LABEL[r.kind] ?? r.kind;
        return (
          <li key={r.kind} className="flex flex-wrap items-center gap-3 rounded-[8px] bg-surface-2 px-3 py-2">
            <FileIcon doc={doc} />
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">
                {name}
                {r.mandatory ? null : <span className="font-normal text-muted"> (optional)</span>}
              </p>
              <p className="tnum truncate text-xs text-muted">
                {doc
                  ? [
                      EXT[doc.content_type] ?? doc.content_type,
                      fileSize(doc.size_bytes),
                      `uploaded ${when(doc.created_at)}`,
                      doc.expires_on ? `expires ${doc.expires_on}` : undefined,
                    ]
                      .filter(Boolean)
                      .join(" · ")
                  : "Not uploaded"}
              </p>
            </div>
            {doc ? <ExpiryChip doc={doc} /> : null}
            {doc ? (
              <Chip tone={DOC_STATUS[doc.status]?.tone}>{DOC_STATUS[doc.status]?.label ?? doc.status}</Chip>
            ) : null}
            {doc?.renewal ? <Chip tone="live">Renewal waiting</Chip> : null}
            <div className="flex gap-1">
              {doc?.url ? (
                <a
                  href={apiUrl(doc.url)}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex h-8 items-center gap-1.5 rounded-[8px] px-3 text-xs font-semibold text-brand-text hover:bg-surface"
                  aria-label={`Open ${name}`}
                >
                  <ExternalLink size={13} aria-hidden="true" /> Open
                </a>
              ) : null}
              {doc && canReview && doc.status !== "APPROVED" ? (
                <>
                  <Button
                    size="sm"
                    variant="soft"
                    icon={<Check size={13} aria-hidden="true" />}
                    onClick={() => onApprove(doc.id)}
                  >
                    Approve
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    icon={<X size={13} aria-hidden="true" />}
                    onClick={() => onReject(doc.id)}
                  >
                    Reject
                  </Button>
                </>
              ) : null}
            </div>
          </li>
        );
      })}
    </ul>
  );
}

function FileIcon({ doc }: { doc?: Doc }) {
  const cls = "grid size-9 shrink-0 place-items-center rounded-[8px] bg-surface text-muted";
  if (!doc)
    return (
      <span className={cls}>
        <FileX size={17} aria-hidden="true" />
      </span>
    );
  return (
    <span className={cls}>
      {doc.content_type === "application/pdf" ? (
        <FileText size={17} aria-hidden="true" />
      ) : (
        <FileImage size={17} aria-hidden="true" />
      )}
    </span>
  );
}

function ExpiryChip({ doc }: { doc: Doc }) {
  switch (doc.expiry_state) {
    case "EXPIRED":
      return <Chip tone="bad">Expired</Chip>;
    case "EXPIRING":
      return <Chip tone="warn">{doc.days_left === 0 ? "Expires today" : `${doc.days_left} days left`}</Chip>;
    case "UNKNOWN":
      return <Chip tone="muted">No expiry date</Chip>;
    default:
      return null;
  }
}
