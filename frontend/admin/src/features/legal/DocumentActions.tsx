"use client";
/**
 * DocumentActions — Save draft, Discard draft and Publish for the document
 * being edited. Publishing needs a saved draft and a reason (audited); for a
 * text people accept the dialog warns that everyone will be asked again.
 */
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { Portal } from "@/components/ui/Portal";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { ErrorNote } from "@/components/ui/States";
import { type LegalApp, type LegalKind, useDeleteDraft, usePublish, useSaveDraft } from "./hooks";
import { APP_PEOPLE } from "./labels";
import type { useEditorState } from "./useEditorState";

type Editor = ReturnType<typeof useEditorState>;

export function DocumentActions({
  app,
  kind,
  consent,
  editor,
}: {
  app: LegalApp;
  kind: LegalKind;
  consent: boolean;
  editor: Editor;
}) {
  const save = useSaveDraft(app, kind);
  const discard = useDeleteDraft(app, kind);
  const publish = usePublish(app, kind);
  const [asking, setAsking] = useState(false);
  const error = save.error ?? discard.error ?? publish.error;
  return (
    <div className="grid justify-items-end gap-2">
      <div className="flex flex-wrap items-center justify-end gap-2">
        {editor.dirty ? <Chip tone="warn">Unsaved changes</Chip> : null}
        {editor.draft && !editor.dirty ? (
          <Chip tone="live">{`Draft v${editor.draft.version} saved`}</Chip>
        ) : null}
        {editor.draft ? (
          <Button size="sm" variant="ghost" loading={discard.isPending} onClick={() => discard.mutate()}>
            Discard draft
          </Button>
        ) : null}
        {editor.dirty && !editor.draft ? (
          <Button size="sm" variant="ghost" onClick={editor.reset}>
            Undo changes
          </Button>
        ) : null}
        <Button
          size="sm"
          variant="soft"
          disabled={!editor.dirty}
          loading={save.isPending}
          onClick={() => save.mutate(editor.texts)}
        >
          Save draft
        </Button>
        <Button
          size="sm"
          variant="brand"
          disabled={!editor.draft || editor.dirty}
          onClick={() => setAsking(true)}
        >
          Publish
        </Button>
      </div>
      {error ? <ErrorNote error={error} /> : null}
      <Portal>
        <ReasonDialog
          open={asking}
          title={`Publish version ${editor.draft?.version ?? ""}?`}
          body={
            consent
              ? `All ${APP_PEOPLE[app]} will be asked to accept the new version the next time they open the app, before they can book or go online. The published text can never be changed.`
              : "The published text can never be changed; a correction is a new version."
          }
          confirm="Publish"
          busy={publish.isPending}
          onClose={() => setAsking(false)}
          onConfirm={(reason) => publish.mutate(reason, { onSuccess: () => setAsking(false) })}
        />
      </Portal>
    </div>
  );
}
