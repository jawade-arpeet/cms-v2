package koala

import (
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
)

var errCycle = errors.New("cyclic value detected")

type node struct {
	name     string
	text     string
	hasText  bool
	children []node
}

type builder struct {
	cfg      config
	visiting map[visitKey]bool
}

type visitKey struct {
	typ reflect.Type
	ptr uintptr
	len int
	cap int
}

func buildDocument(v any, cfg config) (node, error) {
	root := node{name: cfg.root}
	if v == nil {
		return root, nil
	}

	rv, err := deref(reflect.ValueOf(v))
	if err != nil {
		return root, err
	}
	if !rv.IsValid() {
		return root, nil
	}

	b := &builder{cfg: cfg, visiting: make(map[visitKey]bool)}
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		if isByteSequence(rv) {
			return root, errors.New("koala: top-level byte sequence is not row data")
		}
		root.children = make([]node, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			row, err := b.buildRow(cfg.row, rv.Index(i))
			if err != nil {
				return root, fmt.Errorf("koala: row %d: %w", i, err)
			}
			root.children = append(root.children, row)
		}
		return root, nil
	case reflect.Map, reflect.Struct:
		row, err := b.buildRow(cfg.row, rv)
		if err != nil {
			return root, err
		}
		root.children = []node{row}
		return root, nil
	default:
		return root, fmt.Errorf("koala: top-level value must be a slice, map, or struct, got %s", rv.Type())
	}
}

func (b *builder) buildRow(name string, v reflect.Value) (node, error) {
	n := node{name: name}
	v, err := deref(v)
	if err != nil {
		return n, err
	}
	if !v.IsValid() {
		return n, nil
	}
	if tm, ok := asTextMarshaler(v); ok {
		text, err := tm.MarshalText()
		if err != nil {
			return n, fmt.Errorf("marshal text: %w", err)
		}
		n.hasText = true
		n.text = string(text)
		return n, nil
	}

	switch v.Kind() {
	case reflect.Map:
		children, err := b.mapChildren(v)
		if err != nil {
			return n, err
		}
		n.children = children
	case reflect.Struct:
		children, err := b.structChildren(v)
		if err != nil {
			return n, err
		}
		n.children = children
	case reflect.Slice, reflect.Array:
		if isByteSequence(v) {
			n.hasText = true
			n.text = base64.StdEncoding.EncodeToString(bytesOf(v))
			return n, nil
		}
		text, err := b.repr(v)
		if err != nil {
			return n, err
		}
		n.hasText = true
		n.text = text
	default:
		text, err := formatScalar(v)
		if err != nil {
			return n, err
		}
		n.hasText = true
		n.text = text
	}
	return n, nil
}

func (b *builder) buildField(name string, v reflect.Value) (node, error) {
	n := node{name: name}
	v = unwrapInterface(v)
	if !v.IsValid() {
		return n, nil
	}
	if tm, ok := asTextMarshaler(v); ok {
		text, err := tm.MarshalText()
		if err != nil {
			return n, fmt.Errorf("marshal text: %w", err)
		}
		n.hasText = true
		n.text = string(text)
		return n, nil
	}
	if b.cfg.nestedElements {
		return b.buildFieldNested(name, v)
	}
	return b.buildFieldText(name, v)
}

func (b *builder) buildFieldText(name string, v reflect.Value) (node, error) {
	n := node{name: name}
	v, err := deref(v)
	if err != nil {
		return n, err
	}
	if !v.IsValid() {
		return n, nil
	}
	if tm, ok := asTextMarshaler(v); ok {
		text, err := tm.MarshalText()
		if err != nil {
			return n, fmt.Errorf("marshal text: %w", err)
		}
		n.hasText = true
		n.text = string(text)
		return n, nil
	}
	if isByteSequence(v) {
		n.hasText = true
		n.text = base64.StdEncoding.EncodeToString(bytesOf(v))
		return n, nil
	}

	switch v.Kind() {
	case reflect.Map, reflect.Struct, reflect.Slice, reflect.Array:
		text, err := b.repr(v)
		if err != nil {
			return n, err
		}
		n.hasText = true
		n.text = text
	default:
		text, err := formatScalar(v)
		if err != nil {
			return n, err
		}
		n.hasText = true
		n.text = text
	}
	return n, nil
}

func (b *builder) buildFieldNested(name string, v reflect.Value) (node, error) {
	n := node{name: name}
	v = unwrapInterface(v)
	if !v.IsValid() {
		return n, nil
	}
	if tm, ok := asTextMarshaler(v); ok {
		text, err := tm.MarshalText()
		if err != nil {
			return n, fmt.Errorf("marshal text: %w", err)
		}
		n.hasText = true
		n.text = string(text)
		return n, nil
	}
	if isByteSequence(v) {
		n.hasText = true
		n.text = base64.StdEncoding.EncodeToString(bytesOf(v))
		return n, nil
	}

	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return n, nil
		}
		if !b.enter(v) {
			return n, errCycle
		}
		defer b.leave(v)
		return b.buildFieldNested(name, v.Elem())
	case reflect.Map:
		if v.IsNil() {
			return n, nil
		}
		if !b.enter(v) {
			return n, errCycle
		}
		defer b.leave(v)
		children, err := b.mapChildren(v)
		if err != nil {
			return n, err
		}
		n.children = children
		return n, nil
	case reflect.Struct:
		children, err := b.structChildren(v)
		if err != nil {
			return n, err
		}
		n.children = children
		return n, nil
	case reflect.Slice:
		if v.IsNil() {
			return n, nil
		}
		if !b.enter(v) {
			return n, errCycle
		}
		defer b.leave(v)
		children, err := b.sliceChildren(v)
		if err != nil {
			return n, err
		}
		n.children = children
		return n, nil
	case reflect.Array:
		children, err := b.sliceChildren(v)
		if err != nil {
			return n, err
		}
		n.children = children
		return n, nil
	default:
		text, err := formatScalar(v)
		if err != nil {
			return n, err
		}
		n.hasText = true
		n.text = text
		return n, nil
	}
}

func (b *builder) mapChildren(v reflect.Value) ([]node, error) {
	if v.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("map key type %s is not supported; use string keys", v.Type().Key())
	}
	keys := v.MapKeys()
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })

	children := make([]node, 0, len(keys))
	for _, key := range keys {
		name := key.String()
		if err := validateXMLName(name); err != nil {
			return nil, fmt.Errorf("map key %q: %w", name, err)
		}
		child, err := b.buildField(name, v.MapIndex(key))
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", name, err)
		}
		children = append(children, child)
	}
	return children, nil
}

func (b *builder) sliceChildren(v reflect.Value) ([]node, error) {
	children := make([]node, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		child, err := b.buildField(b.cfg.item, v.Index(i))
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", i, err)
		}
		children = append(children, child)
	}
	return children, nil
}

func (b *builder) enter(v reflect.Value) bool {
	key, ok := visitKeyFor(v)
	if !ok {
		return true
	}
	if b.visiting[key] {
		return false
	}
	b.visiting[key] = true
	return true
}

func (b *builder) leave(v reflect.Value) {
	if key, ok := visitKeyFor(v); ok {
		delete(b.visiting, key)
	}
}

func visitKeyFor(v reflect.Value) (visitKey, bool) {
	switch v.Kind() {
	case reflect.Ptr, reflect.Map:
		if v.IsNil() {
			return visitKey{}, false
		}
		return visitKey{typ: v.Type(), ptr: v.Pointer()}, true
	case reflect.Slice:
		if v.IsNil() {
			return visitKey{}, false
		}
		return visitKey{typ: v.Type(), ptr: v.Pointer(), len: v.Len(), cap: v.Cap()}, true
	default:
		return visitKey{}, false
	}
}

func deref(v reflect.Value) (reflect.Value, error) {
	seen := make(map[visitKey]bool)
	for {
		v = unwrapInterface(v)
		if !v.IsValid() {
			return v, nil
		}
		if v.Kind() != reflect.Ptr {
			return v, nil
		}
		if v.IsNil() {
			return reflect.Value{}, nil
		}
		if key, ok := visitKeyFor(v); ok {
			if seen[key] {
				return reflect.Value{}, errCycle
			}
			seen[key] = true
		}
		v = v.Elem()
	}
}

func unwrapInterface(v reflect.Value) reflect.Value {
	for v.IsValid() && v.Kind() == reflect.Interface {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

func isByteSequence(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		return v.Type().Elem().Kind() == reflect.Uint8
	default:
		return false
	}
}

func bytesOf(v reflect.Value) []byte {
	if v.Kind() == reflect.Slice {
		return v.Bytes()
	}
	out := make([]byte, v.Len())
	for i := range out {
		out[i] = byte(v.Index(i).Uint())
	}
	return out
}

func formatScalar(v reflect.Value) (string, error) {
	v = unwrapInterface(v)
	if !v.IsValid() {
		return "", nil
	}
	switch v.Kind() {
	case reflect.String:
		return v.String(), nil
	case reflect.Bool:
		return strconv.FormatBool(v.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32:
		return formatScalarFloat(v.Float(), 32), nil
	case reflect.Float64:
		return formatScalarFloat(v.Float(), 64), nil
	default:
		return "", fmt.Errorf("unsupported type %s", v.Type())
	}
}

func formatScalarFloat(f float64, bits int) string {
	return strconv.FormatFloat(f, 'f', 1, bits)
}
