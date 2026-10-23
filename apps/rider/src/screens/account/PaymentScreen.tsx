/**
 * PaymentScreen — balance card and the rider's payment method with the
 * Personal/Work toggle (design: Payment). Cash and mobile money are live; the
 * choice is remembered for every booking. Cards wait for a card gateway.
 */
import { Button, Header, ListRow, RadioRow, Screen, Section, Text, useTheme } from "@platform/ui";
import { LinearGradient } from "expo-linear-gradient";
import { useRouter } from "expo-router";
import { useState } from "react";
import { Pressable, StyleSheet, View } from "react-native";
import { useAppConfig } from "@/hooks/useAppConfig";
import { useBack } from "@/hooks/useBack";
import { usePaymentMethod } from "@/hooks/usePaymentMethod";

export function PaymentScreen() {
  const router = useRouter();
  const back = useBack();
  const { palette } = useTheme();
  const { data: config } = useAppConfig();
  const [profile, setProfile] = useState<"personal" | "work">("personal");
  const { method, choose } = usePaymentMethod();
  const currency = config?.brand.currency ?? "TZS";
  return (
    <Screen align="top">
      <Header leading="close" onLeading={() => back()} title="Payment" large />
      <LinearGradient
        colors={[palette.primary, palette.accent]}
        start={{ x: 0, y: 0 }}
        end={{ x: 1, y: 1 }}
        style={styles.balance}
      >
        <View style={{ flex: 1, gap: 4 }}>
          <Text style={{ color: palette.on_primary ?? "#FFFFFF", opacity: 0.85 }}>
            {config?.brand.name} balance
          </Text>
          <Text style={[styles.amount, { color: palette.on_primary ?? "#FFFFFF" }]}>{currency} 0</Text>
          <Text style={{ color: palette.on_primary ?? "#FFFFFF", opacity: 0.85, fontSize: 12 }}>
            100% secure payments
          </Text>
        </View>
        <Button title="Add money" variant="secondary" onPress={() => undefined} />
      </LinearGradient>
      <Section title="Payment methods">
        <View style={[styles.toggle, { borderColor: palette.primary }]}>
          {(["personal", "work"] as const).map((p) => (
            <Pressable
              key={p}
              onPress={() => setProfile(p)}
              style={[styles.toggleItem, profile === p && { backgroundColor: palette.primary }]}
            >
              <Text
                style={{
                  color: profile === p ? (palette.on_primary ?? "#FFFFFF") : palette.text,
                  fontWeight: "600",
                }}
              >
                {p === "personal" ? "Personal" : "Work"}
              </Text>
            </Pressable>
          ))}
        </View>
        <RadioRow label="Cash" selected={method === "CASH"} onPress={() => choose("CASH")} />
        <RadioRow
          label="Mobile money"
          subtitle="You approve the fare on your phone when the ride ends"
          selected={method === "MOBILE_MONEY"}
          onPress={() => choose("MOBILE_MONEY")}
        />
        <ListRow icon="plus" title="Add debit/credit card" onPress={() => undefined} chevron={false} />
      </Section>
      <Section>
        <ListRow
          icon="pencil-outline"
          title="Manage work profile"
          onPress={() => router.push("/account/work-profile")}
          chevron={false}
        />
      </Section>
    </Screen>
  );
}

const styles = StyleSheet.create({
  balance: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    padding: 16,
    borderRadius: 14,
    borderWidth: 1,
  },
  amount: { fontSize: 24, fontWeight: "700" },
  toggle: { flexDirection: "row", borderWidth: 1, borderRadius: 999, overflow: "hidden", marginVertical: 12 },
  toggleItem: { flex: 1, alignItems: "center", paddingVertical: 12 },
});
