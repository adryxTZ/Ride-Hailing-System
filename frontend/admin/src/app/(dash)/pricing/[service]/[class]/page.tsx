import { VehicleTypeScreen } from "@/features/pricing/VehicleTypeScreen";

export default async function VehicleTypePage({
  params,
}: {
  params: Promise<{ service: string; class: string }>;
}) {
  const { service, class: cls } = await params;
  return <VehicleTypeScreen service={service} cls={cls} />;
}
