import { DriverDetail } from "@/features/drivers/DriverDetail";

export default async function DriverPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <DriverDetail id={id} />;
}
