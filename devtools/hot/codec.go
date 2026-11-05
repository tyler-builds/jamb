package hot

import (
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unsafe"
)

// node is an encoded value. Unlike encoding/json, the codec keeps unexported
// fields, keeps a pointer shared by two values shared (each pointer is written
// once, with an ID, and referred to by that ID after), and matches struct
// fields by name, so a value saved by one build of the app is read by the next
// even when fields were added, removed or reordered.
type node struct {
	K   string           `json:"k"`             // the kind; see the switch in encode
	B   bool             `json:"b,omitempty"`   // bool
	I   int64            `json:"i,omitempty"`   // int kinds
	U   uint64           `json:"u,omitempty"`   // uint kinds
	F   []float64        `json:"f,omitempty"`   // float (one) and complex (two) kinds
	S   string           `json:"s,omitempty"`   // string; and the type of an interface's value
	Bin []byte           `json:"bin,omitempty"` // []byte, and what MarshalBinary wrote
	A   []*node          `json:"a,omitempty"`   // slice and array elements
	M   [][2]*node       `json:"m,omitempty"`   // map entries
	O   map[string]*node `json:"o,omitempty"`   // struct fields, by name
	P   int              `json:"p,omitempty"`   // pointer ID
	V   *node            `json:"v,omitempty"`   // what a pointer, interface or signal holds
}

// Kinds of node.
const (
	kNil    = "nil"  // a nil pointer, slice, map or interface
	kBool   = "bool" //
	kInt    = "int"
	kUint   = "uint"
	kFloat  = "float"
	kString = "str"
	kBytes  = "bytes"
	kSeq    = "seq"  // slice or array
	kMap    = "map"  //
	kStruct = "obj"  //
	kPtr    = "ptr"  // the first time a pointer is written: P and V
	kRef    = "ref"  // a pointer written before: P
	kBin    = "bin"  // a value that writes itself, such as a time.Time
	kZero   = "zero" // a value of package sync, which restarts as its zero value
	kSignal = "sig"  // a ui.Signal: V is its value
	kIface  = "if"   // an interface: S is the type of its value, V the value
)

const uiPath = "github.com/tyler-builds/jamb/ui"

var (
	binMarshaler   = reflect.TypeFor[encoding.BinaryMarshaler]()
	binUnmarshaler = reflect.TypeFor[encoding.BinaryUnmarshaler]()
)

// isSignal reports whether t is a ui.Signal[T].
func isSignal(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && t.PkgPath() == uiPath && strings.HasPrefix(t.Name(), "Signal[")
}

// writable returns v without the read-only flag that reflect gives values
// reached through unexported fields, so that it can be read and set. v must be
// addressable.
func writable(v reflect.Value) reflect.Value {
	return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
}

// addressable returns v, or an addressable copy of it.
func addressable(v reflect.Value) reflect.Value {
	if v.CanAddr() {
		return writable(v)
	}
	c := reflect.New(v.Type()).Elem()
	c.Set(v)
	return c
}

type ptrKey struct {
	addr uintptr
	typ  reflect.Type
}

// encoder writes values, with the pointers they share numbered across all of them.
type encoder struct {
	ids  map[ptrKey]int
	next int
}

func newEncoder() *encoder { return &encoder{ids: map[ptrKey]int{}} }

// encode writes v. It fails on what cannot be restored: a func, a channel, or
// an interface holding a type the reader could not make.
func (e *encoder) encode(v reflect.Value) (*node, error) {
	t := v.Type()
	if t.Kind() == reflect.Struct {
		switch {
		case isSignal(t):
			f, ok := t.FieldByName("value")
			if !ok {
				return nil, fmt.Errorf("ui.Signal has no value field")
			}
			inner, err := e.encode(writable(addressable(v).FieldByIndex(f.Index)))
			if err != nil {
				return nil, err
			}
			return &node{K: kSignal, V: inner}, nil
		case t.PkgPath() == "sync" || t.PkgPath() == "sync/atomic":
			return &node{K: kZero}, nil
		case reflect.PointerTo(t).Implements(binUnmarshaler) && (t.Implements(binMarshaler) || reflect.PointerTo(t).Implements(binMarshaler)):
			b, err := addressable(v).Addr().Interface().(encoding.BinaryMarshaler).MarshalBinary()
			if err != nil {
				return nil, fmt.Errorf("%v: %w", t, err)
			}
			return &node{K: kBin, Bin: b}, nil
		}
	}
	switch t.Kind() {
	case reflect.Bool:
		return &node{K: kBool, B: v.Bool()}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &node{K: kInt, I: v.Int()}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return &node{K: kUint, U: v.Uint()}, nil
	case reflect.Float32, reflect.Float64:
		return &node{K: kFloat, F: []float64{v.Float()}}, nil
	case reflect.Complex64, reflect.Complex128:
		c := v.Complex()
		return &node{K: kFloat, F: []float64{real(c), imag(c)}}, nil
	case reflect.String:
		return &node{K: kString, S: v.String()}, nil
	case reflect.Slice:
		if v.IsNil() {
			return &node{K: kNil}, nil
		}
		if t.Elem().Kind() == reflect.Uint8 {
			return &node{K: kBytes, Bin: append([]byte(nil), v.Bytes()...)}, nil
		}
		fallthrough
	case reflect.Array:
		n := &node{K: kSeq, A: make([]*node, v.Len())}
		for i := range v.Len() {
			c, err := e.encode(writable(v.Index(i)))
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			n.A[i] = c
		}
		if n.A == nil {
			n.A = []*node{}
		}
		return n, nil
	case reflect.Map:
		if v.IsNil() {
			return &node{K: kNil}, nil
		}
		n := &node{K: kMap, M: [][2]*node{}}
		it := v.MapRange()
		for it.Next() {
			k, err := e.encode(addressable(it.Key()))
			if err != nil {
				return nil, fmt.Errorf("a map key: %w", err)
			}
			val, err := e.encode(addressable(it.Value()))
			if err != nil {
				return nil, fmt.Errorf("[%v]: %w", it.Key(), err)
			}
			n.M = append(n.M, [2]*node{k, val})
		}
		// In a fixed order, so that equal maps encode alike (see baseOf).
		keys := make([]string, len(n.M))
		for i, kv := range n.M {
			b, _ := json.Marshal(kv[0])
			keys[i] = string(b)
		}
		sort.Sort(byKey{n.M, keys})
		return n, nil
	case reflect.Struct:
		v = addressable(v)
		n := &node{K: kStruct, O: map[string]*node{}}
		for i := range t.NumField() {
			c, err := e.encode(writable(v.Field(i)))
			if err != nil {
				return nil, fmt.Errorf(".%s: %w", t.Field(i).Name, err)
			}
			n.O[t.Field(i).Name] = c
		}
		return n, nil
	case reflect.Pointer:
		if v.IsNil() {
			return &node{K: kNil}, nil
		}
		k := ptrKey{v.Pointer(), t}
		if id, ok := e.ids[k]; ok {
			return &node{K: kRef, P: id}, nil
		}
		e.next++
		id := e.next
		e.ids[k] = id // before the value, for a pointer that leads back to itself
		inner, err := e.encode(writable(v.Elem()))
		if err != nil {
			return nil, err
		}
		return &node{K: kPtr, P: id, V: inner}, nil
	case reflect.Interface:
		if v.IsNil() {
			return &node{K: kNil}, nil
		}
		c := v.Elem()
		if _, ok := basicTypes[c.Type().String()]; !ok {
			return nil, fmt.Errorf("holds a %v in an interface, which a restart cannot rebuild", c.Type())
		}
		inner, err := e.encode(addressable(c))
		if err != nil {
			return nil, err
		}
		return &node{K: kIface, S: c.Type().String(), V: inner}, nil
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		if v.IsNil() {
			return &node{K: kNil}, nil
		}
		return nil, fmt.Errorf("holds a %v, which a restart cannot keep", t)
	}
	return nil, fmt.Errorf("holds a %v, which a restart cannot keep", t)
}

// basicTypes are the types an interface may hold for the codec to rebuild it.
var basicTypes = map[string]reflect.Type{}

func init() {
	for _, v := range []any{false, "", 0, int8(0), int16(0), int32(0), int64(0), uint(0), uint8(0), uint16(0),
		uint32(0), uint64(0), float32(0), float64(0)} {
		basicTypes[reflect.TypeOf(v).String()] = reflect.TypeOf(v)
	}
}

// decoder reads values written by one encoder. Pointers are made once each,
// so values that shared one share it again; and a pointer that is live in this
// process, such as a signal already restored, stands for itself.
type decoder struct {
	first map[int]*node         // the node that wrote each pointer, by ID
	made  map[int]reflect.Value // the pointers made or bound so far, by ID
}

func newDecoder() *decoder {
	return &decoder{first: map[int]*node{}, made: map[int]reflect.Value{}}
}

// index records where each pointer was written in n.
func (d *decoder) index(n *node) {
	if n == nil {
		return
	}
	if n.K == kPtr {
		d.first[n.P] = n
	}
	d.index(n.V)
	for _, c := range n.A {
		d.index(c)
	}
	for _, kv := range n.M {
		d.index(kv[0])
		d.index(kv[1])
	}
	for _, c := range n.O {
		d.index(c)
	}
}

// bind makes the live pointer p stand for ID id, unless something was already
// made for it.
func (d *decoder) bind(id int, p reflect.Value) {
	if _, ok := d.made[id]; !ok {
		d.made[id] = p
	}
}

// decode reads n into v, which must be settable. A kind that does not fit v's
// type fails: the type changed since n was written.
func (d *decoder) decode(n *node, v reflect.Value) error {
	t := v.Type()
	mismatch := func() error { return fmt.Errorf("was a %s, is now a %v", n.K, t) }
	switch n.K {
	case kNil:
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Interface, reflect.Func, reflect.Chan:
			v.SetZero()
			return nil
		}
		return mismatch()
	case kZero:
		v.SetZero()
		return nil
	case kSignal:
		if !isSignal(t) {
			return mismatch()
		}
		f, _ := t.FieldByName("value")
		return d.decode(n.V, writable(v.FieldByIndex(f.Index)))
	case kBin:
		if !reflect.PointerTo(t).Implements(binUnmarshaler) {
			return mismatch()
		}
		return v.Addr().Interface().(encoding.BinaryUnmarshaler).UnmarshalBinary(n.Bin)
	case kBool:
		if t.Kind() != reflect.Bool {
			return mismatch()
		}
		v.SetBool(n.B)
	case kInt:
		switch t.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if v.OverflowInt(n.I) {
				return fmt.Errorf("%d does not fit a %v", n.I, t)
			}
			v.SetInt(n.I)
		default:
			return mismatch()
		}
	case kUint:
		switch t.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			if v.OverflowUint(n.U) {
				return fmt.Errorf("%d does not fit a %v", n.U, t)
			}
			v.SetUint(n.U)
		default:
			return mismatch()
		}
	case kFloat:
		switch t.Kind() {
		case reflect.Float32, reflect.Float64:
			if len(n.F) != 1 {
				return mismatch()
			}
			v.SetFloat(n.F[0])
		case reflect.Complex64, reflect.Complex128:
			if len(n.F) != 2 {
				return mismatch()
			}
			v.SetComplex(complex(n.F[0], n.F[1]))
		default:
			return mismatch()
		}
	case kString:
		if t.Kind() != reflect.String {
			return mismatch()
		}
		v.SetString(n.S)
	case kBytes:
		if t.Kind() != reflect.Slice || t.Elem().Kind() != reflect.Uint8 {
			return mismatch()
		}
		v.SetBytes(append([]byte(nil), n.Bin...))
	case kSeq:
		switch t.Kind() {
		case reflect.Slice:
			s := reflect.MakeSlice(t, len(n.A), len(n.A))
			for i, c := range n.A {
				if err := d.decode(c, writable(s.Index(i))); err != nil {
					return fmt.Errorf("[%d]: %w", i, err)
				}
			}
			v.Set(s)
		case reflect.Array:
			if t.Len() != len(n.A) {
				return fmt.Errorf("had %d elements, is now a %v", len(n.A), t)
			}
			for i, c := range n.A {
				if err := d.decode(c, writable(v.Index(i))); err != nil {
					return fmt.Errorf("[%d]: %w", i, err)
				}
			}
		default:
			return mismatch()
		}
	case kMap:
		if t.Kind() != reflect.Map {
			return mismatch()
		}
		m := reflect.MakeMapWithSize(t, len(n.M))
		for _, kv := range n.M {
			k := reflect.New(t.Key()).Elem()
			if err := d.decode(kv[0], k); err != nil {
				return fmt.Errorf("a map key: %w", err)
			}
			val := reflect.New(t.Elem()).Elem()
			if err := d.decode(kv[1], val); err != nil {
				return fmt.Errorf("[%v]: %w", k, err)
			}
			m.SetMapIndex(k, val)
		}
		v.Set(m)
	case kStruct:
		if t.Kind() != reflect.Struct {
			return mismatch()
		}
		for i := range t.NumField() {
			c, ok := n.O[t.Field(i).Name]
			if !ok {
				continue // a field added since: it keeps its zero value
			}
			if err := d.decode(c, writable(v.Field(i))); err != nil {
				return fmt.Errorf(".%s: %w", t.Field(i).Name, err)
			}
		}
	case kPtr, kRef:
		if t.Kind() != reflect.Pointer {
			return mismatch()
		}
		if p, ok := d.made[n.P]; ok {
			if p.Type() != t {
				return fmt.Errorf("shared a %v, is now a %v", p.Type(), t)
			}
			v.Set(p)
			return nil
		}
		w := d.first[n.P]
		if w == nil {
			return fmt.Errorf("refers to a value the checkpoint lacks")
		}
		p := reflect.New(t.Elem())
		d.made[n.P] = p // before its value, for a pointer that leads back to itself
		if err := d.decode(w.V, p.Elem()); err != nil {
			delete(d.made, n.P)
			return err
		}
		v.Set(p)
	case kIface:
		if t.Kind() != reflect.Interface {
			return mismatch()
		}
		ct, ok := basicTypes[n.S]
		if !ok || !ct.Implements(t) {
			return fmt.Errorf("held a %s, which a %v cannot", n.S, t)
		}
		c := reflect.New(ct).Elem()
		if err := d.decode(n.V, c); err != nil {
			return err
		}
		v.Set(c)
	default:
		return fmt.Errorf("unknown kind %q", n.K)
	}
	return nil
}

// byKey sorts map entries by their encoded keys.
type byKey struct {
	m    [][2]*node
	keys []string
}

func (b byKey) Len() int           { return len(b.m) }
func (b byKey) Less(i, j int) bool { return b.keys[i] < b.keys[j] }
func (b byKey) Swap(i, j int) {
	b.m[i], b.m[j] = b.m[j], b.m[i]
	b.keys[i], b.keys[j] = b.keys[j], b.keys[i]
}

// sameBase reports whether two bases are the same value as far as both say:
// struct fields that only one of them has are left out, so that a field
// added to a type or taken from it does not count as a change.
func sameBase(a, b *node) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.K != b.K || a.B != b.B || a.I != b.I || a.U != b.U || a.S != b.S || a.P != b.P ||
		!slicesEqual(a.F, b.F) || !bytesEqual(a.Bin, b.Bin) || len(a.A) != len(b.A) || len(a.M) != len(b.M) {
		return false
	}
	for i := range a.A {
		if !sameBase(a.A[i], b.A[i]) {
			return false
		}
	}
	for i := range a.M {
		if !sameBase(a.M[i][0], b.M[i][0]) || !sameBase(a.M[i][1], b.M[i][1]) {
			return false
		}
	}
	for name, x := range a.O {
		if y, ok := b.O[name]; ok && !sameBase(x, y) {
			return false
		}
	}
	return sameBase(a.V, b.V)
}

func slicesEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func bytesEqual(a, b []byte) bool { return string(a) == string(b) }
