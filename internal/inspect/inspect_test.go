package inspect

import (
	"testing"
	"time"
)

func TestPerfKeepsTheNewestSamplesOldestFirst(t *testing.T) {
	p := NewPerf(3)
	if got := p.Samples(); len(got) != 0 || p.Last() != (Sample{}) {
		t.Fatalf("an empty Perf keeps %v, last %v", got, p.Last())
	}
	for i := 1; i <= 5; i++ {
		p.Add(Sample{Nodes: i})
	}
	got := p.Samples()
	if len(got) != 3 || got[0].Nodes != 3 || got[1].Nodes != 4 || got[2].Nodes != 5 {
		t.Fatalf("Samples = %+v, want the nodes 3, 4, 5 oldest first", got)
	}
	if p.Last().Nodes != 5 {
		t.Fatalf("Last = %+v, want the sample with 5 nodes", p.Last())
	}
}

func TestPerfMaxTotalSumsTheStages(t *testing.T) {
	p := NewPerf(4)
	p.Add(Sample{Build: time.Millisecond, Layout: time.Millisecond, Paint: time.Millisecond})
	p.Add(Sample{Paint: 5 * time.Millisecond})
	p.Add(Sample{Build: 2 * time.Millisecond})
	if got := p.MaxTotal(); got != 5*time.Millisecond {
		t.Fatalf("MaxTotal = %v, want 5ms", got)
	}
}

func TestPerfWithoutSizeKeepsNothing(t *testing.T) {
	var p Perf
	p.Add(Sample{Nodes: 1})
	if len(p.Samples()) != 0 {
		t.Fatalf("a zero Perf kept a sample")
	}
}
