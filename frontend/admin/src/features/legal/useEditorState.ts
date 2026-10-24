"use client";
/**
 * useEditorState — the texts being edited for one document: they start from
 * the open draft, else from the live version (the next draft builds on it),
 * and can be reloaded from any earlier version.
 */
import { useEffect, useMemo, useState } from "react";
import type { DraftTexts, LegalDoc, Texts } from "./hooks";

const EMPTY: DraftTexts = { title: {}, summary: {}, body: {} };

function textsOf(d?: LegalDoc): DraftTexts {
  if (!d) return EMPTY;
  return { title: { ...d.title }, summary: { ...d.summary }, body: { ...d.body } };
}

function same(a: Texts, b: Texts): boolean {
  const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
  return [...keys].every((k) => (a[k] ?? "").trim() === (b[k] ?? "").trim());
}

export function useEditorState(versions: LegalDoc[] | undefined) {
  const draft = versions?.find((v) => v.status === "DRAFT");
  const live = versions?.find((v) => v.status === "PUBLISHED");
  const base = useMemo(() => textsOf(draft ?? live), [draft, live]);
  const [texts, setTexts] = useState<DraftTexts>(base);
  // A save or publish reloads the versions: the editor follows the new base.
  useEffect(() => setTexts(base), [base]);
  const dirty =
    !same(texts.title, base.title) || !same(texts.summary, base.summary) || !same(texts.body, base.body);
  const set = (part: keyof DraftTexts, lang: string, value: string) =>
    setTexts((t) => ({ ...t, [part]: { ...t[part], [lang]: value } }));
  return {
    texts,
    set,
    dirty,
    draft,
    live,
    reset: () => setTexts(base),
    load: (v: LegalDoc) => setTexts(textsOf(v)),
  };
}
