/** Route "/book/requested?tripId=…" → TripScreen (the live trip, any phase). */
import { useLocalSearchParams } from "expo-router";
import { TripScreen } from "@/screens/booking/TripScreen";

export default function Requested() {
  const { tripId } = useLocalSearchParams<{ tripId?: string }>();
  return <TripScreen tripId={tripId} />;
}
