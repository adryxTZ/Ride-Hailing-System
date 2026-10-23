import { ProfileScreen } from "@/features/pricing/profiles/ProfileScreen";

export default async function ProfilePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <ProfileScreen id={id} />;
}
