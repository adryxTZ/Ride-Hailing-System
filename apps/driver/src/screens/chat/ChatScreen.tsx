/** ChatScreen — the driver's conversation with the rider of the current trip. */
import { ChatView } from "@platform/chat";
import { Header, useTheme } from "@platform/ui";
import { KeyboardAvoidingView, Platform, StyleSheet, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useBack } from "@/hooks/useBack";
import { useChat } from "@/hooks/useChat";
import { useDriverTrip } from "@/hooks/useDriverTrips";

export function ChatScreen({ tripId }: { tripId: string }) {
  const back = useBack();
  const { palette } = useTheme();
  const insets = useSafeAreaInsets();
  const { data: trip } = useDriverTrip();
  const { page, send } = useChat(tripId, true);
  const live = trip?.id === tripId;
  const title = trip?.contacts?.pickup?.name ?? trip?.rider?.name ?? "Rider";
  return (
    <KeyboardAvoidingView
      style={[styles.root, { backgroundColor: palette.background, paddingTop: insets.top + 8 }]}
      behavior={Platform.OS === "ios" ? "padding" : undefined}
    >
      <View style={styles.header}>
        <Header title={title} leading="back" onLeading={() => back()} />
      </View>
      <View style={[styles.flex, { paddingBottom: insets.bottom }]}>
        <ChatView
          messages={page?.data ?? []}
          me="DRIVER"
          quickReplies={page?.quick_replies ?? []}
          locale="sw"
          sending={send.isPending}
          closed={!live}
          onSend={(body, quickReply) => send.mutate({ body, quickReply })}
        />
      </View>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  header: { paddingHorizontal: 16 },
  flex: { flex: 1 },
});
