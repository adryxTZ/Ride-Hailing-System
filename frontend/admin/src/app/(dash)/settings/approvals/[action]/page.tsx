import { WorkflowEditorScreen } from "@/features/approvals/WorkflowEditorScreen";

export default async function ApprovalWorkflowPage({ params }: { params: Promise<{ action: string }> }) {
  const { action } = await params;
  return <WorkflowEditorScreen action={decodeURIComponent(action)} />;
}
