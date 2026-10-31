package koala

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode"
)

func writeDocument(w io.Writer, root node, indent string) error {
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	if err := writeNode(w, root, 0, indent); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func writeNode(w io.Writer, n node, depth int, indent string) error {
	prefix := strings.Repeat(indent, depth)
	if depth > 0 {
		if _, err := io.WriteString(w, prefix); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "<"+n.name+">"); err != nil {
		return err
	}
	if n.hasText {
		if _, err := io.WriteString(w, escapeXMLText(n.text)); err != nil {
			return err
		}
	} else if len(n.children) > 0 {
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
		for _, child := range n.children {
			if err := writeNode(w, child, depth+1, indent); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, prefix); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "</"+n.name+">")
	return err
}

func escapeXMLText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '\r':
			b.WriteString("&#xD;")
		default:
			if !isXMLChar(r) {
				b.WriteRune('\uFFFD')
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isXMLChar(r rune) bool {
	return r == 0x9 || r == 0xA || r == 0xD ||
		(r >= 0x20 && r <= 0xD7FF) ||
		(r >= 0xE000 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0x10FFFF)
}

func validateXMLName(name string) error {
	if name == "" {
		return fmt.Errorf("invalid XML element name %q: name is empty", name)
	}
	for i, r := range name {
		if i == 0 {
			if unicode.IsLetter(r) || r == '_' {
				continue
			}
			return fmt.Errorf("invalid XML element name %q", name)
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) ||
			r == '_' || r == '-' || r == '.' {
			continue
		}
		return fmt.Errorf("invalid XML element name %q", name)
	}
	return nil
}
