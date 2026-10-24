import { RequestScreen } from "@/features/privacy/RequestScreen";

export default async function PrivacyRequestPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <RequestScreen id={id} />;
}
