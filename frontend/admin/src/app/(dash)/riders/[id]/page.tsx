import { RiderDetail } from "@/features/people/RiderDetail";

export default async function RiderPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <RiderDetail id={id} />;
}
