/** Route "/trip/[id]" → TripDetailScreen (a past trip and its receipt). */
import { useLocalSearchParams } from "expo-router";
import { TripDetailScreen } from "@/screens/activity/TripDetailScreen";

export default function TripDetail() {
  const { id } = useLocalSearchParams<{ id?: string }>();
  return <TripDetailScreen id={id} />;
}
