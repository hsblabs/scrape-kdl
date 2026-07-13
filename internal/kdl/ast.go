package kdl

import (
	"fmt"

	"github.com/hsblabs/scrape-kdl/internal/source"
)

type ValueKind string

const (
	ValueString ValueKind = "string"
	ValueInt    ValueKind = "int"
	ValueFloat  ValueKind = "float"
	ValueBool   ValueKind = "bool"
	ValueNull   ValueKind = "null"
)

type Value struct {
	Kind  ValueKind
	Raw   string
	Value any
	Span  source.Span
}

func (v Value) String() string {
	if v.Kind == ValueString {
		return fmt.Sprintf("%q", v.Value)
	}
	return v.Raw
}

type Property struct {
	Name  string
	Value Value
	Span  source.Span
}

type Node struct {
	Name       string
	Arguments  []Value
	Properties []Property
	Children   []*Node
	Span       source.Span
}

func (n *Node) Property(name string) (Value, bool) {
	for i := len(n.Properties) - 1; i >= 0; i-- {
		if n.Properties[i].Name == name {
			return n.Properties[i].Value, true
		}
	}
	return Value{}, false
}

func (n *Node) PropertiesNamed(name string) []Property {
	var out []Property
	for _, p := range n.Properties {
		if p.Name == name {
			out = append(out, p)
		}
	}
	return out
}

func (n *Node) ChildrenNamed(name string) []*Node {
	var out []*Node
	for _, child := range n.Children {
		if child.Name == name {
			out = append(out, child)
		}
	}
	return out
}

type Document struct {
	Path  string
	Nodes []*Node
	Span  source.Span
}
