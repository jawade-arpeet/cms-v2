package koala_test

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"cms/koala"
)

type embeddedBase struct {
	ID string `json:"id"`
}

type embeddedRow struct {
	embeddedBase
	HSCode string
	LCVATs []string `xml:"lc_vats"`
	Skip   string   `xml:"-"`
	Empty  string   `xml:"empty,omitempty"`
}

func TestEmbeddedStructAndFieldNames(t *testing.T) {
	got := compressXML(t, []embeddedRow{{
		embeddedBase: embeddedBase{ID: "1"},
		HSCode:       "450023234",
		LCVATs:       []string{"23 %"},
		Skip:         "hidden",
	}})
	for _, want := range []string{
		"<id>1</id>",
		"<hs_code>450023234</hs_code>",
		"<lc_vats>['23 %']</lc_vats>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in XML:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"hidden", "<empty>"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("unexpected %q in XML:\n%s", unwanted, got)
		}
	}
	assertWellFormed(t, got)
}

type upperText string

func (u upperText) MarshalText() ([]byte, error) {
	return []byte(strings.ToUpper(string(u))), nil
}

func TestTextMarshalerAndTime(t *testing.T) {
	rows := []map[string]any{{
		"when": time.Date(2026, 1, 21, 0, 0, 0, 0, time.UTC),
		"code": upperText("abc"),
	}}
	got := compressXML(t, rows)
	for _, want := range []string{
		"<when>2026-01-21T00:00:00Z</when>",
		"<code>ABC</code>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in XML:\n%s", want, got)
		}
	}
}

func TestByteSlices(t *testing.T) {
	rows := []map[string]any{{
		"data":   []byte("hello"),
		"nested": []any{[]byte("hello")},
	}}
	got := compressXML(t, rows)
	for _, want := range []string{
		"<data>aGVsbG8=</data>",
		"<nested>['aGVsbG8=']</nested>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in XML:\n%s", want, got)
		}
	}
}

func TestSingleStruct(t *testing.T) {
	got := compressXML(t, embeddedRow{
		embeddedBase: embeddedBase{ID: "1"},
		HSCode:       "450023234",
	})
	if !strings.HasPrefix(got, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<rows>\n    <row>\n") {
		t.Fatalf("single struct was not wrapped in rows/row:\n%s", got)
	}
	if !strings.Contains(got, "<id>1</id>") || !strings.Contains(got, "<hs_code>450023234</hs_code>") {
		t.Fatalf("unexpected XML:\n%s", got)
	}
	assertWellFormed(t, got)
}

func TestCompressTo(t *testing.T) {
	var buf bytes.Buffer
	err := koala.CompressTo(&buf, []map[string]any{{"value": 15.0001}})
	if err != nil {
		t.Fatalf("CompressTo: %v", err)
	}
	got := decompress(t, buf.Bytes())
	if !strings.Contains(got, "<value>15.0</value>") {
		t.Fatalf("unexpected XML:\n%s", got)
	}
}

func TestCompressToNilWriter(t *testing.T) {
	err := koala.CompressTo(nil, []map[string]any{{"a": 1}})
	if err == nil || !strings.Contains(err.Error(), "nil writer") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func ExampleCompress() {
	rows := []map[string]any{{
		"hs_code":              "100120012",
		"applicable_countries": []string{"AT", "CN"},
	}}

	body, err := koala.Compress(rows)
	if err != nil {
		panic(err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		panic(err)
	}
	defer zr.Close()
	xmlBody, err := io.ReadAll(zr)
	if err != nil {
		panic(err)
	}

	fmt.Print(string(xmlBody))
	// Output:
	// <?xml version="1.0" encoding="UTF-8"?>
	// <rows>
	//     <row>
	//         <applicable_countries>['AT', 'CN']</applicable_countries>
	//         <hs_code>100120012</hs_code>
	//     </row>
	// </rows>
}
