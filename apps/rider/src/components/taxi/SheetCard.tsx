/** SheetCard — a white block inside the grey trip sheet (status, route, payment, more). */
import { useTheme } from "@platform/ui";
import type { PropsWithChildren } from "react";
import { type StyleProp, StyleSheet, View, type ViewStyle } from "react-native";

export function SheetCard({ children, style }: PropsWithChildren<{ style?: StyleProp<ViewStyle> }>) {
  const { palette } = useTheme();
  return <View style={[styles.card, { backgroundColor: palette.background }, style]}>{children}</View>;
}

const styles = StyleSheet.create({ card: { borderRadius: 18, padding: 20, gap: 12 } });
