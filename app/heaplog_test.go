package app

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestHeapLogRowMatchesHeader(t *testing.T) {
	m := runtime.MemStats{HeapInuse: 100, HeapObjects: 2, TotalAlloc: 300, Mallocs: 4, NumGC: 5}
	got := heapLogRow(10*time.Second, &m, 7, 800, 900)
	if want := "10.0,100,2,300,4,5,7,800,900\n"; got != want {
		t.Errorf("heapLogRow = %q, want %q", got, want)
	}
	cols := strings.Count(heapLogHeader, ",")
	if n := strings.Count(got, ","); n != cols {
		t.Errorf("row has %d separators, header has %d", n, cols)
	}
}
