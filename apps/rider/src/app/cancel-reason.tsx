import { useLocalSearchParams } from "expo-router";
import { CancelReasonScreen } from "@/screens/activity/CancelReasonScreen";

/** Presented as a modal from an active trip. */
export default function CancelReason() {
  const { tripId } = useLocalSearchParams<{ tripId?: string }>();
  return <CancelReasonScreen tripId={tripId} />;
}
