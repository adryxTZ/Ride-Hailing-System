/**
 * Prose — shows a legal text the server rendered. The HTML comes from the
 * API's Markdown renderer, which escapes every character it does not turn into
 * its own headings, lists, emphasis and safe links, so no script can reach here.
 */
const PROSE =
  "text-sm leading-relaxed [&_a]:text-brand-text [&_a]:underline [&_blockquote]:mb-3 [&_blockquote]:rounded-[8px] [&_blockquote]:bg-surface-2 [&_blockquote]:px-3 [&_blockquote]:py-2 [&_h1]:mb-2 [&_h1]:text-xl [&_h1]:font-semibold [&_h2]:mt-5 [&_h2]:mb-2 [&_h2]:text-base [&_h2]:font-semibold [&_h3]:mt-4 [&_h3]:mb-1 [&_h3]:font-semibold [&_hr]:my-4 [&_hr]:border-line [&_li]:mb-1 [&_ol]:mb-3 [&_ol]:list-decimal [&_ol]:pl-5 [&_p]:mb-3 [&_ul]:mb-3 [&_ul]:list-disc [&_ul]:pl-5";

export function Prose({ html }: { html: string }) {
  // Server-sanitised legal HTML (see above).
  return <div className={PROSE} dangerouslySetInnerHTML={{ __html: html }} />;
}
