package object

import (
	"fmt"
	"regexp"
)

type ObjectType string

const (
	NULL_OBJ  = "NULL"
	ERROR_OBJ = "ERROR"

	STRING_OBJ  = "STRING"
	INTEGER_OBJ = "INTEGER"
	BOOLEAN_OBJ = "BOOLEAN"
	REGEXP_OBJ  = "REGEXP"

	STRUCT_OBJ   = "STRUCT"
	FUNCTION_OBJ = "FUNCTION"
)

type Object interface {
	Type() ObjectType
	String() string
}

type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) String() string   { return fmt.Sprintf("%d", i.Value) }

type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) String() string   { return fmt.Sprintf("%t", b.Value) }

type String struct {
	Value string
}

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) String() string   { return fmt.Sprintf("%q", s.Value) }

type Regexp struct {
	*regexp.Regexp
}

func (r *Regexp) Type() ObjectType { return REGEXP_OBJ }
func (r *Regexp) String() string   { return r.Regexp.String() }

type Null struct{}

func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) String() string   { return "null" }

type Function struct {
	Fn func(args []Object) Object
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) String() string   { return "function" }

type Struct struct {
	Props map[string]Object
}

func (s *Struct) Type() ObjectType { return STRUCT_OBJ }
func (s *Struct) String() string   { return "struct" }

type Error struct {
	Message string
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) String() string   { return "ERROR: " + e.Message }
