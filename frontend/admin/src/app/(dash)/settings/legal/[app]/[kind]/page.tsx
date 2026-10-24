import { DocumentScreen } from "@/features/legal/DocumentScreen";
import type { LegalApp } from "@/features/legal/hooks";
import { kindOf } from "@/features/legal/labels";

export default async function LegalDocumentPage({
  params,
}: {
  params: Promise<{ app: string; kind: string }>;
}) {
  const { app, kind } = await params;
  return <DocumentScreen app={app.toUpperCase() as LegalApp} kind={kindOf(kind)} />;
}
