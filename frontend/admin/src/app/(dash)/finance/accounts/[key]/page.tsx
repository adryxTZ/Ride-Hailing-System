import { AccountDetail } from "@/features/finance/accounts/AccountDetail";

export default async function FinanceAccountPage({ params }: { params: Promise<{ key: string }> }) {
  const { key } = await params;
  return <AccountDetail accountKey={decodeURIComponent(key)} />;
}
