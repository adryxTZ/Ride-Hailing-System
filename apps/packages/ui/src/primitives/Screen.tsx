import type { PropsWithChildren } from "react";
import { KeyboardAvoidingView, Platform, ScrollView, StyleSheet } from "react-native";
import { useTheme } from "../ThemeProvider";

/**
 * Full-height page container with brand background and standard padding.
 * Scrolls and keeps taps alive while the keyboard is open. `align` chooses
 * vertical centring (auth screens) or top alignment (lists, settings).
 */
export function Screen({ children, align = "center" }: PropsWithChildren<{ align?: "center" | "top" }>) {
  const { palette } = useTheme();
  return (
    <KeyboardAvoidingView
      style={[styles.root, { backgroundColor: palette.background }]}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
    >
      <ScrollView
        contentContainerStyle={[styles.inner, align === "center" ? styles.centre : styles.top]}
        keyboardShouldPersistTaps="handled"
        keyboardDismissMode="on-drag"
      >
        {children}
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  inner: { flexGrow: 1, padding: 24, paddingBottom: 110, gap: 16 },
  centre: { justifyContent: "center" },
  top: { justifyContent: "flex-start", paddingTop: 56 },
});
