/**
 * VehicleScreen — the registered vehicle: ride type, plate, model and review
 * status. Changes go through registration (and a new review), never a local switch.
 */
import { Button, Header, ListRow, Screen, Section, Text } from "@platform/ui";
import { useRouter } from "expo-router";
import { useBack } from "@/hooks/useBack";
import { useMyVehicle } from "@/hooks/useMyVehicle";
import { useOnboarding } from "@/hooks/useOnboarding";

const REVIEW_TEXT: Record<string, string> = {
  APPROVED: "Approved",
  PENDING: "Waiting for review",
  REJECTED: "Needs changes",
};

export function VehicleScreen() {
  const router = useRouter();
  const back = useBack();
  const { data: ob } = useOnboarding();
  const mine = useMyVehicle();
  const v = mine.vehicle;
  return (
    <Screen align="top">
      <Header title="Vehicle" leading="back" onLeading={() => back()} />
      {v ? (
        <Section>
          <ListRow icon="car-side" title="Ride type" subtitle={mine.label} chevron={false} />
          <ListRow icon="card-text-outline" title="Number plate" subtitle={v.plate} chevron={false} />
          {mine.model ? (
            <ListRow icon="car-info" title="Vehicle" subtitle={mine.model} chevron={false} />
          ) : null}
          {v.year ? <ListRow icon="calendar" title="Year" subtitle={String(v.year)} chevron={false} /> : null}
          <ListRow
            icon="shield-check-outline"
            title="Status"
            subtitle={`${REVIEW_TEXT[v.status] ?? v.status}${v.status_reason ? `: ${v.status_reason}` : ""}`}
            chevron={false}
          />
        </Section>
      ) : (
        <Text variant="muted">You have not registered a vehicle yet.</Text>
      )}
      {ob?.can_edit ? (
        <Button
          title={v ? "Change vehicle" : "Add your vehicle"}
          onPress={() => router.push("/onboarding/vehicle")}
        />
      ) : (
        <Text variant="muted">
          To change an approved vehicle, contact support. Trips are only offered for this vehicle.
        </Text>
      )}
    </Screen>
  );
}
