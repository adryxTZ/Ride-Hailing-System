/** AccountScreen — the Account tab (design: name, rating, setup banner, menu sections). */
import { Avatar, Banner, Icon, ListRow, Screen, Section, Text, useTheme } from "@platform/ui";
import { useRouter } from "expo-router";
import { StyleSheet, View } from "react-native";
import { useProfile } from "@/hooks/useProfile";

export function AccountScreen() {
  const router = useRouter();
  const { palette } = useTheme();
  const { data: me } = useProfile();
  const missing = [!me?.full_name, !me?.email].filter(Boolean).length;
  return (
    <Screen align="top">
      <Text variant="title">Account</Text>
      <View style={styles.identity}>
        <View style={{ flex: 1, gap: 4 }}>
          <Text style={styles.name}>{me?.full_name ?? me?.phone ?? ""}</Text>
          <View style={styles.rating}>
            <Icon name="star" size={18} color={palette.accent} />
            <Text>5.00</Text>
          </View>
        </View>
        <Avatar name={me?.full_name ?? undefined} onPress={() => router.push("/account/profile")} />
      </View>
      {missing > 0 ? (
        <Banner
          icon="shield-check"
          title="Finish setting up your profile"
          highlight={`${missing} new suggestion${missing > 1 ? "s" : ""}`}
          onPress={() => router.push("/account/profile")}
        />
      ) : null}
      <Section>
        <ListRow
          icon="account-circle-outline"
          title="Profile"
          onPress={() => router.push("/account/profile")}
        />
        <ListRow icon="credit-card-outline" title="Payment" onPress={() => router.push("/account/payment")} />
        <ListRow icon="help-circle-outline" title="Support" onPress={() => undefined} />
        <ListRow icon="shield-outline" title="Safety" onPress={() => router.push("/account/safety")} />
        <ListRow
          icon="map-marker-outline"
          title="Saved places"
          onPress={() => router.push("/account/places")}
        />
        <ListRow icon="cog-outline" title="Settings" onPress={() => router.push("/account/settings")} />
      </Section>
      <Section>
        <ListRow
          icon="tag-outline"
          title="Promotions"
          subtitle="Promo codes, offers, and savings"
          onPress={() => undefined}
        />
        <ListRow
          icon="home-heart"
          title="Family profile"
          subtitle="Manage and pay for your family's trips"
          onPress={() => undefined}
        />
        <ListRow
          icon="briefcase-outline"
          title="Work profile"
          subtitle={me?.email ? me.email : "Add company name"}
          onPress={() => router.push("/account/work-profile")}
        />
      </Section>
    </Screen>
  );
}

const styles = StyleSheet.create({
  identity: { flexDirection: "row", alignItems: "center", gap: 12 },
  name: { fontSize: 22, fontWeight: "700" },
  rating: { flexDirection: "row", alignItems: "center", gap: 6 },
});
