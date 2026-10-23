/** SettingsScreen (driver) — language and log out. Log out lives only here, never on Home. */
import { Header, ListRow, Screen, Section } from "@platform/ui";
import { useBack } from "@/hooks/useBack";
import { useProfile, useUpdateProfile } from "@/hooks/useProfile";
import { auth } from "@/services/api";

export function SettingsScreen() {
  const back = useBack();
  const { data: me } = useProfile();
  const update = useUpdateProfile();
  const locale = me?.locale ?? "sw";
  return (
    <Screen align="top">
      <Header title="Settings" leading="back" onLeading={() => back()} />
      <Section>
        <ListRow
          icon="translate"
          title="Language"
          subtitle={locale === "sw" ? "Kiswahili" : "English"}
          onPress={() => update.mutate({ locale: locale === "sw" ? "en" : "sw" })}
        />
      </Section>
      <Section>
        <ListRow icon="logout" title="Log out" chevron={false} onPress={() => void auth.logout()} />
      </Section>
    </Screen>
  );
}
