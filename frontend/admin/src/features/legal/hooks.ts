"use client";
/**
 * Legal hooks — the Legal page (every app's documents, the Legal details and
 * the placeholders), one document's versions, saving and discarding a draft,
 * publishing it with a reason, the details form, and the live preview.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { api, type Schemas, unwrap } from "@/lib/api";

export type LegalDoc = Schemas["LegalDocument"];
export type LegalItem = Schemas["LegalOverviewItem"];
export type LegalApp = LegalDoc["app"];
export type LegalKind = LegalDoc["kind"];
export type Placeholder = Schemas["LegalPlaceholder"];
export type Texts = Record<string, string>;

const KEY = ["admin", "legal"];
/** Wait this long after the last keystroke before asking the server for a preview. */
const PREVIEW_DEBOUNCE_MS = 400;

/** The Legal page's data. */
export function useLegal() {
  return useQuery({ queryKey: KEY, queryFn: async () => unwrap(await api.GET("/admin/legal")) });
}

const docPath = (app: LegalApp, kind: LegalKind) => ({ params: { path: { app, kind } } });

/** One document's versions, newest first, with their texts. */
export function useVersions(app: LegalApp, kind: LegalKind) {
  return useQuery({
    queryKey: [...KEY, "versions", app, kind],
    queryFn: async () => unwrap(await api.GET("/admin/legal/{app}/{kind}", docPath(app, kind))),
  });
}

function useRefresh() {
  const qc = useQueryClient();
  return () => void qc.invalidateQueries({ queryKey: KEY });
}

export interface DraftTexts {
  title: Texts;
  summary: Texts;
  body: Texts;
}

/** Creates the next version as a draft, or replaces the open draft. */
export function useSaveDraft(app: LegalApp, kind: LegalKind) {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: async (body: DraftTexts) =>
      unwrap(await api.PUT("/admin/legal/{app}/{kind}/draft", { ...docPath(app, kind), body })),
    onSuccess: refresh,
  });
}

/** Discards the open draft. */
export function useDeleteDraft(app: LegalApp, kind: LegalKind) {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: async () => {
      const r = await api.DELETE("/admin/legal/{app}/{kind}/draft", docPath(app, kind));
      if (r.response.status !== 204) unwrap(r);
    },
    onSuccess: refresh,
  });
}

/** Publishes the draft with a reason. */
export function usePublish(app: LegalApp, kind: LegalKind) {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: async (reason: string) =>
      unwrap(
        await api.POST("/admin/legal/{app}/{kind}/publish", { ...docPath(app, kind), body: { reason } }),
      ),
    onSuccess: refresh,
  });
}

/** Saves the Legal details (placeholder values) with a reason. */
export function useSaveDetails() {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: async (body: { fields: Texts; reason: string }) =>
      unwrap(await api.PUT("/admin/legal/details", { body })),
    onSuccess: refresh,
  });
}

/** The text being edited, rendered by the server with today's placeholder values. */
export function usePreview(app: LegalApp, kind: LegalKind, lang: string, title: string, body: string) {
  const [debounced, setDebounced] = useState({ title, body });
  useEffect(() => {
    const t = setTimeout(() => setDebounced({ title, body }), PREVIEW_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [title, body]);
  return useQuery({
    queryKey: [...KEY, "preview", app, kind, lang, debounced],
    queryFn: async () =>
      unwrap(
        await api.POST("/admin/legal/preview", {
          body: { app, kind, lang, title: { [lang]: debounced.title }, body: { [lang]: debounced.body } },
        }),
      ),
    placeholderData: (prev) => prev,
  });
}
