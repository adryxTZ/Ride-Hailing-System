import { StyleSheet, TextInput, type TextInputProps, View } from "react-native";
import { useTheme } from "../ThemeProvider";
import { Text } from "./Text";

interface Props extends TextInputProps {
  label?: string;
  error?: string;
}

/** Labelled input with brand border/surface colours and an inline error line. */
export function TextField({ label, error, style, ...rest }: Props) {
  const { palette } = useTheme();
  return (
    <View style={styles.wrap}>
      {label ? <Text variant="muted">{label}</Text> : null}
      <TextInput
        {...rest}
        placeholderTextColor={palette.muted}
        style={[
          styles.input,
          {
            backgroundColor: palette.surface,
            borderColor: error ? palette.danger : palette.border,
            color: palette.text,
          },
          style,
        ]}
      />
      {error ? <Text variant="error">{error}</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { gap: 6 },
  input: { height: 52, borderRadius: 10, borderWidth: 1, paddingHorizontal: 14, fontSize: 18 },
});
