"use client";
/** Branding hooks — the page's data, the live preview, Color Hunt import, uploads, drafts and publishing. */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { api, type Schemas, unwrap } from "@/lib/api";
import { apiUrl } from "@/lib/config";

const KEY = ["admin", "branding"];
/** Wait this long after the last colour change before asking the server for a preview. */
const PREVIEW_DEBOUNCE_MS = 250;

export function useBranding() {
  return useQuery({ queryKey: KEY, queryFn: async () => unwrap(await api.GET("/admin/branding")) });
}

/** Refreshes the page and the app config (the dashboard's own colours) after a write. */
function useRefresh() {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: KEY });
    void qc.invalidateQueries({ queryKey: ["app-config"] });
  };
}

/** The derived palettes and contrast report for the colours being edited. */
export function usePreview(roles: Schemas["ThemeRoles"]) {
  const [debounced, setDebounced] = useState(roles);
  useEffect(() => {
    const t = setTimeout(() => setDebounced(roles), PREVIEW_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [roles]);
  return useQuery({
    queryKey: ["admin", "branding-preview", debounced],
    queryFn: async () => unwrap(await api.POST("/admin/branding/preview", { body: debounced })),
    placeholderData: (prev) => prev,
  });
}

export function useImportColorHunt() {
  return useMutation({
    mutationFn: async (url: string) => unwrap(await api.POST("/admin/branding/import", { body: { url } })),
  });
}

/** Uploads one image and returns its key (to attach to the draft). */
export function useUploadAsset() {
  return useMutation({
    mutationFn: async (file: File) => {
      const type = file.type as "image/png" | "image/webp" | "image/jpeg";
      const ticket = unwrap(
        await api.POST("/admin/branding/assets/upload-url", { body: { content_type: type } }),
      );
      const res = await fetch(apiUrl(ticket.upload_url), {
        method: "PUT",
        body: file,
        headers: { "Content-Type": type },
      });
      if (!res.ok) throw new Error("The upload did not go through. Try again.");
      return ticket.key;
    },
  });
}

export function useSaveDraft() {
  const done = useRefresh();
  return useMutation({
    mutationFn: async (a: { id?: string; body: Schemas["ThemeDraftInput"] }) =>
      a.id
        ? unwrap(
            await api.PUT("/admin/branding/themes/{id}", { params: { path: { id: a.id } }, body: a.body }),
          )
        : unwrap(await api.POST("/admin/branding/themes", { body: a.body })),
    onSuccess: done,
  });
}

export function usePublish() {
  const done = useRefresh();
  return useMutation({
    mutationFn: async (a: { id: string; reason: string }) =>
      unwrap(
        await api.POST("/admin/branding/themes/{id}/publish", {
          params: { path: { id: a.id } },
          body: { reason: a.reason },
        }),
      ),
    onSuccess: done,
  });
}

export function useDeleteDraft() {
  const done = useRefresh();
  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(await api.DELETE("/admin/branding/themes/{id}", { params: { path: { id } } })),
    onSuccess: done,
  });
}

export function useSaveIdentity() {
  const done = useRefresh();
  return useMutation({
    mutationFn: async (body: Schemas["BrandIdentityInput"]) =>
      unwrap(await api.PUT("/admin/branding/identity", { body })),
    onSuccess: done,
  });
}
