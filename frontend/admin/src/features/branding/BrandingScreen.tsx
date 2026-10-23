"use client";
/**
 * BrandingScreen — Settings › Branding. Left: identity, colours (presets, a
 * Color Hunt link or custom), logos and icons, fonts and corners, and every
 * version. Right: a live preview with the contrast checks, Save draft and
 * Publish (reason required). Publishing changes the apps and this dashboard
 * within seconds; app icon, splash and the name under the icon need an app release.
 */
import { Rocket, Save } from "lucide-react";
import { motion } from "motion/react";
import { Suspense, useEffect, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Field";
import { enter } from "@/components/ui/motion";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReasonDialog } from "@/components/ui/ReasonDialog";
import { ErrorNote, Skeleton } from "@/components/ui/States";
import { Tabs } from "@/components/ui/Tabs";
import type { Schemas } from "@/lib/api";
import { useUrlState } from "@/lib/urlState";
import { AssetsEditor } from "./AssetsEditor";
import { ColoursEditor } from "./ColoursEditor";
import { type Draft, fromVersion } from "./draft";
import { useBranding, useDeleteDraft, usePreview, usePublish, useSaveDraft } from "./hooks";
import { IdentityForm } from "./IdentityForm";
import { PreviewPane } from "./PreviewPane";
import { TypeShapeEditor } from "./TypeShapeEditor";
import { VersionsTable } from "./VersionsTable";

const TABS = [
  { value: "colours", label: "Colours" },
  { value: "assets", label: "Logos and icons" },
  { value: "type", label: "Fonts and corners" },
  { value: "identity", label: "Identity" },
  { value: "versions", label: "Versions" },
] as const;
type Tab = (typeof TABS)[number]["value"];
const DEFAULT_RADIUS = 10;

export function BrandingScreen() {
  return (
    <Suspense>
      <Branding />
    </Suspense>
  );
}

/** The draft to open with: the newest unpublished draft, else a copy of the live version. */
function startingDraft(versions: Schemas["ThemeVersion"][]): Draft | undefined {
  const draft = versions.find((v) => v.status === "DRAFT");
  const live = versions.find((v) => v.status === "PUBLISHED");
  const from = draft ?? live ?? versions[0];
  return from ? fromVersion(from, true) : undefined;
}

function Branding() {
  const { data, isLoading, error } = useBranding();
  const url = useUrlState();
  const tab = (url.get("tab") ?? "colours") as Tab;
  const [draft, setDraft] = useState<Draft>();
  const [ask, setAsk] = useState<{ id: string; rollback: boolean }>();
  const save = useSaveDraft();
  const publish = usePublish();
  const remove = useDeleteDraft();
  const preview = usePreview(draft?.roles ?? { light: {}, dark: {} });

  useEffect(() => {
    if (data && !draft) setDraft(startingDraft(data.versions));
  }, [data, draft]);

  const saved = data?.versions.find((v) => v.id === draft?.id);
  const urls = saved?.asset_urls ?? data?.versions.find((v) => v.status === "PUBLISHED")?.asset_urls ?? {};
  const saveDraft = (then?: (v: Schemas["ThemeVersion"]) => void) =>
    draft &&
    save.mutate(
      { id: draft.id, body: { ...draft, id: undefined } as Schemas["ThemeDraftInput"] },
      {
        onSuccess: (v) => {
          setDraft(fromVersion(v, true));
          then?.(v);
        },
      },
    );
  return (
    <div>
      <PageHeader
        title="Branding"
        subtitle="Name, colours, logos and icons for the apps, receipts and this dashboard."
      />
      <ErrorNote error={error ?? save.error ?? publish.error ?? remove.error} />
      {isLoading || !data || !draft ? (
        <Skeleton className="h-96" />
      ) : (
        <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
          <div className="grid min-w-0 gap-4">
            <Tabs
              id="branding"
              value={tab}
              options={[...TABS]}
              onChange={(v) => url.set({ tab: v === "colours" ? undefined : v })}
            />
            {tab === "identity" ? (
              <Card index={1} title="Identity">
                <IdentityForm identity={data.identity} />
              </Card>
            ) : tab === "versions" ? (
              <Card index={1} title="Versions">
                <VersionsTable
                  versions={data.versions}
                  onOpen={(v) => {
                    setDraft(fromVersion(v, true));
                    url.set({ tab: undefined });
                  }}
                  onRollback={(v) => setAsk({ id: v.id, rollback: true })}
                  onDelete={(v) =>
                    remove.mutate(v.id, { onSuccess: () => draft.id === v.id && setDraft(undefined) })
                  }
                />
              </Card>
            ) : (
              <Card index={1} title={draft.id ? `Draft, version ${saved?.version ?? ""}` : "New draft"}>
                <label className="mb-5 grid gap-1.5 text-sm">
                  <span className="font-medium">Draft name</span>
                  <Input value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} />
                </label>
                {tab === "colours" ? (
                  <ColoursEditor
                    draft={draft}
                    presets={data.presets}
                    derived={preview.data?.palette}
                    onChange={setDraft}
                  />
                ) : tab === "assets" ? (
                  <AssetsEditor draft={draft} slots={data.asset_slots} urls={urls} onChange={setDraft} />
                ) : (
                  <TypeShapeEditor
                    draft={draft}
                    fonts={data.fonts}
                    shapeMax={data.shape_max}
                    onChange={setDraft}
                  />
                )}
              </Card>
            )}
          </div>
          <motion.aside
            {...enter(2)}
            className="min-w-0 rounded-[10px] bg-surface p-5 xl:sticky xl:top-4"
            aria-label="Preview and publish"
          >
            <PreviewPane
              preview={preview.data}
              name={data.identity.name}
              logo={
                draft.assets?.logo_full && urls.logo_full?.endsWith(draft.assets.logo_full)
                  ? urls.logo_full
                  : undefined
              }
              radius={draft.shape?.radius_md ?? DEFAULT_RADIUS}
            />
            <div className="mt-5 flex flex-wrap justify-end gap-2">
              <Button
                variant="soft"
                icon={<Save size={15} aria-hidden="true" />}
                loading={save.isPending}
                onClick={() => saveDraft()}
              >
                Save draft
              </Button>
              <Button
                icon={<Rocket size={15} aria-hidden="true" />}
                disabled={!preview.data?.report.pass}
                onClick={() => saveDraft((v) => setAsk({ id: v.id, rollback: false }))}
              >
                Publish
              </Button>
            </div>
          </motion.aside>
        </div>
      )}
      <ReasonDialog
        open={!!ask}
        title={ask?.rollback ? "Roll back to this version?" : "Publish this theme?"}
        body="The apps and this dashboard change within a few seconds. App icon, splash screen and the name under the icon change with the next app release."
        confirm={ask?.rollback ? "Roll back" : "Publish"}
        busy={publish.isPending}
        onClose={() => setAsk(undefined)}
        onConfirm={(reason) =>
          ask &&
          publish.mutate(
            { id: ask.id, reason },
            {
              onSuccess: (v) => {
                setAsk(undefined);
                setDraft(fromVersion(v, false));
              },
            },
          )
        }
      />
    </div>
  );
}
