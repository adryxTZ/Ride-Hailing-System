/**
 * DetailsScreen — between the route and the options. For a ride: "Who is
 * riding?" when the pickup is not where the rider stands. For a package: the
 * receiver (always), the sender when the rider is not at the pickup, and what
 * is being sent.
 */
import { Header, Screen, TextField } from "@platform/ui";
import { useRouter } from "expo-router";
import { StyleSheet, View } from "react-native";
import { ContactCard } from "@/components/booking/ContactCard";
import { PillButton } from "@/components/common/PillButton";
import { useBack } from "@/hooks/useBack";
import { useBookingDetails } from "@/hooks/useBookingDetails";

export function DetailsScreen() {
  const router = useRouter();
  const back = useBack();
  const d = useBookingDetails();
  const next = () => {
    d.commit();
    router.push("/book/options");
  };
  return (
    <View style={styles.root}>
      <Screen align="top">
        <Header
          title={d.isPackage ? "Package details" : "Who is riding?"}
          leading="back"
          onLeading={() => back()}
        />
        {d.askWho ? (
          <ContactCard
            index={0}
            icon={d.isPackage ? "package-variant" : "account-arrow-right"}
            title={d.isPackage ? "Who hands over the package?" : "Is this ride for someone else?"}
            subtitle="The pickup is not where you are. The driver calls this person when they arrive."
            whoOptions={{ me: "Me", other: "Someone else" }}
            forOther={d.forOther}
            onForOther={d.setForOther}
            value={d.atPickup}
            onChange={d.setAtPickup}
          />
        ) : null}
        {d.isPackage ? (
          <>
            <ContactCard
              index={1}
              icon="account-check"
              title="Who receives it?"
              subtitle="The driver calls the receiver at the drop-off."
              value={d.receiver}
              onChange={d.setReceiver}
            />
            <TextField
              label="What are you sending? (optional)"
              placeholder="e.g. documents, a phone, keys"
              value={d.note}
              onChangeText={d.setNote}
            />
          </>
        ) : null}
      </Screen>
      <View style={styles.footer}>
        <PillButton title="Continue" onPress={next} disabled={!d.complete} />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  footer: { position: "absolute", left: 24, right: 24, bottom: 28 },
});
