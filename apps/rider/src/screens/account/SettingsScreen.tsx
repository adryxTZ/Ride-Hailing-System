/** SettingsScreen — communication, language, privacy, security, legal, log out (design: Settings). */
import { Header, ListRow, Screen, Section } from "@platform/ui";
import { useBack } from "@/hooks/useBack";
import { useProfile, useUpdateProfile } from "@/hooks/useProfile";
import { auth } from "@/services/api";

export function SettingsScreen() {
  const back = useBack();
  const { data: me } = useProfile();
  const update = useUpdateProfile();
  const locale = me?.locale ?? "sw";
  const toggleLanguage = () => update.mutate({ locale: locale === "sw" ? "en" : "sw" });
  return (
    <Screen align="top">
      <Header title="Settings" leading="back" onLeading={() => back()} />
      <Section>
        <ListRow icon="volume-high" title="Communication" onPress={() => undefined} />
        <ListRow
          icon="calendar-blank-outline"
          title="Calendar"
          subtitle="Get smart trip suggestions"
          onPress={() => undefined}
        />
        <ListRow
          icon="translate"
          title="Language"
          subtitle={locale === "sw" ? "Kiswahili" : "English"}
          onPress={toggleLanguage}
        />
        <ListRow
          icon="account-lock-outline"
          title="Privacy"
          subtitle="Manage your data and permissions"
          onPress={() => undefined}
        />
        <ListRow
          icon="lock-outline"
          title="Sign in and security"
          subtitle="Devices and verification"
          onPress={() => undefined}
        />
        <ListRow icon="bank-outline" title="Legal" onPress={() => undefined} />
      </Section>
      <Section>
        <ListRow icon="logout" title="Log out" chevron={false} onPress={() => void auth.logout()} />
      </Section>
    </Screen>
  );
}
