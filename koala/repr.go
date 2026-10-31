package koala

import (
	"encoding/base64"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type reprRenderer struct {
	visiting map[visitKey]bool
}

func (b *builder) repr(v reflect.Value) (string, error) {
	r := &reprRenderer{visiting: make(map[visitKey]bool)}
	return r.render(v)
}

func (r *reprRenderer) render(v reflect.Value) (string, error) {
	v = unwrapInterface(v)
	if !v.IsValid() {
		return "None", nil
	}
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return "None", nil
	}
	if tm, ok := asTextMarshaler(v); ok {
		text, err := tm.MarshalText()
		if err != nil {
			return "", err
		}
		return quotePython(string(text)), nil
	}

	switch v.Kind() {
	case reflect.Ptr:
		return r.withVisit(v, func() (string, error) {
			return r.render(v.Elem())
		})
	case reflect.Map:
		return r.renderMap(v)
	case reflect.Slice:
		if isByteSequence(v) {
			return quotePython(base64.StdEncoding.EncodeToString(v.Bytes())), nil
		}
		if v.IsNil() {
			return "[]", nil
		}
		return r.renderSlice(v)
	case reflect.Array:
		if isByteSequence(v) {
			return quotePython(base64.StdEncoding.EncodeToString(bytesOf(v))), nil
		}
		return r.renderSequence(v)
	case reflect.Struct:
		return r.renderStruct(v)
	case reflect.String:
		return quotePython(v.String()), nil
	case reflect.Bool:
		if v.Bool() {
			return "True", nil
		}
		return "False", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32:
		return formatReprFloat(v.Float(), 32), nil
	case reflect.Float64:
		return formatReprFloat(v.Float(), 64), nil
	default:
		return "", fmt.Errorf("unsupported type %s", v.Type())
	}
}

func (r *reprRenderer) renderMap(v reflect.Value) (string, error) {
	if v.Type().Key().Kind() != reflect.String {
		return "", fmt.Errorf("map key type %s is not supported; use string keys", v.Type().Key())
	}
	if v.IsNil() {
		return "{}", nil
	}

	return r.withVisit(v, func() (string, error) {
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })

		var b strings.Builder
		b.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(quotePython(key.String()))
			b.WriteString(": ")
			value, err := r.render(v.MapIndex(key))
			if err != nil {
				return "", err
			}
			b.WriteString(value)
		}
		b.WriteByte('}')
		return b.String(), nil
	})
}

func (r *reprRenderer) renderSlice(v reflect.Value) (string, error) {
	return r.withVisit(v, func() (string, error) {
		return r.renderSequence(v)
	})
}

func (r *reprRenderer) renderSequence(v reflect.Value) (string, error) {
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < v.Len(); i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		value, err := r.render(v.Index(i))
		if err != nil {
			return "", err
		}
		b.WriteString(value)
	}
	b.WriteByte(']')
	return b.String(), nil
}

func (r *reprRenderer) renderStruct(v reflect.Value) (string, error) {
	fields, err := structFieldValues(v, make(map[uintptr]bool))
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteByte('{')
	for i, field := range fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(quotePython(field.name))
		b.WriteString(": ")
		value, err := r.render(field.value)
		if err != nil {
			return "", err
		}
		b.WriteString(value)
	}
	b.WriteByte('}')
	return b.String(), nil
}

func (r *reprRenderer) withVisit(v reflect.Value, fn func() (string, error)) (string, error) {
	key, ok := visitKeyFor(v)
	if ok {
		if r.visiting[key] {
			return "", errCycle
		}
		r.visiting[key] = true
		defer delete(r.visiting, key)
	}
	return fn()
}

func quotePython(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04x`, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

func formatReprFloat(f float64, bits int) string {
	switch {
	case math.IsNaN(f):
		return "nan"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	default:
		return strconv.FormatFloat(f, 'f', 1, bits)
	}
}
