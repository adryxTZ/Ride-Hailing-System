import { FlagScreen } from "@/features/fraud/FlagScreen";

export default async function FlagPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <FlagScreen id={id} />;
}
