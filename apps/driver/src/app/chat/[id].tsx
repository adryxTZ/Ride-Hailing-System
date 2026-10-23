/** Route "/chat/[id]" → ChatScreen for the current trip. */
import { useLocalSearchParams } from "expo-router";
import { ChatScreen } from "@/screens/chat/ChatScreen";

export default function Chat() {
  const { id } = useLocalSearchParams<{ id: string }>();
  return <ChatScreen tripId={id} />;
}
