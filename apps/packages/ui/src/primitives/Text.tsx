import { Text as RNText, StyleSheet, type TextProps } from "react-native";
import { useTheme } from "../ThemeProvider";

type Variant = "title" | "body" | "muted" | "error";

/** Themed text. Variant picks size and colour role. */
export function Text({ variant = "body", style, ...rest }: TextProps & { variant?: Variant }) {
  const { palette } = useTheme();
  const colour = {
    title: palette.text,
    body: palette.text,
    muted: palette.muted ?? palette.text,
    error: palette.danger ?? palette.text,
  }[variant];
  return <RNText {...rest} style={[styles[variant], { color: colour }, style]} />;
}

const styles = StyleSheet.create({
  title: { fontSize: 28, fontWeight: "700" },
  body: { fontSize: 16 },
  muted: { fontSize: 14 },
  error: { fontSize: 14, fontWeight: "600" },
});
