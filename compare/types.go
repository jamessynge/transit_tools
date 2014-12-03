// Inspired by https://groups.google.com/d/msg/golang-nuts/neGz_Rxtxxw/EDNKIGhk44EJ
package compare

import (
	"reflect"
)

type Logger interface {
	Log(args ...interface{})
	Logf(format string, args ...interface{})
}

type Config struct {
	// For types in this collection, == is used to compare instances rather than
	// examining instances.  Primitive types are always compared with ==.
	UseBuiltinEquals map[reflect.Type]bool

	// Optional function for logging progress while comparing.  A debugging
	// tool.  If nil, no logging will occur.
	Logger Logger
}

type Difference struct {
	// The name (or path, such as 'MapField["SomeKey"].AnotherField') of the
	// field that is different.
	FieldName string

	// The type of the field; if TypeDiff is true, this is the static type,
	// and Value1 and Value2 contain the dynamic types that arent' the same.
	FieldType reflect.Type

	// The two values that are different.  Will not be filled in for values that
	// are not exported AND are not primitive (i.e. reflect.Indirect() panics,
	// and we can't work around it).
	Value1 interface{}
	Value2 interface{}

	// The reflect.Value wrappers for the two values that are different.
	ReflectValue1 reflect.Value
	ReflectValue2 reflect.Value

	TypeDiff bool // Are the types different
	Msg      string
}

type Differences []*Difference
