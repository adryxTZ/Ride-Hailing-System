/** SignupPanel — the home sheet until the driver is approved: where registration stands and the way back into it. */
import { Button, Text } from "@platform/ui";
import { StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { StatusCard } from "@/components/onboarding/StatusCard";
import { type DriverStatus, statusCopy } from "@/models/onboarding";

interface Props {
  firstName?: string;
  status: DriverStatus;
  reason?: string;
  done: number;
  total: number;
  onOpen: () => void;
}

export function SignupPanel({ firstName, status, reason, done, total, onOpen }: Props) {
  const copy = statusCopy(status);
  const action =
    status === "PENDING_REVIEW" || status === "SUSPENDED" ? "View registration" : "Continue registration";
  return (
    <View style={styles.wrap}>
      <Animated.View entering={FadeInDown.duration(280)}>
        <Text variant="muted">{firstName ? `Hi, ${firstName}` : "Welcome"}</Text>
      </Animated.View>
      <StatusCard
        icon={copy.icon}
        title={copy.title}
        body={copy.body}
        reason={status === "REJECTED" ? reason : undefined}
        done={done}
        total={total}
        showProgress={status === "ONBOARDING" || status === "REJECTED"}
      />
      <Animated.View entering={FadeInDown.delay(60).duration(280)}>
        <Button title={action} onPress={onOpen} />
      </Animated.View>
    </View>
  );
}

const styles = StyleSheet.create({ wrap: { gap: 14 } });
