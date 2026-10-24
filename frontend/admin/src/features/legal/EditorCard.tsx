"use client";
/**
 * EditorCard — one document's texts in one language: title, the consent
 * screen's short summary (texts people accept), and the Markdown body.
 * Placeholders are inserted at the cursor from the list below the editor.
 */
import { type RefObject, useImperativeHandle, useRef } from "react";
import { Field, Input } from "@/components/ui/Field";
import type { DraftTexts } from "./hooks";

/** Lets the placeholder list insert {{key}} where the cursor is. */
export interface BodyInserter {
  insert: (text: string) => void;
}

const AREA =
  "w-full resize-y rounded-[8px] bg-surface-2 px-3 py-2 text-sm outline-none placeholder:text-muted focus:ring-2 focus:ring-brand";
const BODY_ROWS = 26;

export function EditorCard({
  lang,
  texts,
  consent,
  onChange,
  inserter,
}: {
  lang: string;
  texts: DraftTexts;
  consent: boolean;
  onChange: (part: keyof DraftTexts, lang: string, value: string) => void;
  inserter: RefObject<BodyInserter | null>;
}) {
  const body = useRef<HTMLTextAreaElement>(null);
  useImperativeHandle(inserter, () => ({
    insert: (text: string) => {
      const el = body.current;
      const value = texts.body[lang] ?? "";
      const at = el ? el.selectionStart : value.length;
      onChange("body", lang, value.slice(0, at) + text + value.slice(el ? el.selectionEnd : at));
      requestAnimationFrame(() => el?.focus());
    },
  }));
  return (
    <div className="grid gap-3">
      <Field label="Title">
        <Input value={texts.title[lang] ?? ""} onChange={(e) => onChange("title", lang, e.target.value)} />
      </Field>
      {consent ? (
        <Field
          label="Consent screen summary"
          hint="The short text the app shows above “I agree”. Empty: keep the current one."
        >
          <textarea
            rows={3}
            className={AREA}
            value={texts.summary[lang] ?? ""}
            onChange={(e) => onChange("summary", lang, e.target.value)}
          />
        </Field>
      ) : null}
      <Field
        label="Text"
        hint="Markdown: ## heading, **bold**, *italic*, - list, 1. list, > note, [link](https://…). HTML is shown as plain text."
      >
        <textarea
          ref={body}
          rows={BODY_ROWS}
          spellCheck
          className={`${AREA} font-mono text-[13px] leading-relaxed`}
          value={texts.body[lang] ?? ""}
          onChange={(e) => onChange("body", lang, e.target.value)}
        />
      </Field>
    </div>
  );
}
