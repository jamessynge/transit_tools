package compare

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func toStringPtr(s string) *string {
	x := fmt.Sprintf("%s", s)
	return &x
}

func makeConfig(t *testing.T, args ...interface{}) *Config {
	if t == nil && len(args) == 0 {
		return nil
	}
	config := new(Config)
	config.Logger = t
	if len(args) > 0 {
		config.UseBuiltinEquals = make(map[reflect.Type]bool)
		for _, arg := range args {
			config.UseBuiltinEquals[reflect.TypeOf(arg)] = true
		}
	}
	return config
}

/*
func log(t *testing.T, s string) {
	if t != nil {
		t.Log(s)
	}
}

func logf(t *testing.T, format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	log(t, s)
}
*/

func deepCompareWrapper3(a, b interface{}, t *testing.T, args ...interface{}) (
	eq bool, diffs []*Difference) {
	t.Log("======================================================")
	t.Logf("deepCompareWrapper3\n\ta: %#v\n\tb: %#v", a, b)
	config := makeConfig(t, args...)
	eq, diffs = DeepCompare3(a, b, config)
	t.Logf("deepCompareWrapper3 -> %v\n\ta: %#v\n\tb: %#v", eq, a, b)
	if eq != (len(diffs) == 0) {
		panic(fmt.Errorf(
			"Mismatch from DeepCompare!\na: %#v\nb: %#v\nareEqual: %v\ndiffs: %#v",
			a, b, eq, diffs))
	}
	t.Log("======================================================")
	return eq, diffs
}

func deepCompareWrapper(a, b interface{}, t *testing.T, args ...interface{}) bool {
	t.Log("======================================================")
	t.Logf("deepCompareWrapper\n\ta: %#v\n\tb: %#v", a, b)
	config := makeConfig(t, args...)
	eq, diffs := DeepCompare3(a, b, config)
	t.Logf("deepCompareWrapper -> %v\n\ta: %#v\n\tb: %#v", eq, a, b)
	if eq != (len(diffs) == 0) {
		panic(fmt.Errorf(
			"Mismatch from DeepCompare!\na: %#v\nb: %#v\nareEqual: %v\ndiffs: %#v",
			a, b, eq, diffs))
	}
	t.Log("======================================================\n")
	return eq // && false
}

func TestDeepCompareDiffString(t *testing.T) {
	type testStruct struct {
		Str string
	}

	struct1 := testStruct{"A"}
	struct2 := testStruct{"BB"}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs != nil && len(diffs) == 1 {
		diff := diffs[0]
		val1, val2 := diff.Value1, diff.Value2
		if val1 != "A" || val2 != "BB" {
			t.Errorf("Name: %v, Value1: %v, Value2: %v \n", diff.FieldName, diff.Value1, diff.Value2)
		}
	} else {
		t.Errorf("Incorrect number of diffs returned")
	}
}

func TestDeepCompareDiffInt(t *testing.T) {
	type testStruct struct {
		Integer int
	}

	struct1 := testStruct{1}
	struct2 := testStruct{2}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs != nil && len(diffs) == 1 {
		diff := diffs[0]
		t.Logf("diff: %#v\n", diff)
		if diff.FieldName != "Integer" {
			t.Errorf("Name: %v, Value1: %v, Value2: %v \n",
				diff.FieldName, diff.Value1, diff.Value2)
		}
		if diff.Value1 != 1 {
			t.Errorf("Name: %v, Value1: %v, Value2: %v \n",
				diff.FieldName, diff.Value1, diff.Value2)
		}
		if diff.Value2 != 2 {
			t.Errorf("Name: %v, Value1: %v, Value2: %v \n",
				diff.FieldName, diff.Value1, diff.Value2)
		}
	} else {
		t.Errorf("Incorrect number of diffs returned")
	}
}

func TestDeepCompareDiffFloat(t *testing.T) {
	type testStruct struct {
		F float64
	}

	struct1 := testStruct{1.01}
	struct2 := testStruct{2.02}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs != nil && len(diffs) == 1 {
		diff := diffs[0]
		if diff.FieldName != "F" || diff.Value1 != 1.01 || diff.Value2 != 2.02 {
			t.Errorf("Name: %v, Value1: %v, Value2: %v \n", diff.FieldName, diff.Value1, diff.Value2)
		}
	} else {
		t.Errorf("Incorrect number of diffs returned")
	}
}

func TestDeepCompareDiffBool(t *testing.T) {
	type testStruct struct {
		B bool
	}

	struct1 := testStruct{true}
	struct2 := testStruct{false}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs != nil && len(diffs) == 1 {
		diff := diffs[0]
		if diff.FieldName != "B" || diff.Value1 != true || diff.Value2 != false {
			t.Errorf("Name: %v, Value1: %v, Value2: %v \n", diff.FieldName, diff.Value1, diff.Value2)
		}
	} else {
		t.Errorf("Incorrect number of diffs returned")
	}
}

func aaaaaTestDeepCompareDiffTime(t *testing.T) {
	type testStruct struct {
		Ti time.Time
	}

	struct1 := testStruct{time.Now()}
	struct2 := testStruct{time.Now().AddDate(0, 0, 1)}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs != nil && len(diffs) == 1 {
		diff := diffs[0]
		if !strings.HasPrefix(diff.FieldName, "Ti.") {
			t.Errorf("Name: %v, Value1: %v, Value2: %v \n", diff.FieldName, diff.Value1, diff.Value2)
		}
	} else {
		t.Errorf("Incorrect number of diffs returned")
	}
}

func TestDeepCompareSameString(t *testing.T) {
	type testStruct struct {
		Str string
	}
	struct1 := testStruct{"Luke"}
	struct2 := testStruct{"Luke"}
	areEqual, diffs := deepCompareWrapper3(struct1, struct2, t)
	if !areEqual {
		t.Errorf("areEqual is false")
	}
	if diffs == nil || len(diffs) != 0 {
		t.Errorf("Incorrect number of diffs returned: %v", diffs)
	}
}

func TestDeepCompareSameInt(t *testing.T) {
	type testStruct struct {
		Integer int
	}

	struct1 := testStruct{1}
	struct2 := testStruct{1}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs == nil || len(diffs) != 0 {
		t.Errorf("Incorrect number of diffs returned")
	}
}

func TestDeepCompareSameBool(t *testing.T) {
	type testStruct struct {
		b bool
	}

	struct1 := testStruct{true}
	struct2 := testStruct{true}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs == nil || len(diffs) != 0 {
		t.Errorf("Incorrect number of diffs returned")
	}
}

func TestDeepCompareMultiple(t *testing.T) {
	type testStruct struct {
		Str     string
		Integer int
		F       float64
		B       bool
	}

	struct1 := testStruct{"a", 1, 1.01, true}
	struct2 := testStruct{"b", 2, 2.02, false}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs == nil || len(diffs) != 4 {
		t.Errorf("Incorrect number of diffs returned")
	}
}

func TestDeepCompareDiffStringPtr(t *testing.T) {
	type testStruct struct {
		Str *string
	}

	struct1 := testStruct{toStringPtr("A")}
	struct2 := testStruct{toStringPtr("BB")}

	t.Logf("struct1: %v\n", struct1)
	t.Logf("struct2: %v\n", struct2)

	areEqual, diffs := deepCompareWrapper3(struct1, struct2, t)
	if areEqual {
		t.Errorf("areEqual == true!")
	}
	if diffs != nil && len(diffs) == 1 {
		diff := diffs[0]
		if diff.FieldName != "Str" ||
			diff.Value1 != "A" || diff.Value2 != "BB" {
			t.Errorf("Name: %v, Value1: %v, Value2: %v \n",
				diff.FieldName, diff.Value1, diff.Value2)
		}
	} else {
		t.Errorf("Incorrect number of diffs returned: %v", diffs)
	}
}

func TestDeepCompareDiffStringNilPtr(t *testing.T) {
	type testStruct struct {
		Str *string
	}

	struct1 := testStruct{nil}
	struct2 := testStruct{toStringPtr("BB")}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs != nil && len(diffs) == 1 {
		diff := diffs[0]
		if diff.FieldName != "Str" {
			t.Errorf("Incorrect FieldName: %#v\n", diff.FieldName)
		}
		if diff.Value1 != struct1.Str {
			t.Errorf("struct1.Str: %#v\n", struct1.Str)
			t.Errorf("Incorrect diff.Value1: %#v\n", diff.Value1)
			t.Errorf("Incorrect TypeOf(diff.Value1): %#v\n", reflect.TypeOf(diff.Value1))
		}
		if diff.Value2 != struct2.Str {
			t.Errorf("struct2.Str: %#v\n", struct2.Str)
			t.Errorf("Incorrect diff.Value2: %#v\n", diff.Value2)
			t.Errorf("Incorrect TypeOf(diff.Value2): %#v\n", reflect.TypeOf(diff.Value2))
		}
	} else {
		t.Errorf("Incorrect number of diffs returned")
	}
}

func TestDeepCompareSameStringPtr(t *testing.T) {
	type testStruct struct {
		Str *string
	}

	struct1 := testStruct{toStringPtr("AA")}
	struct2 := testStruct{toStringPtr("AA")}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs == nil || len(diffs) != 0 {
		t.Errorf("Incorrect number of diffs returned")
	}
}

func TestDeepCompareDiffIntPtr(t *testing.T) {
	type testStruct struct {
		In *int
	}

	i1 := 1
	i2 := 2
	struct1 := testStruct{&i1}
	struct2 := testStruct{&i2}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs != nil && len(diffs) == 1 {
		diff := diffs[0]
		if diff.FieldName != "In" || diff.Value1 != 1 || diff.Value2 != 2 {
			t.Errorf("Name: %v, Value1: %v, Value2: %v \n", diff.FieldName, diff.Value1, diff.Value2)
		}
	} else {
		t.Errorf("Incorrect number of diffs returned")
	}
}

func TestDeepCompareDiffIntNilPtr(t *testing.T) {
	type testStruct struct {
		In *int
	}

	var i1 *int
	i1 = nil
	i2 := 2
	struct1 := testStruct{i1}
	struct2 := testStruct{&i2}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs != nil && len(diffs) == 1 {
		diff := diffs[0]
		if diff.FieldName != "In" {
			t.Errorf("Incorrect FieldName: %#v\n", diff.FieldName)
		}
		if diff.Value1 != struct1.In {
			t.Errorf("struct1.In: %#v\n", struct1.In)
			t.Errorf("Incorrect diff.Value1: %#v\n", diff.Value1)
			t.Errorf("Incorrect TypeOf(diff.Value1): %#v\n", reflect.TypeOf(diff.Value1))
		}
		if diff.Value2 != struct2.In {
			t.Errorf("struct2.In: %#v\n", struct2.In)
			t.Errorf("Incorrect diff.Value2: %#v\n", diff.Value2)
			t.Errorf("Incorrect TypeOf(diff.Value2): %#v\n", reflect.TypeOf(diff.Value2))
		}
	} else {
		t.Errorf("Incorrect number of diffs returned")
	}
}

func TestDeepCompareSameIntPtr(t *testing.T) {
	type testStruct struct {
		In *int
	}

	i1 := 1
	i2 := 1
	struct1 := testStruct{&i1}
	struct2 := testStruct{&i2}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs == nil || len(diffs) != 0 {
		t.Errorf("Incorrect number of diffs returned: %v", len(diffs))
	}
}

func TestDeepCompareDiffBoolPtr(t *testing.T) {
	type testStruct struct {
		B *bool
	}

	b1 := true
	b2 := false
	struct1 := testStruct{&b1}
	struct2 := testStruct{&b2}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs != nil && len(diffs) == 1 {
		diff := diffs[0]
		if diff.FieldName != "B" || diff.Value1 != true || diff.Value2 != false {
			t.Errorf("Name: %v, Value1: %v, Value2: %v \n", diff.FieldName, diff.Value1, diff.Value2)
		}
	} else {
		t.Errorf("Incorrect number of diffs returned")
	}
}

func TestDeepCompareSameBoolPtr(t *testing.T) {
	type testStruct struct {
		B *bool
	}

	b1 := true
	b2 := true
	struct1 := testStruct{&b1}
	struct2 := testStruct{&b2}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs == nil || len(diffs) != 0 {
		t.Errorf("Incorrect number of diffs returned")
	}
}

func TestDeepCompareDiffTimePtr(t *testing.T) {
	type testStruct struct {
		Ti *time.Time
	}

	time1 := time.Now()
	time2 := time.Now().AddDate(0, 0, 1)

	struct1 := testStruct{&time1}
	struct2 := testStruct{&time2}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs != nil && len(diffs) == 1 {
		diff := diffs[0]
		if !strings.HasPrefix(diff.FieldName, "Ti.") {
			t.Errorf("Name: %v, Value1: %v, Value2: %v \n", diff.FieldName, diff.Value1, diff.Value2)
		}
	} else {
		t.Errorf("Incorrect number of diffs returned")
	}
}

func TestDeepComparePtrStruct(t *testing.T) {
	type testStruct struct {
		Str string
	}

	struct1 := &testStruct{"A"}
	struct2 := &testStruct{"BB"}
	_, diffs := deepCompareWrapper3(struct1, struct2, t)
	if diffs != nil && len(diffs) == 1 {
		diff := diffs[0]
		if diff.FieldName != "Str" || diff.Value1 != "A" || diff.Value2 != "BB" {
			t.Errorf("Name: %v, Value1: %v, Value2: %v \n", diff.FieldName, diff.Value1, diff.Value2)
		}
	} else {
		t.Errorf("Incorrect number of diffs returned: %v", len(diffs))
	}
}

type Basic struct {
	x int
	y float32
}

type NotBasic Basic

type DeepEqualTest struct {
	a, b interface{}
	eq   bool
}

// Member functions for DeepEqual tests
func (_ *Basic) String() string {
	return "a string"
}

var (
	basic1 = &Basic{1, 0.5}
	basic2 = &Basic{1, 0.5}
	// Functions for DeepEqual tests.
	fn1 func()             // nil.
	fn2 func()             // nil.
	fn3 = func() { fn1() } // Not nil.
	fn4 = fn3              // Not nil.
	fn5 = basic1.String
	fn6 = basic1.String
	fn7 = basic2.String
	fn8 = basic2.String
)

var deepEqualTests = []DeepEqualTest{
	// Equalities
	{nil, nil, true},
	{1, 1, true},
	{int32(1), int32(1), true},
	{0.5, 0.5, true},
	{float32(0.5), float32(0.5), true},
	{"hello", "hello", true},
	{make([]int, 10), make([]int, 10), true},
	{&[3]int{1, 2, 3}, &[3]int{1, 2, 3}, true},
	{Basic{1, 0.5}, Basic{1, 0.5}, true},
	{error(nil), error(nil), true},
	{map[int]string{1: "one", 2: "two"}, map[int]string{2: "two", 1: "one"}, true},
	{basic1, basic1, true},
	{basic1, basic2, true},
	{fn1, fn2, true},
	{fn3, fn3, true},
	{fn3, fn4, true},
	{fn5, fn5, true},

	// Inequalities
	{1, 2, false},
	{int32(1), int32(2), false},
	{0.5, 0.6, false},
	{float32(0.5), float32(0.6), false},
	{"hello", "hey", false},
	{make([]int, 10), make([]int, 11), false},
	{&[3]int{1, 2, 3}, &[3]int{1, 2, 4}, false},
	{Basic{1, 0.5}, Basic{1, 0.6}, false},
	{Basic{1, 0}, Basic{2, 0}, false},
	{map[int]string{1: "one", 3: "two"}, map[int]string{2: "two", 1: "one"}, false},
	{map[int]string{1: "one", 2: "txo"}, map[int]string{2: "two", 1: "one"}, false},
	{map[int]string{1: "one"}, map[int]string{2: "two", 1: "one"}, false},
	{map[int]string{2: "two", 1: "one"}, map[int]string{1: "one"}, false},
	{nil, 1, false},
	{1, nil, false},
	{fn1, fn3, false},
	{fn5, fn6, false},
	{fn7, fn8, false},
	{fn5, fn8, false},

	// Nil vs empty: not the same.
	{[]int{}, []int(nil), false},
	{[]int{}, []int{}, true},
	{[]int(nil), []int(nil), true},
	{map[int]int{}, map[int]int(nil), false},
	{map[int]int{}, map[int]int{}, true},
	{map[int]int(nil), map[int]int(nil), true},

	// Mismatched types
	{1, 1.0, false},
	{int32(1), int64(1), false},
	{0.5, "hello", false},
	{[]int{1, 2, 3}, [3]int{1, 2, 3}, false},
	{&[3]interface{}{1, 2, 4}, &[3]interface{}{1, 2, "s"}, false},
	{Basic{1, 0.5}, NotBasic{1, 0.5}, false},
	{map[uint]string{1: "one", 2: "two"}, map[int]string{2: "two", 1: "one"}, false},
}

func TestDeepEqual(t *testing.T) {
	for i, test := range deepEqualTests {
		t.Logf("\ndeepEqualTests[%d] = %v\n", i, test)
		r := deepCompareWrapper(test.a, test.b, t)
		t.Logf("\nr=%v\ndeepEqualTests[%d] = %v\n", r, i, test)
		if r != test.eq {
			t.Errorf("%d deepCompareWrapper(%v, %v) = %v, want %v", i, test.a, test.b, r, test.eq)
		}
	}
}

func TestTypeOf(t *testing.T) {
	// Special case for nil
	for _, test := range deepEqualTests {
		v := reflect.ValueOf(test.a)
		if !v.IsValid() {
			continue
		}
		typ := reflect.TypeOf(test.a)
		if typ != v.Type() {
			t.Errorf("TypeOf(%v) = %v, but ValueOf(%v).Type() = %v", test.a, typ, test.a, v.Type())
		}
	}
}

type Recursive struct {
	x int
	r *Recursive
}

func TestDeepEqualRecursiveStruct(t *testing.T) {
	a, b := new(Recursive), new(Recursive)
	*a = Recursive{12, a}
	*b = Recursive{12, b}
	if !deepCompareWrapper(a, b, t) {
		t.Error("deepCompareWrapper(recursive same) = false, want true")
	}
}

type _Complex struct {
	a int
	b [3]*_Complex
	c *string
	d map[float64]float64
}

func TestDeepEqualComplexStruct(t *testing.T) {
	m := make(map[float64]float64)
	stra, strb := "hello", "hello"
	a, b := new(_Complex), new(_Complex)
	*a = _Complex{5, [3]*_Complex{a, b, a}, &stra, m}
	*b = _Complex{5, [3]*_Complex{b, a, a}, &strb, m}
	if !deepCompareWrapper(a, b, t) {
		t.Error("deepCompareWrapper(complex same) = false, want true")
	}
}

func TestDeepEqualComplexStructInequality(t *testing.T) {
	m := make(map[float64]float64)
	stra, strb := "hello", "helloo" // Difference is here
	a, b := new(_Complex), new(_Complex)
	*a = _Complex{5, [3]*_Complex{a, b, a}, &stra, m}
	*b = _Complex{5, [3]*_Complex{b, a, a}, &strb, m}
	if deepCompareWrapper(a, b, t) {
		t.Error("deepCompareWrapper(complex different) = true, want false")
	}
}

type UnexpT struct {
	m map[int]int
}

func TestDeepEqualUnexportedMap(t *testing.T) {
	// Check that deepCompareWrapper can look at unexported fields.
	x1 := UnexpT{map[int]int{1: 2}}
	x2 := UnexpT{map[int]int{1: 2}}
	if !deepCompareWrapper(&x1, &x2, t) {
		t.Error("deepCompareWrapper(x1, x2) = false, want true")
	}

	y1 := UnexpT{map[int]int{2: 3}}
	if deepCompareWrapper(&x1, &y1, t) {
		t.Error("deepCompareWrapper(x1, y1) = true, want false")
	}
}

func BenchmarkCompareMultiple(b *testing.B) {
	b.StopTimer()

	type testStruct struct {
		Str     string
		Integer int
		F       float64
		B       bool
	}

	struct1 := &testStruct{"a", 1, 1.01, true}
	struct2 := &testStruct{"b", 2, 2.02, false}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DeepCompare3(struct1, struct2, nil)
	}
}
