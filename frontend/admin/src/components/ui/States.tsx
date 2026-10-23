/** Loading, empty and error states, so no page is ever a blank hole. */
import { AlertTriangle, Inbox } from "lucide-react";

export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`animate-pulse rounded-[10px] bg-surface-2 ${className}`} aria-hidden="true" />;
}

export function Empty({ title, body, icon }: { title: string; body?: string; icon?: React.ReactNode }) {
  return (
    <div className="grid place-items-center gap-2 py-12 text-center">
      <span
        className="grid size-12 place-items-center rounded-full bg-surface-2 text-muted"
        aria-hidden="true"
      >
        {icon ?? <Inbox size={20} />}
      </span>
      <p className="font-semibold">{title}</p>
      {body ? <p className="max-w-sm text-sm text-muted">{body}</p> : null}
    </div>
  );
}

export function ErrorNote({ error }: { error: unknown }) {
  if (!error) return null;
  return (
    <p
      role="alert"
      className="flex items-center gap-2 rounded-[10px] bg-[color-mix(in_srgb,var(--critical)_10%,transparent)] px-4 py-3 text-sm text-critical"
    >
      <AlertTriangle size={16} aria-hidden="true" />
      {(error as Error).message ?? "Something went wrong"}
    </p>
  );
}
