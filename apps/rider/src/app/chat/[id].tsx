/** Route "/chat/[id]" → ChatScreen for a trip. */
import { useLocalSearchParams } from "expo-router";
import { ChatScreen } from "@/screens/chat/ChatScreen";

export default function Chat() {
  const { id } = useLocalSearchParams<{ id: string }>();
  return <ChatScreen tripId={id} />;
}
