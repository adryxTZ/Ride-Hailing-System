import { GeofenceScreen } from "@/features/geofences/GeofenceScreen";

export default async function GeofencePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <GeofenceScreen id={id} />;
}
