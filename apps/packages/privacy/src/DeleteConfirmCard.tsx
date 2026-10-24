/**
 * DeleteConfirmCard — the inline confirmation before a deletion request is sent:
 * what is removed, what the law keeps, and the two choices. Inline rather than an
 * Alert so it reads the same on every platform.
 */
import { Button, Text, useTheme } from "@platform/ui";
import Animated, { FadeInDown, FadeOut } from "react-native-reanimated";
import { cardStyles, ENTER_MS } from "./cardStyle";
import { DELETE_CONFIRMATION } from "./model";

const EXIT_MS = 180;

interface Props {
  busy: boolean;
  onConfirm: () => void;
  onKeep: () => void;
}

export function DeleteConfirmCard({ busy, onConfirm, onKeep }: Props) {
  const { palette } = useTheme();
  return (
    <Animated.View
      entering={FadeInDown.duration(ENTER_MS)}
      exiting={FadeOut.duration(EXIT_MS)}
      style={[
        cardStyles.card,
        {
          backgroundColor: palette.surface,
          shadowColor: palette.text,
          borderColor: palette.danger,
          borderWidth: 1,
        },
      ]}
    >
      <Text style={{ fontWeight: "700" }}>Delete your account?</Text>
      <Text>{DELETE_CONFIRMATION}</Text>
      <Button title="Delete my account" variant="danger" loading={busy} onPress={onConfirm} />
      <Button title="Keep my account" variant="secondary" disabled={busy} onPress={onKeep} />
    </Animated.View>
  );
}
