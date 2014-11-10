// Based on https://groups.google.com/d/msg/golang-nuts/neGz_Rxtxxw/EDNKIGhk44EJ
// and on Go's reflect.DeepEqual.
package compare

import (
	"fmt"
	"reflect"
)

// During deepValueEqual, must keep track of checks that are
// in progress.  The comparison algorithm assumes that all
// checks in progress are true when it reencounters them.
// Visited are stored in a map indexed by 17 * a1 + a2;
type visit struct {
	a1   uintptr
	a2   uintptr
	typ  reflect.Type
	next *visit
}

type compareState struct {
	useBuiltinEquals map[reflect.Type]bool
	logger           Logger
	eq               bool
	diffs            []*Difference
	visited          map[uintptr]*visit
}

type fieldPathEntry interface {
	Append(prev string) string
}

type compareStackFrame struct {
	lastFrame *compareStackFrame
	entry     fieldPathEntry
}

func (f *compareStackFrame) String() string {
	if f == nil {
		return ""
	}
	prev := f.lastFrame.String()
	this := f.entry.Append(prev)
	return this
}

type fieldNameEntry string

func (f fieldNameEntry) Append(prev string) string {
	s := string(f)
	if len(prev) == 0 {
		return s
	}
	return prev + "." + s
}

type keyEntry struct {
	key interface{}
}

func (f keyEntry) Append(prev string) string {
	k := fmt.Sprintf("%s[%#v]", prev, f.key)
	return k
}

type comparator func(fieldPath *compareStackFrame, typ reflect.Type,
	v1 reflect.Value, v2 reflect.Value,
	state *compareState)

var kindToComparator map[reflect.Kind]comparator
var ptrLikeKinds map[reflect.Kind]bool

func init() {
	kindToComparator = make(map[reflect.Kind]comparator)
	kindToComparator[reflect.Bool] = primitiveComparator
	kindToComparator[reflect.Int] = primitiveComparator
	kindToComparator[reflect.Int8] = primitiveComparator
	kindToComparator[reflect.Int16] = primitiveComparator
	kindToComparator[reflect.Int32] = primitiveComparator
	kindToComparator[reflect.Int64] = primitiveComparator
	kindToComparator[reflect.Uint] = primitiveComparator
	kindToComparator[reflect.Uint8] = primitiveComparator
	kindToComparator[reflect.Uint16] = primitiveComparator
	kindToComparator[reflect.Uint32] = primitiveComparator
	kindToComparator[reflect.Uint64] = primitiveComparator
	kindToComparator[reflect.Uintptr] = primitiveComparator
	kindToComparator[reflect.Float32] = primitiveComparator
	kindToComparator[reflect.Float64] = primitiveComparator
	kindToComparator[reflect.Complex64] = primitiveComparator
	kindToComparator[reflect.Complex128] = primitiveComparator
	kindToComparator[reflect.Array] = arrayComparator
	kindToComparator[reflect.Chan] = uncomparableComparator
	kindToComparator[reflect.Func] = uncomparableComparator
	kindToComparator[reflect.Interface] = interfaceComparator
	kindToComparator[reflect.Map] = mapComparator
	kindToComparator[reflect.Ptr] = ptrComparator
	kindToComparator[reflect.Slice] = arrayComparator
	kindToComparator[reflect.String] = primitiveComparator
	kindToComparator[reflect.Struct] = structComparator
	kindToComparator[reflect.UnsafePointer] = uncomparableComparator

	ptrLikeKinds = make(map[reflect.Kind]bool)
	ptrLikeKinds[reflect.Chan] = true
	ptrLikeKinds[reflect.Func] = true
	ptrLikeKinds[reflect.Interface] = true
	ptrLikeKinds[reflect.Map] = true
	ptrLikeKinds[reflect.Ptr] = true
	ptrLikeKinds[reflect.Slice] = true
}

func rawSafeEquals(val1, val2 interface{}, state *compareState) (eq bool) {
	state.logger.Logf("rawSafeEquals: val1=%#v\n", val1)
	state.logger.Logf("               val2=%#v\n", val2)
	defer func() {
		if p := recover(); p != nil {
			state.logger.Logf("rawSafeEquals: recovered from %v\n", p)
			eq = false
		}
	}()
	eq = (val1 == val2)
	return
}

func safeEquals(val1, val2 reflect.Value, state *compareState) (eq bool) {
	state.logger.Logf("safeEquals: type=%v\n", val1.Type())
	state.logger.Logf("            val1=%v      %#v\n", val1, val1)
	state.logger.Logf("            val2=%v      %#v\n", val2, val2)
	defer func() {
		if p := recover(); p != nil {
			state.logger.Logf("safeEquals: recovered from %v\n", p)
			eq = false
		}
	}()
	eq = rawSafeEquals(castInterface(val1), castInterface(val2), state)
	return
}

// reflect.Value.Interface panics if the value isn't from
// an exported field, so this function attempts to work around this.
func castInterface(val reflect.Value) (i interface{}) {
	if !val.IsValid() {
		return nil
	}
	k := val.Kind()
	switch k {
	case reflect.Bool:
		return val.Bool()
	case reflect.Int:
		return int(val.Int())
	case reflect.Int8:
		return int8(val.Int())
	case reflect.Int16:
		return int16(val.Int())
	case reflect.Int32:
		return int32(val.Int())
	case reflect.Int64:
		return int64(val.Int())
	case reflect.Uint:
		return uint(val.Uint())
	case reflect.Uint8:
		return uint8(val.Uint())
	case reflect.Uint16:
		return uint16(val.Uint())
	case reflect.Uint32:
		return uint32(val.Uint())
	case reflect.Uint64:
		return uint64(val.Uint())
	case reflect.Uintptr:
		return uintptr(val.Uint())
	case reflect.Float32:
		return float32(val.Float())
	case reflect.Float64:
		return float64(val.Float())
	case reflect.Complex64:
		return complex64(val.Complex())
	case reflect.Complex128:
		return complex128(val.Complex())
	case reflect.String:
		return val.String()
	}
	// Fallback to calling reflect.Value.Interface(), which may panic.
	i = val.Interface()
	return
}

func safeCastInterface(val reflect.Value, state *compareState) (
	i interface{}, ok bool) {
	defer func() {
		if p := recover(); p != nil {
			state.logger.Logf("safeCastInterface: recovered from %v\n", p)
			ok = false
		}
	}()
	i = castInterface(val)
	ok = true
	return
}

// val1 and val2 are values of the same declared type,
// though they may have different concrete types.
// For example, when comparing two maps of type map[K]I, where I is an interface
// type, we need to be able to compare two values that we know will be of the
// same interface type, but may have different concrete types.
func compareMatchedPair(
	fieldPath *compareStackFrame, typ reflect.Type,
	val1, val2 reflect.Value,
	state *compareState) {
	kind := typ.Kind()
	state.logger.Logf("compareMatchedPair: type=%v,   kind=%v\n\ttype=%#v\n", typ, kind, typ)
	if yes, _ := ptrLikeKinds[kind]; yes {
		state.logger.Logf("compareMatchedPair: val1.IsNil() -> %v     val2.IsNil() -> %v\n", val1.IsNil(), val2.IsNil())
		if val1.IsNil() || val2.IsNil() {
			if val1.IsNil() != val2.IsNil() {
				reportValueDifference(fieldPath, typ, val1, val2, state)
			}
			return
		}
		if rawSafeEquals(val1, val2, state) {
			return
		}
		// Pointers aren't the same, but maybe the underlying values are...
	}
	// From reflect.DeepEqual.
	if val1.CanAddr() && val2.CanAddr() {
		addr1 := val1.UnsafeAddr()
		addr2 := val2.UnsafeAddr()
		state.logger.Logf("compareMatchedPair: addr1=%v,  addr=%v\n", addr1, addr2)
		// Short circuit if references are identical.
		if addr1 == addr2 {
			return
		}
		// Have we already visited the pair v1 and v2?
		// Canonicalize order to reduce number of entries in visited.
		if addr1 > addr2 {
			addr1, addr2 = addr2, addr1
		}
		h := 17*addr1 + addr2
		seen := state.visited[h]
		for p := seen; p != nil; p = p.next {
			if p.a1 == addr1 && p.a2 == addr2 && p.typ == typ {
				return
			}
		}
		// Remember for later.
		state.visited[h] = &visit{addr1, addr2, typ, seen}
	}
	cmp := kindToComparator[kind]
	if cmp == nil {
		// TODO Record this as a field specific difference so that other fields
		// are compared?
		panic(fmt.Errorf("Unknown reflect.Kind value: %v", kind))
	}
	cmp(fieldPath, typ, val1, val2, state)
}

func recordDifference(diff *Difference, state *compareState) {
	state.diffs = append(state.diffs, diff)
	state.eq = false
	state.logger.Logf("difference %d: %#v\n", len(state.diffs), *diff)
}

func reportValueDifference(
	fieldPath *compareStackFrame, typ reflect.Type,
	val1, val2 reflect.Value,
	state *compareState) *Difference {
	diff := &Difference{
		FieldName:     fieldPath.String(),
		FieldType:     typ,
		ReflectValue1: val1,
		ReflectValue2: val2,
		Msg:           "Values are different",
	}
	v1, ok1 := safeCastInterface(val1, state)
	v2, ok2 := safeCastInterface(val2, state)
	if ok1 != ok2 {
		msg := fmt.Sprintf("Fields aren't both exported: %s", diff.FieldName)
		state.logger.Logf(msg)
		panic(msg)
	}
	if !ok1 || !ok2 {
		diff.Msg = "Values aren't exported"
	} else {
		diff.Value1 = v1
		diff.Value2 = v2
		state.logger.Logf("reportValueDifference:\n\tval1: %#v\n\tval2: %#v\n",
			v1, v2)
	}
	recordDifference(diff, state)
	return diff
}

func reportTypeDifference(
	fieldPath *compareStackFrame, type1, type2 reflect.Type,
	state *compareState) {
	diff := &Difference{
		FieldName: fieldPath.String(),
		Value1:    type1,
		Value2:    type2,
		TypeDiff:  true,
		Msg:       "Types are different",
	}
	recordDifference(diff, state)
}

func reportLengthDifference(
	fieldPath *compareStackFrame, typ reflect.Type,
	len1, len2 int, state *compareState) {
	diff := &Difference{
		FieldName: fieldPath.String(),
		FieldType: typ,
		Value1:    len1,
		Value2:    len2,
		Msg:       "Lengths are different",
	}
	recordDifference(diff, state)
}

func reportMissingValueDifference(
	fieldPath *compareStackFrame, typ reflect.Type,
	val1 reflect.Value, state *compareState) {
	diff := &Difference{
		FieldName:     fieldPath.String(),
		FieldType:     typ,
		ReflectValue1: val1,
		Msg:           "Missing collection value",
	}
	if val1.IsValid() {
		diff.Value1 = castInterface(val1)
	}
	recordDifference(diff, state)
}

func reportExtraValueDifference(
	fieldPath *compareStackFrame, typ reflect.Type,
	val2 reflect.Value, state *compareState) {
	diff := &Difference{
		FieldName:     fieldPath.String(),
		FieldType:     typ,
		ReflectValue2: val2,
		Msg:           "Extra collection value",
	}
	if val2.IsValid() {
		diff.Value2 = castInterface(val2)
	}
	recordDifference(diff, state)
}

// typ is an interface type, but the values will be concrete, so their types
// must be the same, else they aren't equal.
// Should have already been checked for being nil by compareMatchedPair.
func interfaceComparator(
	fieldPath *compareStackFrame, typ reflect.Type,
	val1, val2 reflect.Value,
	state *compareState) {
	elem1, elem2 := val1.Elem(), val2.Elem()
	type1, type2 := elem1.Type(), elem2.Type()
	state.logger.Logf("interfaceComparator:\ntype1=%v\ntype2=%v\n", type1, type2)
	if type1 != type2 {
		reportTypeDifference(fieldPath, type1, type2, state)
		return
	}
	compareMatchedPair(fieldPath, type1, elem1, elem2, state)
}

// Should have already been checked for being nil by compareMatchedPair.
func ptrComparator(
	fieldPath *compareStackFrame, typ reflect.Type,
	val1 reflect.Value, val2 reflect.Value,
	state *compareState) {
	state.logger.Logf("ptrComparator field %s\n", fieldPath.String())
	// Dereference the pointers, and compare the concrete types.
	// TODO Add a compareStackFrame that represents dereference.
	elem1, elem2 := reflect.Indirect(val1), reflect.Indirect(val2)
	type1, type2 := elem1.Type(), elem2.Type()
	if type1 != type2 {
		reportTypeDifference(fieldPath, type1, type2, state)
		return
	}
	// And now compare the values.
	compareMatchedPair(fieldPath, type1, elem1, elem2, state)
}

func ignoreComparator(
	fieldPath *compareStackFrame, typ reflect.Type,
	val1, val2 reflect.Value,
	state *compareState) {
	return
}

func uncomparableComparator(
	fieldPath *compareStackFrame, typ reflect.Type,
	val1, val2 reflect.Value,
	state *compareState) {
	diff := reportValueDifference(fieldPath, typ, val1, val2, state)
	diff.Msg = "Uncomparable types"
	return
}

func primitiveComparator(
	fieldPath *compareStackFrame, typ reflect.Type,
	val1, val2 reflect.Value,
	state *compareState) {
	if safeEquals(val1, val2, state) {
		return
	}
	// Report the difference
	reportValueDifference(fieldPath, typ, val1, val2, state)
}

// Compare two arrays or two slices of the same declared type.
// For slices, we don't compare capacity.
func arrayComparator(
	fieldPath *compareStackFrame, typ reflect.Type,
	val1 reflect.Value, val2 reflect.Value,
	state *compareState) {
	state.logger.Logf("arrayComparator: type=%v\n", typ)
	len1, len2 := val1.Len(), val2.Len()
	if len1 != len2 {
		reportLengthDifference(fieldPath, typ, len1, len2, state)
	}
	// Compare entries that both slices have.
	elemType := typ.Elem()
	len0 := len1
	if len0 > len2 {
		len0 = len2
	}
	for ndx := 0; ndx < len0; ndx++ {
		elem1, elem2 := val1.Index(ndx), val2.Index(ndx)
		arrayPath := &compareStackFrame{
			lastFrame: fieldPath,
			entry: keyEntry{
				key: ndx,
			},
		}
		compareMatchedPair(arrayPath, elemType, elem1, elem2, state)
	}
	// Report the elements on only one side.
	for ndx := len0; ndx < len1; ndx++ {
		arrayPath := &compareStackFrame{
			lastFrame: fieldPath,
			entry: keyEntry{
				key: ndx,
			},
		}
		reportMissingValueDifference(arrayPath, elemType, val1.Index(ndx), state)
	}
	for ndx := len0; ndx < len2; ndx++ {
		arrayPath := &compareStackFrame{
			lastFrame: fieldPath,
			entry: keyEntry{
				key: ndx,
			},
		}
		reportExtraValueDifference(arrayPath, elemType, val2.Index(ndx), state)
	}
	return
}

// Compare two maps of the same declared type (i.e. the key
// types are the same, and the value types are the same).
func mapComparator(
	fieldPath *compareStackFrame, typ reflect.Type,
	val1 reflect.Value, val2 reflect.Value,
	state *compareState) {
	elemType := typ.Elem()
	len1, len2 := val1.Len(), val2.Len()
	state.logger.Logf("mapComparator: type=%v, len1=%d, len2=%d\n", typ, len1, len2)
	if len1 != len2 {
		reportLengthDifference(fieldPath, typ, len1, len2, state)
	}
	for _, key := range val1.MapKeys() {
		mapPath := &compareStackFrame{
			lastFrame: fieldPath,
			entry: keyEntry{
				key: castInterface(key),
			},
		}
		elem1, elem2 := val1.MapIndex(key), val2.MapIndex(key)
		if !elem2.IsValid() {
			// key isn't in val2.
			reportMissingValueDifference(mapPath, elemType, elem1, state)
		} else {
			compareMatchedPair(mapPath, elemType, elem1, elem2, state)
		}
	}
	for _, key := range val2.MapKeys() {
		if val1.MapIndex(key).IsValid() {
			// The key is also in val1, so we already did this comparison.
			continue
		}
		// key isn't in val1
		elem2 := val2.MapIndex(key)
		mapPath := &compareStackFrame{
			lastFrame: fieldPath,
			entry: keyEntry{
				key: castInterface(key),
			},
		}
		reportExtraValueDifference(mapPath, elemType, elem2, state)
	}
	return
}

// Caller has checked that both v1 and v2 are both of Kind Struct of the
// same type (typ), and are valid.
func structComparator(
	fieldPath *compareStackFrame, typ reflect.Type,
	val1 reflect.Value, val2 reflect.Value,
	state *compareState) {
	state.logger.Logf("structComparator: type=%v\n", typ)
	for i, numFields := 0, val1.NumField(); i < numFields; i++ {
		// Get a reference to the field type.
		fieldType := typ.Field(i)
		// Get values of the structures' fields, and compare them.
		field1, field2 := val1.Field(i), val2.Field(i)
		fp := &compareStackFrame{
			lastFrame: fieldPath,
			entry:     fieldNameEntry(fieldType.Name),
		}
		state.logger.Logf("Comparing struct field %s of type %v\n", fp.String(), fieldType)
		state.logger.Logf("field1: %v\n", field1)
		state.logger.Logf("field2: %v\n", field2)
		compareMatchedPair(fp, fieldType.Type, field1, field2, state)
	}
	return
}

type logNothing int

func (x logNothing) Log(args ...interface{})                 {}
func (x logNothing) Logf(format string, args ...interface{}) {}

func DeepCompare3(i1, i2 interface{}, config *Config) (
	eq bool, diffs []*Difference) {
	state := &compareState{
		eq:      true,
		diffs:   make([]*Difference, 0),
		visited: make(map[uintptr]*visit),
	}
	if config != nil {
		state.useBuiltinEquals = config.UseBuiltinEquals
		state.logger = config.Logger
	}
	if state.logger == nil {
		state.logger = logNothing(0)
	}

	val1, val2 := reflect.ValueOf(i1), reflect.ValueOf(i2)
	type1, type2 := reflect.TypeOf(i1), reflect.TypeOf(i2)
	defer func() {
		if p := recover(); p != nil {
			state.logger.Logf("DeepCompare panic: %v\ntype1: %v\ntype2: %v\n\n\n",
				p, type1, type2)
			panic(p)
		}
		diffs = state.diffs
		eq = state.eq && len(diffs) == 0
		state.logger.Logf("DeepCompare eq=%v\ntype1: %v\ntype2: %v\n\n\n",
			eq, type1, type2)
	}()
	// If either is nil, they should both be nil.
	if i1 == nil || i2 == nil {
		if i1 != i2 {
			if type1 == nil {
				type2 = type2
			}
			reportValueDifference(nil, type1, val1, val2, state)
		}
		return
	}
	//Verify both v1 and v2 are the same type
	if type1 != type2 {
		reportTypeDifference(nil, type1, type2, state)
	} else {
		compareMatchedPair(nil, type1, val1, val2, state)
	}
	return
}

func DeepCompare(i1, i2 interface{}) (eq bool, diffs []*Difference) {
	return DeepCompare3(i1, i2, nil)
}
