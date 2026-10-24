import { RequestScreen } from "@/features/approvals/RequestScreen";

export default async function ApprovalRequestPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <RequestScreen id={id} />;
}
