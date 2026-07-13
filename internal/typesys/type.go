package typesys

import (
	"fmt"
	"strings"
)

type Kind string

const (
	KindPrimitive Kind = "primitive"
	KindArray     Kind = "array"
	KindNullable  Kind = "nullable"
)

type Type struct {
	Kind    Kind   `json:"kind"`
	Name    string `json:"name,omitempty"`
	Element *Type  `json:"element,omitempty"`
	Inner   *Type  `json:"inner,omitempty"`
}

var primitives = map[string]struct{}{
	"string": {}, "bool": {}, "int": {},
	"u8": {}, "u16": {}, "u32": {}, "u64": {},
	"i8": {}, "i16": {}, "i32": {}, "i64": {},
	"float": {}, "f32": {}, "f64": {},
	"object": {}, "unknown": {},
}

func Primitive(name string) Type { return Type{Kind: KindPrimitive, Name: name} }
func Nullable(inner Type) Type {
	if inner.Kind == KindNullable {
		return inner
	}
	return Type{Kind: KindNullable, Inner: &inner}
}
func Array(element Type) Type { return Type{Kind: KindArray, Element: &element} }

func Parse(input string) (Type, error) {
	p := typeParser{s: strings.TrimSpace(input)}
	t, err := p.parseType()
	if err != nil {
		return Type{}, err
	}
	p.skipSpace()
	if p.i != len(p.s) {
		return Type{}, fmt.Errorf("unexpected type syntax at byte %d", p.i)
	}
	return t, nil
}

type typeParser struct {
	s string
	i int
}

func (p *typeParser) skipSpace() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t') {
		p.i++
	}
}
func (p *typeParser) parseType() (Type, error) {
	p.skipSpace()
	var t Type
	if p.i < len(p.s) && p.s[p.i] == '(' {
		p.i++
		inner, err := p.parseType()
		if err != nil {
			return Type{}, err
		}
		p.skipSpace()
		if p.i >= len(p.s) || p.s[p.i] != ')' {
			return Type{}, fmt.Errorf("missing closing parenthesis")
		}
		p.i++
		t = inner
	} else {
		start := p.i
		for p.i < len(p.s) && ((p.s[p.i] >= 'a' && p.s[p.i] <= 'z') || (p.s[p.i] >= '0' && p.s[p.i] <= '9')) {
			p.i++
		}
		name := p.s[start:p.i]
		if _, ok := primitives[name]; !ok {
			return Type{}, fmt.Errorf("unknown primitive type %q", name)
		}
		t = Primitive(name)
	}
	for {
		p.skipSpace()
		if strings.HasPrefix(p.s[p.i:], "[]") {
			p.i += 2
			t = Array(t)
			continue
		}
		if p.i < len(p.s) && p.s[p.i] == '?' {
			if t.Kind == KindNullable {
				return Type{}, fmt.Errorf("nested nullable type is invalid")
			}
			p.i++
			t = Nullable(t)
			continue
		}
		break
	}
	return t, nil
}

func (t Type) String() string {
	switch t.Kind {
	case KindPrimitive:
		return t.Name
	case KindArray:
		return t.Element.String() + "[]"
	case KindNullable:
		return t.Inner.String() + "?"
	default:
		return "<invalid>"
	}
}

func Equal(a, b Type) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case KindPrimitive:
		return a.Name == b.Name
	case KindArray:
		return a.Element != nil && b.Element != nil && Equal(*a.Element, *b.Element)
	case KindNullable:
		return a.Inner != nil && b.Inner != nil && Equal(*a.Inner, *b.Inner)
	default:
		return false
	}
}

func IsAssignable(from, to Type) bool {
	if Equal(from, to) {
		return true
	}
	if to.Kind == KindPrimitive && to.Name == "unknown" {
		return true
	}
	if to.Kind == KindNullable && to.Inner != nil {
		if from.Kind == KindNullable && from.Inner != nil {
			return IsAssignable(*from.Inner, *to.Inner)
		}
		return IsAssignable(from, *to.Inner)
	}
	if from.Kind == KindArray && to.Kind == KindArray && from.Element != nil && to.Element != nil {
		return IsAssignable(*from.Element, *to.Element)
	}
	return false
}

func IsNullable(t Type) bool   { return t.Kind == KindNullable }
func LiftNullable(t Type) Type { return Nullable(t) }
func IsString(t Type) bool     { return t.Kind == KindPrimitive && t.Name == "string" }
func IsStringArray(t Type) bool {
	return t.Kind == KindArray && t.Element != nil && IsString(*t.Element)
}
func IsNumeric(t Type) bool {
	if t.Kind != KindPrimitive {
		return false
	}
	switch t.Name {
	case "int", "u8", "u16", "u32", "u64", "i8", "i16", "i32", "i64", "float", "f32", "f64":
		return true
	default:
		return false
	}
}
func IsInteger(t Type) bool {
	if t.Kind != KindPrimitive {
		return false
	}
	switch t.Name {
	case "int", "u8", "u16", "u32", "u64", "i8", "i16", "i32", "i64":
		return true
	default:
		return false
	}
}
func IsScalar(t Type) bool {
	if t.Kind == KindNullable && t.Inner != nil {
		return IsScalar(*t.Inner)
	}
	return t.Kind == KindPrimitive && t.Name != "object" && t.Name != "unknown"
}
