// Package koala encodes row-oriented data as XML and returns the body
// compressed with gzip.
//
// The input may be a slice of maps, a slice of structs, or a single map or
// struct. Struct fields are named with xml or json tags, falling back to
// snake_case. Map keys are emitted in sorted order so the output is
// deterministic.
//
// Nested maps, slices, and structs are rendered as a Python-like text value
// inside a single element by default. Use WithNestedElements to render them as
// nested XML elements instead. Floating-point values are written with one
// decimal place.
//
// Compress returns the gzip-compressed body of an XML document. When serving
// the result over HTTP, set Content-Type to application/xml and
// Content-Encoding to gzip.
package koala

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
)

const (
	// DefaultRoot is the default name of the document element.
	DefaultRoot = "rows"
	// DefaultRow is the default name of a row element.
	DefaultRow = "row"
	// DefaultItem is the default name of an item inside a nested slice when
	// WithNestedElements is used.
	DefaultItem = "item"
	// DefaultIndent is the default indentation used for child elements.
	DefaultIndent = "    "
)

type config struct {
	root             string
	row              string
	item             string
	indent           string
	compressionLevel int
	nestedElements   bool
}

// Option configures Compress and CompressTo.
type Option func(*config)

// WithRoot sets the name of the document element. The default is "rows".
func WithRoot(name string) Option {
	return func(c *config) { c.root = name }
}

// WithRow sets the name of each row element. The default is "row".
func WithRow(name string) Option {
	return func(c *config) { c.row = name }
}

// WithItem sets the name of elements inside a nested slice when nested
// elements are enabled. The default is "item".
func WithItem(name string) Option {
	return func(c *config) { c.item = name }
}

// WithIndent sets the indentation used for child elements. The default is
// four spaces. An empty string disables indentation but keeps newlines.
func WithIndent(indent string) Option {
	return func(c *config) { c.indent = indent }
}

// WithCompressionLevel sets the gzip compression level. Valid values are
// gzip.HuffmanOnly through gzip.BestCompression. The default is
// gzip.DefaultCompression.
func WithCompressionLevel(level int) Option {
	return func(c *config) { c.compressionLevel = level }
}

// WithNestedElements renders nested maps, structs, and slices as nested XML
// elements instead of a single text value. By default nested values are
// rendered in a Python-like text representation.
func WithNestedElements() Option {
	return func(c *config) { c.nestedElements = true }
}

func newConfig(opts ...Option) (config, error) {
	cfg := config{
		root:             DefaultRoot,
		row:              DefaultRow,
		item:             DefaultItem,
		indent:           DefaultIndent,
		compressionLevel: gzip.DefaultCompression,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	for _, item := range []struct {
		label string
		name  string
	}{
		{"root name", cfg.root},
		{"row name", cfg.row},
		{"item name", cfg.item},
	} {
		if err := validateXMLName(item.name); err != nil {
			return cfg, fmt.Errorf("koala: %s: %w", item.label, err)
		}
	}
	if cfg.compressionLevel < gzip.HuffmanOnly || cfg.compressionLevel > gzip.BestCompression {
		return cfg, fmt.Errorf("koala: compression level %d out of range", cfg.compressionLevel)
	}
	return cfg, nil
}

// Compress returns the gzip-compressed XML body for v.
//
// The input may be a slice of maps or structs, or a single map or struct.
// A nil top-level value produces an empty document element.
func Compress(v any, opts ...Option) ([]byte, error) {
	var buf bytes.Buffer
	if err := CompressTo(&buf, v, opts...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// CompressTo writes the gzip-compressed XML body for v to w.
//
// The XML document is built before anything is written, so encoding errors do
// not leave a partial document. Write and close errors are still possible if
// w fails.
func CompressTo(w io.Writer, v any, opts ...Option) error {
	if w == nil {
		return errors.New("koala: nil writer")
	}
	cfg, err := newConfig(opts...)
	if err != nil {
		return err
	}
	doc, err := buildDocument(v, cfg)
	if err != nil {
		return err
	}
	zw, err := gzip.NewWriterLevel(w, cfg.compressionLevel)
	if err != nil {
		return fmt.Errorf("koala: create gzip writer: %w", err)
	}
	writeErr := writeDocument(zw, doc, cfg.indent)
	closeErr := zw.Close()
	if writeErr != nil {
		return fmt.Errorf("koala: write xml: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("koala: close gzip writer: %w", closeErr)
	}
	return nil
}
