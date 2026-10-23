/**
 * useChat — one trip's conversation: the cached thread (kept live by chat.message
 * frames in useTripRealtime), sending, and how many of the driver's lines are
 * unread (cleared when the chat is opened, which marks them read on the server).
 */
import { appendMessage } from "@platform/chat";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type ChatPage, fetchChat, sendMessage } from "@/services/messaging";

export const chatKey = (tripId: string) => ["chat", tripId] as const;

export function useChat(tripId: string | undefined, open: boolean) {
  const qc = useQueryClient();
  const query = useQuery({
    queryKey: chatKey(tripId ?? ""),
    queryFn: ({ signal }) => fetchChat(tripId as string, signal),
    enabled: !!tripId && open,
  });
  const send = useMutation({
    mutationFn: (v: { body: string; quickReply?: string }) =>
      sendMessage(tripId as string, v.body, v.quickReply),
    onSuccess: (m) =>
      qc.setQueryData<ChatPage>(chatKey(tripId as string), (page) =>
        page ? { ...page, data: appendMessage(page.data, m) } : page,
      ),
  });
  return { page: query.data, loading: query.isLoading, error: query.error, send };
}

/** Unread lines from the other side in the cached thread (frames arrive unread). */
export function useUnreadCount(tripId: string | undefined, me: "RIDER" | "DRIVER"): number {
  // Reads the cache only (frames fill it); fetching happens when the chat opens.
  const { data } = useQuery<ChatPage>({
    queryKey: chatKey(tripId ?? ""),
    queryFn: ({ signal }) => fetchChat(tripId as string, signal),
    enabled: false,
  });
  return (data?.data ?? []).filter((m) => m.sender_role !== me && !m.read_at).length;
}
