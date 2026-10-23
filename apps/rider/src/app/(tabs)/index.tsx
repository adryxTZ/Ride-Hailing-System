/** Route "/" → TaxiHomeScreen: the app opens on the ride map. Route files only map URLs to screens. */
import { TaxiHomeScreen } from "@/screens/taxi/TaxiHomeScreen";

export default function Index() {
  return <TaxiHomeScreen />;
}
