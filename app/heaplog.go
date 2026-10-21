package app

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// heapLogHeader is the first line of a JAMB_HEAPLOG file.
const heapLogHeader = "elapsed_s,heap_inuse_bytes,heap_objects,total_alloc_bytes,mallocs,gc_cycles,goroutines,private_bytes,working_set_bytes\n"

// startHeapLog writes a row of memory counters to path now and then every interval,
// until the process exits, for soak runs. Each row is written as it is made, so a
// run that ends abruptly keeps its rows. A file that cannot be created is reported.
func startHeapLog(path string, every time.Duration) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jamb: JAMB_HEAPLOG:", err)
		return
	}
	start := time.Now()
	go func() {
		fmt.Fprint(f, heapLogHeader)
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			private, working := processMemory()
			fmt.Fprint(f, heapLogRow(time.Since(start), &m, runtime.NumGoroutine(), private, working))
			<-t.C
		}
	}()
}

// heapLogRow formats one row of a heap log: the elapsed time in seconds, then the
// memory counters, in the columns of heapLogHeader.
func heapLogRow(elapsed time.Duration, m *runtime.MemStats, goroutines int, private, working uint64) string {
	return fmt.Sprintf("%.1f,%d,%d,%d,%d,%d,%d,%d,%d\n", elapsed.Seconds(),
		m.HeapInuse, m.HeapObjects, m.TotalAlloc, m.Mallocs, m.NumGC, goroutines, private, working)
}
