/** ChatScreen — the rider's conversation with the driver of a trip. */
import { ChatView } from "@platform/chat";
import { Header, useTheme } from "@platform/ui";
import { KeyboardAvoidingView, Platform, StyleSheet, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useAppConfig } from "@/hooks/useAppConfig";
import { useBack } from "@/hooks/useBack";
import { useChat } from "@/hooks/useChat";
import { useTrip } from "@/hooks/useTrips";
import { TERMINAL } from "@/services/trips";

export function ChatScreen({ tripId }: { tripId: string }) {
  const back = useBack();
  const { palette } = useTheme();
  const insets = useSafeAreaInsets();
  const { data: config } = useAppConfig();
  const { data: trip } = useTrip(tripId);
  const { page, send } = useChat(tripId, true);
  const title = trip?.driver?.name ?? "Your driver";
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
          me="RIDER"
          quickReplies={page?.quick_replies ?? []}
          locale={config?.brand.locale ?? "en"}
          sending={send.isPending}
          closed={!!trip && TERMINAL.includes(trip.status)}
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
