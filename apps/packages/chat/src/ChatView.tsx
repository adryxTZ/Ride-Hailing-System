/**
 * ChatView — the in-trip conversation: bubbles (mine on the right, in the
 * brand colour), canned-reply chips, and the input with a send button. When
 * `closed` the input is replaced by a note (the trip has ended).
 */
import { Icon, ScalePressable, Text, useTheme, withAlpha } from "@platform/ui";
import { useEffect, useRef, useState } from "react";
import { FlatList, ScrollView, StyleSheet, TextInput, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { type ChatMessage, type ChatRole, messageTime, type QuickReply } from "./model";

const MAX_LEN = 500;

interface Props {
  messages: ChatMessage[];
  me: ChatRole;
  quickReplies: QuickReply[];
  locale: string;
  sending?: boolean;
  closed?: boolean;
  onSend: (body: string, quickReply?: string) => void;
}

export function ChatView(p: Props) {
  const { palette } = useTheme();
  const [draft, setDraft] = useState("");
  const list = useRef<FlatList<ChatMessage>>(null);
  useEffect(() => {
    if (p.messages.length) setTimeout(() => list.current?.scrollToEnd({ animated: true }), 50);
  }, [p.messages.length]);
  const send = () => {
    const body = draft.trim();
    if (!body || p.sending) return;
    p.onSend(body);
    setDraft("");
  };
  return (
    <View style={styles.root}>
      <FlatList
        ref={list}
        data={p.messages}
        keyExtractor={(m) => m.id}
        style={styles.list}
        contentContainerStyle={styles.thread}
        ListEmptyComponent={
          <Text variant="muted" style={styles.empty}>
            Say hello or use a quick reply.
          </Text>
        }
        renderItem={({ item }) => <Bubble m={item} mine={item.sender_role === p.me} />}
      />
      {p.closed ? (
        <Text variant="muted" style={styles.closed}>
          This trip has ended. Chat is closed.
        </Text>
      ) : (
        <>
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            style={styles.chipRow}
            contentContainerStyle={styles.chips}
          >
            {p.quickReplies.map((q) => {
              const text = q.text[p.locale] ?? q.text.en ?? q.key;
              return (
                <ScalePressable
                  key={q.key}
                  onPress={() => p.onSend(text, q.key)}
                  style={[styles.chip, { borderColor: palette.border, backgroundColor: palette.surface }]}
                >
                  <Text style={styles.chipText}>{text}</Text>
                </ScalePressable>
              );
            })}
          </ScrollView>
          <View style={[styles.inputRow, { borderTopColor: palette.border }]}>
            <TextInput
              value={draft}
              onChangeText={setDraft}
              placeholder="Message"
              placeholderTextColor={palette.muted}
              maxLength={MAX_LEN}
              multiline
              style={[styles.input, { backgroundColor: palette.surface, color: palette.text }]}
            />
            <ScalePressable
              accessibilityLabel="Send"
              onPress={send}
              disabled={!draft.trim() || p.sending}
              style={[styles.send, { backgroundColor: draft.trim() ? palette.primary : palette.border }]}
            >
              <Icon name="send" size={22} color={palette.on_primary ?? palette.background} />
            </ScalePressable>
          </View>
        </>
      )}
    </View>
  );
}

function Bubble({ m, mine }: { m: ChatMessage; mine: boolean }) {
  const { palette } = useTheme();
  const ink = mine ? (palette.on_primary ?? palette.background) : palette.text;
  return (
    <Animated.View
      entering={FadeInDown.duration(240)}
      style={[
        styles.bubble,
        mine ? styles.mine : styles.theirs,
        { backgroundColor: mine ? palette.primary : withAlpha(palette.border ?? palette.surface, 0.6) },
      ]}
    >
      <Text style={[styles.body, { color: ink }]}>{m.body}</Text>
      <Text style={[styles.time, { color: ink }]}>
        {messageTime(m.created_at)}
        {mine && m.read_at ? " · Read" : ""}
      </Text>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  list: { flex: 1 },
  thread: { padding: 16, gap: 8, flexGrow: 1 },
  empty: { textAlign: "center", marginTop: 40 },
  bubble: { maxWidth: "80%", borderRadius: 18, paddingHorizontal: 14, paddingVertical: 10, gap: 2 },
  mine: { alignSelf: "flex-end", borderBottomRightRadius: 6 },
  theirs: { alignSelf: "flex-start", borderBottomLeftRadius: 6 },
  body: { fontSize: 16 },
  time: { fontSize: 11, opacity: 0.75, alignSelf: "flex-end" },
  chipRow: { flexGrow: 0 },
  chips: { paddingHorizontal: 16, paddingVertical: 8, gap: 8 },
  chip: { borderWidth: 1, borderRadius: 18, paddingHorizontal: 14, height: 36, justifyContent: "center" },
  chipText: { fontSize: 14 },
  inputRow: {
    flexDirection: "row",
    alignItems: "flex-end",
    gap: 10,
    paddingHorizontal: 12,
    paddingTop: 10,
    paddingBottom: 12,
    borderTopWidth: StyleSheet.hairlineWidth,
  },
  input: {
    flex: 1,
    minHeight: 48,
    maxHeight: 120,
    borderRadius: 24,
    paddingHorizontal: 16,
    paddingVertical: 12,
    fontSize: 16,
  },
  send: { width: 48, height: 48, borderRadius: 24, alignItems: "center", justifyContent: "center" },
  closed: { textAlign: "center", padding: 16 },
});
