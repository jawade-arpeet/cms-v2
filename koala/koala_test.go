package koala_test

import (
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"cms/koala"
)

func decompress(t *testing.T, body []byte) string {
	t.Helper()
	if len(body) < 2 || body[0] != 0x1f || body[1] != 0x8b {
		t.Fatalf("body is not gzip data: % x", body[:min(len(body), 8)])
	}
	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer zr.Close()
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	return string(out)
}

func compressXML(t *testing.T, v any, opts ...koala.Option) string {
	t.Helper()
	body, err := koala.Compress(v, opts...)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	return decompress(t, body)
}

func assertWellFormed(t *testing.T, doc string) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(doc))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("invalid XML: %v\n%s", err, doc)
		}
	}
}

func sampleRows() []map[string]any {
	return []map[string]any{
		{
			"hs_code":              "100120012",
			"applicable_countries": []string{"AT", "CN"},
			"effective_at":         "2026-01-21",
			"excluding_countries": []map[string]any{
				{"country_code": "AU", "description": "Australia"},
				{"country_code": "US", "description": "United States of America"},
			},
			"lc_vats": []map[string]any{},
		},
		{
			"hs_code":              "450023234",
			"applicable_countries": []string{"IN", "LU", "MT"},
			"effective_at":         "2026-01-21",
			"excluding_countries": []map[string]any{
				{"country_code": "AU", "description": "Australia"},
				{"country_code": "US", "description": "United States of America"},
			},
			"lc_vats": []map[string]any{
				{
					"duty_name":              "Values Added Tax",
					"duty":                   "23 %",
					"lc_calculation_formula": "{1}",
					"lc_duty": []map[string]any{
						{"value": 23.00, "type": "AD_VALOREM"},
					},
				},
			},
		},
	}
}

func TestCompressMapRows(t *testing.T) {
	got := compressXML(t, sampleRows())
	want := `<?xml version="1.0" encoding="UTF-8"?>
<rows>
    <row>
        <applicable_countries>['AT', 'CN']</applicable_countries>
        <effective_at>2026-01-21</effective_at>
        <excluding_countries>[{'country_code': 'AU', 'description': 'Australia'}, {'country_code': 'US', 'description': 'United States of America'}]</excluding_countries>
        <hs_code>100120012</hs_code>
        <lc_vats>[]</lc_vats>
    </row>
    <row>
        <applicable_countries>['IN', 'LU', 'MT']</applicable_countries>
        <effective_at>2026-01-21</effective_at>
        <excluding_countries>[{'country_code': 'AU', 'description': 'Australia'}, {'country_code': 'US', 'description': 'United States of America'}]</excluding_countries>
        <hs_code>450023234</hs_code>
        <lc_vats>[{'duty': '23 %', 'duty_name': 'Values Added Tax', 'lc_calculation_formula': '{1}', 'lc_duty': [{'type': 'AD_VALOREM', 'value': 23.0}]}]</lc_vats>
    </row>
</rows>
`
	if got != want {
		t.Fatalf("unexpected XML\n got:\n%s\nwant:\n%s", got, want)
	}
	assertWellFormed(t, got)
}

func TestCompressStructRows(t *testing.T) {
	type country struct {
		Code        string `xml:"country_code"`
		Description string `xml:"description"`
	}
	type dutyValue struct {
		Value float64 `json:"value"`
		Type  string  `json:"type"`
	}
	type vat struct {
		DutyName              string      `json:"duty_name"`
		Duty                  string      `json:"duty"`
		CalculationFormula    string      `json:"lc_calculation_formula"`
		LCApplicableCondition string      `json:"lc_applicable_condition,omitempty"`
		LCDuty                []dutyValue `json:"lc_duty"`
	}
	type row struct {
		HSCode              string    `xml:"hs_code"`
		ApplicableCountries []string  `xml:"applicable_countries"`
		EffectiveAt         string    `xml:"effective_at"`
		ExcludingCountries  []country `xml:"excluding_countries"`
		LCVATs              []vat     `xml:"lc_vats"`
	}

	rows := []row{{
		HSCode:              "450023234",
		ApplicableCountries: []string{"IN", "LU", "MT"},
		EffectiveAt:         "2026-01-21",
		ExcludingCountries:  []country{{Code: "AU", Description: "Australia"}},
		LCVATs: []vat{{
			DutyName:           "Values Added Tax",
			Duty:               "23 %",
			CalculationFormula: "{1}",
			LCDuty:             []dutyValue{{Value: 23.0, Type: "AD_VALOREM"}},
		}},
	}}

	got := compressXML(t, rows)
	for _, want := range []string{
		"<hs_code>450023234</hs_code>",
		"<applicable_countries>['IN', 'LU', 'MT']</applicable_countries>",
		"<excluding_countries>[{'country_code': 'AU', 'description': 'Australia'}]</excluding_countries>",
		"<lc_vats>[{'duty_name': 'Values Added Tax', 'duty': '23 %', 'lc_calculation_formula': '{1}', 'lc_duty': [{'value': 23.0, 'type': 'AD_VALOREM'}]}]</lc_vats>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in XML:\n%s", want, got)
		}
	}
	if strings.Contains(got, "lc_applicable_condition") {
		t.Fatalf("omitempty field was not omitted:\n%s", got)
	}
	assertWellFormed(t, got)
}

func TestCompressNestedElements(t *testing.T) {
	rows := []map[string]any{{
		"applicable_countries": []string{"AT", "CN"},
		"excluding_countries": []map[string]any{
			{"country_code": "AU", "description": "Australia"},
		},
	}}
	got := compressXML(t, rows, koala.WithNestedElements())
	want := `<?xml version="1.0" encoding="UTF-8"?>
<rows>
    <row>
        <applicable_countries>
            <item>AT</item>
            <item>CN</item>
        </applicable_countries>
        <excluding_countries>
            <item>
                <country_code>AU</country_code>
                <description>Australia</description>
            </item>
        </excluding_countries>
    </row>
</rows>
`
	if got != want {
		t.Fatalf("unexpected XML\n got:\n%s\nwant:\n%s", got, want)
	}
	assertWellFormed(t, got)
}

func TestFloatFormatting(t *testing.T) {
	rows := []map[string]any{{
		"top":    15.0001,
		"nested": map[string]any{"value": 15.0001, "rounded": 15.05},
	}}
	got := compressXML(t, rows)
	for _, want := range []string{
		"<top>15.0</top>",
		"<nested>{'rounded': 15.1, 'value': 15.0}</nested>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in XML:\n%s", want, got)
		}
	}
}

func TestEscaping(t *testing.T) {
	rows := []map[string]any{{
		"text":   `a & b < c > d "quoted" 'single'`,
		"nested": []string{`x & y`},
	}}
	got := compressXML(t, rows)
	if !strings.Contains(got, `<text>a &amp; b &lt; c &gt; d "quoted" 'single'</text>`) {
		t.Fatalf("text not escaped as expected:\n%s", got)
	}
	if !strings.Contains(got, `<nested>['x &amp; y']</nested>`) {
		t.Fatalf("nested text not escaped as expected:\n%s", got)
	}
	assertWellFormed(t, got)
}

func TestEmptyInput(t *testing.T) {
	for name, input := range map[string]any{
		"nil":         nil,
		"empty slice": []map[string]any{},
		"nil slice":   []map[string]any(nil),
	} {
		t.Run(name, func(t *testing.T) {
			got := compressXML(t, input)
			want := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<rows></rows>\n"
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestOptions(t *testing.T) {
	rows := []map[string]any{{"tags": []string{"a", "b"}}}
	got := compressXML(t, rows,
		koala.WithRoot("data"),
		koala.WithRow("entry"),
		koala.WithItem("tag"),
		koala.WithIndent("  "),
		koala.WithNestedElements(),
		koala.WithCompressionLevel(gzip.BestSpeed),
	)
	for _, want := range []string{
		"<data>", "<entry>", "<tags>", "<tag>a</tag>", "</data>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in XML:\n%s", want, got)
		}
	}
	assertWellFormed(t, got)
}

func TestDeterministic(t *testing.T) {
	input := sampleRows()
	first, err := koala.Compress(input)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		next, err := koala.Compress(input)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, next) {
			t.Fatal("Compress output is not deterministic")
		}
	}
}

func TestErrors(t *testing.T) {
	cycle := map[string]any{}
	cycle["self"] = cycle

	tests := []struct {
		name  string
		input any
		opts  []koala.Option
		want  string
	}{
		{"top-level scalar", "nope", nil, "top-level value must be a slice, map, or struct"},
		{"non-string map key", map[int]any{1: "x"}, nil, "map key type int is not supported"},
		{"invalid map key", map[string]any{"bad key": 1}, nil, "invalid XML element name"},
		{"unsupported field type", map[string]any{"ch": make(chan int)}, nil, "unsupported type chan int"},
		{"cycle", cycle, nil, "cyclic value detected"},
		{"cycle nested", cycle, []koala.Option{koala.WithNestedElements()}, "cyclic value detected"},
		{"invalid root", []map[string]any{{"a": 1}}, []koala.Option{koala.WithRoot("bad root")}, "invalid XML element name"},
		{"invalid compression", []map[string]any{{"a": 1}}, []koala.Option{koala.WithCompressionLevel(99)}, "compression level"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := koala.Compress(tt.input, tt.opts...)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}

type attrRow struct {
	Value string `xml:"value,attr"`
}

func TestUnsupportedXMLTag(t *testing.T) {
	_, err := koala.Compress([]attrRow{{Value: "x"}})
	if err == nil || !strings.Contains(err.Error(), "unsupported xml tag option") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func BenchmarkCompress(b *testing.B) {
	rows := sampleRows()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := koala.Compress(rows); err != nil {
			b.Fatal(err)
		}
	}
}
