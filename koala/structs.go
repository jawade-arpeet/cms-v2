package koala

import (
	"encoding"
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

var textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()

func asTextMarshaler(v reflect.Value) (encoding.TextMarshaler, bool) {
	if !v.IsValid() {
		return nil, false
	}
	if (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) && v.IsNil() {
		return nil, false
	}
	if v.CanInterface() {
		if tm, ok := v.Interface().(encoding.TextMarshaler); ok {
			return tm, true
		}
	}
	if v.CanAddr() && v.Addr().CanInterface() {
		if tm, ok := v.Addr().Interface().(encoding.TextMarshaler); ok {
			return tm, true
		}
	}
	return nil, false
}

type structFieldValue struct {
	name  string
	value reflect.Value
}

func (b *builder) structChildren(v reflect.Value) ([]node, error) {
	fields, err := structFieldValues(v, make(map[uintptr]bool))
	if err != nil {
		return nil, err
	}
	children := make([]node, 0, len(fields))
	for _, field := range fields {
		if err := validateXMLName(field.name); err != nil {
			return nil, fmt.Errorf("field %q: %w", field.name, err)
		}
		child, err := b.buildField(field.name, field.value)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", field.name, err)
		}
		children = append(children, child)
	}
	return children, nil
}

func structFieldValues(v reflect.Value, seen map[uintptr]bool) ([]structFieldValue, error) {
	t := v.Type()
	fields := make([]structFieldValue, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" && !sf.Anonymous {
			continue
		}
		fv := v.Field(i)

		if sf.Anonymous && isFlattenable(sf) {
			if fv.Kind() == reflect.Ptr {
				if fv.IsNil() {
					continue
				}
				ptr := fv.Pointer()
				if seen[ptr] {
					return nil, errCycle
				}
				seen[ptr] = true
				sub, err := structFieldValues(fv.Elem(), seen)
				delete(seen, ptr)
				if err != nil {
					return nil, err
				}
				fields = append(fields, sub...)
				continue
			}
			sub, err := structFieldValues(fv, seen)
			if err != nil {
				return nil, err
			}
			fields = append(fields, sub...)
			continue
		}
		if sf.PkgPath != "" {
			// Unexported embedded fields can only be traversed when they are
			// structs or pointers to structs. Normal unexported fields are
			// skipped.
			ft := sf.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() != reflect.Struct {
				continue
			}
		}

		info, err := parseFieldInfo(sf)
		if err != nil {
			return nil, err
		}
		if info.skip {
			continue
		}
		if info.omitEmpty && isEmptyValue(fv) {
			continue
		}
		fields = append(fields, structFieldValue{name: info.name, value: fv})
	}
	return fields, nil
}

func isFlattenable(sf reflect.StructField) bool {
	if !sf.Anonymous {
		return false
	}
	if hasNamedFieldTag(sf) {
		return false
	}
	ft := sf.Type
	if ft.Kind() == reflect.Ptr {
		ft = ft.Elem()
	}
	if ft.Kind() != reflect.Struct {
		return false
	}
	if ft.Implements(textMarshalerType) || reflect.PointerTo(ft).Implements(textMarshalerType) {
		return false
	}
	return true
}

func hasNamedFieldTag(sf reflect.StructField) bool {
	if tag, ok := sf.Tag.Lookup("xml"); ok {
		return strings.Split(tag, ",")[0] != ""
	}
	if tag, ok := sf.Tag.Lookup("json"); ok {
		return strings.Split(tag, ",")[0] != ""
	}
	return false
}

type fieldInfo struct {
	name      string
	omitEmpty bool
	skip      bool
}

func parseFieldInfo(sf reflect.StructField) (fieldInfo, error) {
	if tag, ok := sf.Tag.Lookup("xml"); ok {
		return parseXMLTag(tag, sf.Name)
	}
	if tag, ok := sf.Tag.Lookup("json"); ok {
		return parseJSONTag(tag, sf.Name)
	}
	return fieldInfo{name: snakeCase(sf.Name)}, nil
}

func parseXMLTag(tag, fieldName string) (fieldInfo, error) {
	if tag == "-" {
		return fieldInfo{skip: true}, nil
	}
	parts := strings.Split(tag, ",")
	if parts[0] == "-" {
		return fieldInfo{skip: true}, nil
	}

	info := fieldInfo{}
	name := parts[0]
	if name == "" {
		name = snakeCase(fieldName)
	}
	if strings.Contains(name, ">") {
		return info, fmt.Errorf("nested xml tag paths are not supported: %q", tag)
	}
	info.name = name
	for _, opt := range parts[1:] {
		switch opt {
		case "":
		case "omitempty":
			info.omitEmpty = true
		default:
			return info, fmt.Errorf("unsupported xml tag option %q", opt)
		}
	}
	return info, nil
}

func parseJSONTag(tag, fieldName string) (fieldInfo, error) {
	if tag == "-" {
		return fieldInfo{skip: true}, nil
	}
	parts := strings.Split(tag, ",")
	if parts[0] == "-" {
		return fieldInfo{skip: true}, nil
	}

	info := fieldInfo{}
	name := parts[0]
	if name == "" {
		name = snakeCase(fieldName)
	}
	info.name = name
	for _, opt := range parts[1:] {
		if opt == "omitempty" {
			info.omitEmpty = true
		}
	}
	return info, nil
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	default:
		return false
	}
}

func snakeCase(s string) string {
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(s) + 4)
	for i, r := range runes {
		if !unicode.IsUpper(r) {
			b.WriteRune(r)
			continue
		}
		if i > 0 {
			prev := runes[i-1]
			var next rune
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && unicode.IsLower(next)) {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
