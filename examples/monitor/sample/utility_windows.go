package sample

import (
	"cmp"
	"errors"
	"slices"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	pdh                             = windows.NewLazySystemDLL("pdh.dll")
	procPdhOpenQuery                = pdh.NewProc("PdhOpenQueryW")
	procPdhAddEnglishCounter        = pdh.NewProc("PdhAddEnglishCounterW")
	procPdhCollectQueryData         = pdh.NewProc("PdhCollectQueryData")
	procPdhGetFormattedCounterArray = pdh.NewProc("PdhGetFormattedCounterArrayW")
)

const (
	pdhFmtDouble = 0x200
	pdhMoreData  = 0x800007d2
)

// pdhItem is PDH_FMT_COUNTERVALUE_ITEM_W with a double value, on 64-bit Windows.
type pdhItem struct {
	Name   *uint16
	Status uint32
	_      uint32
	Value  float64
}

// utilityReader reads "% Processor Utility", the measure Task Manager shows: the
// busy time scaled by the processors' speed against their base speed, so a
// processor that runs below its base clock counts as less busy, and one in turbo
// as busier, up to 100. The busy time alone (the processor times) reads lower than
// Task Manager on a machine that turbos, and higher on one that saves power.
type utilityReader struct {
	query, counter uintptr
	primed         bool
	buf            []byte
}

func newUtilityReader() (*utilityReader, error) {
	u := &utilityReader{}
	if r, _, _ := procPdhOpenQuery.Call(0, 0, uintptr(unsafe.Pointer(&u.query))); r != 0 {
		return nil, errors.New("PdhOpenQuery failed: 0x" + strconv.FormatUint(uint64(r), 16))
	}
	path, _ := windows.UTF16PtrFromString(`\Processor Information(*)\% Processor Utility`)
	if r, _, _ := procPdhAddEnglishCounter.Call(u.query, uintptr(unsafe.Pointer(path)), 0,
		uintptr(unsafe.Pointer(&u.counter))); r != 0 {
		return nil, errors.New("PdhAddEnglishCounter failed: 0x" + strconv.FormatUint(uint64(r), 16))
	}
	return u, nil
}

// read returns the utility of all processors and of each, in the order of their
// group and number, or false while it has no rate yet: a rate needs two
// collections, so the first read only primes it.
func (u *utilityReader) read() (total float64, cores []float64, ok bool) {
	if r, _, _ := procPdhCollectQueryData.Call(u.query); r != 0 {
		return 0, nil, false
	}
	if !u.primed {
		u.primed = true
		return 0, nil, false
	}
	var size, n uint32
	for {
		var p uintptr
		if len(u.buf) > 0 {
			p = uintptr(unsafe.Pointer(&u.buf[0]))
		}
		size = uint32(len(u.buf))
		r, _, _ := procPdhGetFormattedCounterArray.Call(u.counter, pdhFmtDouble,
			uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&n)), p)
		if r == pdhMoreData {
			u.buf = make([]byte, size)
			continue
		}
		if r != 0 {
			return 0, nil, false
		}
		break
	}
	type core struct {
		group, number int
		value         float64
	}
	var each []core
	total = -1
	for _, it := range unsafe.Slice((*pdhItem)(unsafe.Pointer(&u.buf[0])), n) {
		if it.Status != 0 {
			continue
		}
		name := windows.UTF16PtrToString(it.Name)
		if name == "_Total" {
			total = it.Value
			continue
		}
		// A processor is "group,number"; "0,_Total" is the total of a group.
		g, num, found := strings.Cut(name, ",")
		gi, err1 := strconv.Atoi(g)
		ni, err2 := strconv.Atoi(num)
		if !found || err1 != nil || err2 != nil {
			continue
		}
		each = append(each, core{gi, ni, it.Value})
	}
	if total < 0 || len(each) == 0 {
		return 0, nil, false
	}
	slices.SortFunc(each, func(a, b core) int {
		return cmp.Or(cmp.Compare(a.group, b.group), cmp.Compare(a.number, b.number))
	})
	cores = make([]float64, len(each))
	for i, c := range each {
		cores[i] = c.value
	}
	return total, cores, true
}
