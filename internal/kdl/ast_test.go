package kdl

import (
	"reflect"
	"testing"
)

func TestValueString(t *testing.T) {
	if got := (Value{Kind: ValueString, Value: "quoted"}).String(); got != `"quoted"` {
		t.Fatalf("string value = %q", got)
	}
	if got := (Value{Kind: ValueInt, Raw: "0x10", Value: int64(16)}).String(); got != "0x10" {
		t.Fatalf("integer value = %q", got)
	}
}

func TestNodePropertyUsesLastValue(t *testing.T) {
	node := &Node{Properties: []Property{
		{Name: "mode", Value: Value{Kind: ValueString, Value: "first"}},
		{Name: "other", Value: Value{Kind: ValueBool, Value: true}},
		{Name: "mode", Value: Value{Kind: ValueString, Value: "last"}},
	}}
	value, ok := node.Property("mode")
	if !ok || value.Value != "last" {
		t.Fatalf("Property(mode) = %#v, %v", value, ok)
	}
	if _, ok := node.Property("missing"); ok {
		t.Fatal("Property(missing) succeeded")
	}
}

func TestNodeNamedLookupsPreserveSourceOrder(t *testing.T) {
	first := &Node{Name: "field"}
	second := &Node{Name: "collection"}
	third := &Node{Name: "field"}
	node := &Node{
		Properties: []Property{
			{Name: "tag", Value: Value{Kind: ValueString, Value: "one"}},
			{Name: "other", Value: Value{Kind: ValueString, Value: "ignored"}},
			{Name: "tag", Value: Value{Kind: ValueString, Value: "two"}},
		},
		Children: []*Node{first, second, third},
	}
	properties := node.PropertiesNamed("tag")
	if len(properties) != 2 || properties[0].Value.Value != "one" || properties[1].Value.Value != "two" {
		t.Fatalf("PropertiesNamed(tag) = %#v", properties)
	}
	if got := node.ChildrenNamed("field"); !reflect.DeepEqual(got, []*Node{first, third}) {
		t.Fatalf("ChildrenNamed(field) = %#v", got)
	}
	if got := node.ChildrenNamed("missing"); len(got) != 0 {
		t.Fatalf("ChildrenNamed(missing) = %#v", got)
	}
}
