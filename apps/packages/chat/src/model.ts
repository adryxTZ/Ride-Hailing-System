/** Chat and rating shapes (structurally the API's Message / QuickReply / RatingInput). */

export type ChatRole = "RIDER" | "DRIVER";

export interface ChatMessage {
  id: string;
  sender_role: ChatRole;
  body: string;
  created_at: string;
  read_at?: string;
}

export interface QuickReply {
  key: string;
  text: Record<string, string>;
}

export interface RatingValue {
  score: number;
  aspects: string[];
  comment?: string;
}

/** One selectable chip on the rating form. */
export interface RatingAspect {
  key: string;
  label: string;
}

/** Adds a message to a thread once (frames and refetches may both deliver it). */
export function appendMessage<T extends { id: string }>(list: T[] | undefined, m: T): T[] {
  const base = list ?? [];
  return base.some((x) => x.id === m.id) ? base : [...base, m];
}

/** "14:05" for a message time. */
export function messageTime(iso: string): string {
  const d = new Date(iso);
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}
