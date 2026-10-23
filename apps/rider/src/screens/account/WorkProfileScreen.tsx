/** WorkProfileScreen — company details, work email, payment method, reports (design: Work profile). Corporate accounts arrive in a later phase. */
import { Banner, Header, ListRow, Screen, Section } from "@platform/ui";
import { useRouter } from "expo-router";
import { useAppConfig } from "@/hooks/useAppConfig";
import { useBack } from "@/hooks/useBack";
import { useProfile } from "@/hooks/useProfile";

export function WorkProfileScreen() {
  const router = useRouter();
  const back = useBack();
  const { data: me } = useProfile();
  const { data: config } = useAppConfig();
  return (
    <Screen align="top">
      <Header
        leading="back"
        onLeading={() => back()}
        title="Work profile"
        large
        trailingIcon="trash-can-outline"
        onTrailing={() => undefined}
      />
      <Section>
        <ListRow icon="briefcase-outline" title="Add company details" onPress={() => undefined} />
        <ListRow
          icon="email-outline"
          title="Work email"
          subtitle={me?.email ?? "Add a work email"}
          onPress={() => router.push("/account/profile")}
        />
        <ListRow
          icon="cash"
          title="Payment method"
          subtitle="Cash"
          onPress={() => router.push("/account/payment")}
        />
      </Section>
      <Section>
        <ListRow icon="chart-box-outline" title="Reports" onPress={() => undefined} />
        <ListRow icon="file-document-outline" title="Add expense provider" onPress={() => undefined} />
      </Section>
      <Banner
        icon="briefcase"
        title="Trips for your whole company?"
        highlight={`Learn about ${config?.brand.name ?? ""} for Business`}
      />
    </Screen>
  );
}
