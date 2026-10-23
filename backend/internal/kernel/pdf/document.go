package pdf

import (
	"bytes"
	"errors"
	"io"
	"time"
)

// Size is a page size in points (1/72 inch).
type Size struct {
	Width, Height float64
}

// A4 is ISO A4 portrait, 210 x 297 mm.
var A4 = Size{Width: 595.28, Height: 841.89}

// Letter is US Letter portrait, 8.5 x 11 in.
var Letter = Size{Width: 612, Height: 792}

// ErrNoPages is returned when writing a Document that has no pages.
var ErrNoPages = errors.New("pdf: document has no pages")

// Info is the optional document information dictionary. Empty fields and a
// zero CreatedAt are omitted, which keeps output reproducible by default.
type Info struct {
	Title     string
	Author    string
	Subject   string
	Creator   string
	CreatedAt time.Time
}

// isEmpty reports whether no Info field is set.
func (i Info) isEmpty() bool {
	return i.Title == "" && i.Author == "" && i.Subject == "" &&
		i.Creator == "" && i.CreatedAt.IsZero()
}

// Document is a PDF under construction. It is not safe for concurrent use.
type Document struct {
	size  Size
	info  Info
	pages []*Page
}

// New starts an empty document whose pages all have the given size.
func New(size Size) *Document {
	return &Document{size: size}
}

// SetInfo replaces the document information dictionary.
func (d *Document) SetInfo(info Info) {
	d.info = info
}

// AddPage appends a blank page and returns it for drawing.
func (d *Document) AddPage() *Page {
	p := &Page{size: d.size}
	d.pages = append(d.pages, p)
	return p
}

// PageCount returns the number of pages added so far.
func (d *Document) PageCount() int {
	return len(d.pages)
}

// WriteTo serialises the document to w; it implements io.WriterTo.
// The document stays usable, so it may be written more than once.
func (d *Document) WriteTo(w io.Writer) (int64, error) {
	data, err := d.Bytes()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

// Bytes renders the complete PDF file.
func (d *Document) Bytes() ([]byte, error) {
	if len(d.pages) == 0 {
		return nil, ErrNoPages
	}
	var buf bytes.Buffer
	newWriter(&buf).writeDocument(d)
	return buf.Bytes(), nil
}
