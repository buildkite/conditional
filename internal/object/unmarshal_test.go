package object

import (
	"testing"
)

func TestUnmarshalString(t *testing.T) {
	into := &String{}

	if err := Unmarshal("llamas", into); err != nil {
		t.Fatal(err)
	}

	if into.Value != "llamas" {
		t.Fatalf("bad value: %#v", into.Value)
	}
}

func TestUnmarshalBoolean(t *testing.T) {
	into := &Boolean{}

	if err := Unmarshal(true, into); err != nil {
		t.Fatal(err)
	}

	if into.Value != true {
		t.Fatalf("bad value: %#v", into.Value)
	}
}

func TestUnmarshalInt(t *testing.T) {
	into := &Integer{}

	if err := Unmarshal(24, into); err != nil {
		t.Fatal(err)
	}

	if into.Value != 24 {
		t.Fatalf("bad value: %#v", into.Value)
	}
}

func TestUnmarshalInt64(t *testing.T) {
	into := &Integer{}

	if err := Unmarshal(int64(24), into); err != nil {
		t.Fatal(err)
	}

	if into.Value != 24 {
		t.Fatalf("bad value: %#v", into.Value)
	}
}

func TestUnmarshalStruct(t *testing.T) {
	into := Struct{}

	var from = struct {
		Name        string
		Age         int
		Valid       bool
		Complicated struct {
			Status bool
		}
	}{
		Name:  "Fred",
		Age:   24,
		Valid: true,
	}

	from.Complicated.Status = true

	if err := Unmarshal(from, into); err != nil {
		t.Fatal(err)
	}

	compare := Struct{
		"name":  &String{"Fred"},
		"age":   &Integer{24},
		"valid": &Boolean{true},
		"complicated": Struct{
			"status": &Boolean{true},
		},
	}

	if !compare.Equals(into) {
		t.Fatalf("bad struct %#v", into)
	}
}

func TestUnmarshalInterfaceMap(t *testing.T) {
	into := Struct{}

	var from = map[string]interface{}{
		"name":  "Fred",
		"age":   24,
		"valid": true,
		"complicated": map[string]interface{}{
			"status": true,
		},
	}

	if err := Unmarshal(from, into); err != nil {
		t.Fatal(err)
	}

	compare := Struct{
		"name":  &String{"Fred"},
		"age":   &Integer{24},
		"valid": &Boolean{true},
		"complicated": Struct{
			"status": &Boolean{true},
		},
	}

	if !compare.Equals(into) {
		t.Fatalf("bad struct %#v", into)
	}
}
