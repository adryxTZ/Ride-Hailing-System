/** AccountScreen (driver) — who you are, your vehicle and documents, and the way into Settings. */
import { Avatar, Header, ListRow, Screen, Section, Text } from "@platform/ui";
import { useRouter } from "expo-router";
import { StyleSheet, View } from "react-native";
import { useMyVehicle } from "@/hooks/useMyVehicle";
import { useProfile } from "@/hooks/useProfile";

export function AccountScreen() {
  const router = useRouter();
  const { data: me } = useProfile();
  const vehicle = useMyVehicle();
  return (
    <Screen align="top">
      <Header title="Account" large />
      <View style={styles.person}>
        <Avatar name={me?.full_name ?? undefined} size={64} />
        <View style={styles.body}>
          <Text style={styles.name}>{me?.full_name ?? "Driver"}</Text>
          <Text variant="muted">{me?.phone ?? ""}</Text>
        </View>
      </View>
      <Section>
        <ListRow
          icon="car-side"
          title="Vehicle"
          subtitle={vehicle.plate ? `${vehicle.label} · ${vehicle.plate}` : vehicle.label}
          onPress={() => router.push("/account/vehicle")}
        />
        <ListRow
          icon="file-document-outline"
          title="Documents"
          subtitle="Expiry dates and renewals"
          onPress={() => router.push("/account/documents")}
        />
        <ListRow
          icon="clipboard-text-outline"
          title="Registration"
          subtitle="Your sign-up checklist"
          onPress={() => router.push("/onboarding")}
        />
      </Section>
      <Section>
        <ListRow icon="cog-outline" title="Settings" onPress={() => router.push("/account/settings")} />
      </Section>
    </Screen>
  );
}

const styles = StyleSheet.create({
  person: { flexDirection: "row", alignItems: "center", gap: 14 },
  body: { flex: 1, gap: 2 },
  name: { fontSize: 20, fontWeight: "700" },
});
