// Package pdf is a small, dependency-free PDF 1.4 writer for simple,
// text-and-rule documents such as trip receipts.
//
// It draws with the two built-in base-14 fonts Helvetica and Helvetica-Bold
// (nothing is embedded) using WinAnsiEncoding, so Latin text in English and
// Swahili renders exactly; characters outside Windows-1252 become '?'.
// Output is deterministic: the same sequence of calls always yields
// byte-identical files, because nothing time- or map-dependent is written
// unless the caller supplies it (see Info.CreatedAt).
//
// Coordinates are in PDF points (1/72 inch) with the origin at the TOP-left
// corner of the page and y growing downwards; the package converts to PDF's
// bottom-left space internally.
//
// Usage:
//
//	doc := pdf.New(pdf.A4)
//	doc.SetInfo(pdf.Info{Title: "Receipt"})
//	p := doc.AddPage()
//	p.Text(40, 60, 18, pdf.Bold, "Receipt 2026-000123")
//	p.TextRight(555, 60, 12, pdf.Regular, "TZS 12,500")
//	data, err := doc.Bytes()
//
// File layout:
//
//	doc.go       package documentation
//	document.go  Document, Info, page sizes, New, SetInfo, AddPage, WriteTo, Bytes
//	page.go      Page drawing operations (text, lines, rectangles, colour)
//	color.go     Color type and constructors
//	fonts.go     Font enum, PDF font names and TextWidth
//	metrics.go   Helvetica / Helvetica-Bold AFM width tables (WinAnsi 32..255)
//	encoding.go  UTF-8 to WinAnsi (cp1252) conversion and string escaping
//	wrap.go      greedy word wrapping by measured width
//	format.go    deterministic number, date and text-string formatting
//	writer.go    object serialisation, xref table and trailer
package pdf
