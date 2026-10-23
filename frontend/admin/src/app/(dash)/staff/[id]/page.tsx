import { StaffDetail } from "@/features/staff/StaffDetail";

export default async function StaffPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <StaffDetail id={id} />;
}
