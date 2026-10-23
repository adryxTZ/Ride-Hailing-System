package pdf

import (
	"bytes"
	"fmt"
	"strings"
)

// header is the PDF version line plus a comment of bytes >= 128 so that
// transfer tools treat the file as binary.
const header = "%PDF-1.4\n%\xE2\xE3\xCF\xD3\n"

// Fixed object numbers; page objects follow from firstPageObject, two per
// page (the page dictionary, then its content stream).
const (
	catalogObject    = 1
	pagesObject      = 2
	regularObject    = 3
	boldObject       = 4
	firstPageObject  = 5
	objectsPerPage   = 2
	freeEntry        = "0000000000 65535 f \n"
	xrefEntryPattern = "%010d 00000 n \n"
)

// writer serialises objects into buf and records each object's byte offset
// for the cross-reference table (offsets[i] belongs to object i+1).
type writer struct {
	buf     *bytes.Buffer
	offsets []int
}

// newWriter wraps buf in a writer.
func newWriter(buf *bytes.Buffer) *writer {
	return &writer{buf: buf}
}

// writeDocument emits the full file: header, objects, xref and trailer.
func (w *writer) writeDocument(d *Document) {
	w.buf.WriteString(header)
	w.object(catalogObject, fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pagesObject))
	w.object(pagesObject, pagesDict(len(d.pages)))
	w.object(regularObject, fontDict(Regular))
	w.object(boldObject, fontDict(Bold))
	for i, p := range d.pages {
		w.writePage(pageObject(i), p)
	}
	infoRef := 0
	if !d.info.isEmpty() {
		infoRef = pageObject(len(d.pages))
		w.object(infoRef, infoDict(d.info))
	}
	w.writeTrailer(infoRef)
}

// writePage emits a page dictionary followed by its content stream.
func (w *writer) writePage(obj int, p *Page) {
	dict := fmt.Sprintf("<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %s %s]"+
		" /Resources << /Font << /F1 %d 0 R /F2 %d 0 R >> >> /Contents %d 0 R >>",
		pagesObject, num(p.size.Width), num(p.size.Height),
		regularObject, boldObject, obj+1)
	w.object(obj, dict)
	w.stream(obj+1, p.content.Bytes())
}

// object writes "N 0 obj <body> endobj" and records its offset.
func (w *writer) object(n int, body string) {
	w.begin(n)
	w.buf.WriteString(body)
	w.buf.WriteString("\nendobj\n")
}

// stream writes an uncompressed stream object with its exact /Length.
func (w *writer) stream(n int, data []byte) {
	w.begin(n)
	fmt.Fprintf(w.buf, "<< /Length %d >>\nstream\n", len(data))
	w.buf.Write(data)
	w.buf.WriteString("\nendstream\nendobj\n")
}

// begin records object n's offset and writes its opening line.
// Objects are always written in ascending order, so n == len(offsets)+1.
func (w *writer) begin(n int) {
	w.offsets = append(w.offsets, w.buf.Len())
	fmt.Fprintf(w.buf, "%d 0 obj\n", n)
}

// writeTrailer emits the xref table, trailer, startxref and %%EOF.
// infoRef is 0 when the document has no Info dictionary.
func (w *writer) writeTrailer(infoRef int) {
	xrefOffset := w.buf.Len()
	size := len(w.offsets) + 1
	fmt.Fprintf(w.buf, "xref\n0 %d\n%s", size, freeEntry)
	for _, off := range w.offsets {
		fmt.Fprintf(w.buf, xrefEntryPattern, off)
	}
	fmt.Fprintf(w.buf, "trailer\n<< /Size %d /Root %d 0 R", size, catalogObject)
	if infoRef != 0 {
		fmt.Fprintf(w.buf, " /Info %d 0 R", infoRef)
	}
	fmt.Fprintf(w.buf, " >>\nstartxref\n%d\n%%%%EOF\n", xrefOffset)
}

// pageObject returns the object number of the i-th (0-based) page.
func pageObject(i int) int {
	return firstPageObject + i*objectsPerPage
}

// pagesDict builds the page tree root listing every page in order.
func pagesDict(count int) string {
	kids := make([]string, count)
	for i := range kids {
		kids[i] = fmt.Sprintf("%d 0 R", pageObject(i))
	}
	return fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>",
		strings.Join(kids, " "), count)
}

// fontDict builds a base-14 Type1 font dictionary with WinAnsiEncoding.
func fontDict(f Font) string {
	return fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /%s"+
		" /Encoding /WinAnsiEncoding >>", f.baseFontName())
}

// infoDict builds the document information dictionary in a fixed key order.
func infoDict(info Info) string {
	var b strings.Builder
	b.WriteString("<<")
	fields := []struct{ key, value string }{
		{"Title", info.Title}, {"Author", info.Author},
		{"Subject", info.Subject}, {"Creator", info.Creator},
	}
	for _, f := range fields {
		if f.value != "" {
			fmt.Fprintf(&b, " /%s %s", f.key, textString(f.value))
		}
	}
	if !info.CreatedAt.IsZero() {
		fmt.Fprintf(&b, " /CreationDate (%s)", pdfDate(info.CreatedAt))
	}
	b.WriteString(" >>")
	return b.String()
}
