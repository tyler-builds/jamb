package ui_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// fetchLog counts the fetches of a test and records their contexts, which the
// fetches run off the UI thread.
type fetchLog struct {
	calls atomic.Int32
	mu    sync.Mutex
	ctxs  []context.Context
}

func (f *fetchLog) record(c context.Context) {
	f.mu.Lock()
	f.ctxs = append(f.ctxs, c)
	f.mu.Unlock()
	f.calls.Add(1) // after the context is kept, so a waiter that sees the count can read it
}

func (f *fetchLog) ctx(i int) context.Context {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ctxs[i]
}

// A query that is loaded shows its data, and Loading is true only until the data arrives.
func TestQueryLoadsData(t *testing.T) {
	var last ui.QueryResult[string]
	var log fetchLog
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		last = ui.Query(ctx, "greeting", func(c context.Context) (string, error) {
			log.record(c)
			return "hello", nil
		})
		return ui.Div().Key("root")
	})
	if !last.Loading || !last.Fetching {
		t.Errorf("first render: Loading %v, Fetching %v; want both true", last.Loading, last.Fetching)
	}
	u.WaitFor("the data", func() bool { return last.Data == "hello" })
	if last.Loading || last.Fetching || last.Err != nil {
		t.Errorf("loaded: Loading %v, Fetching %v, Err %v", last.Loading, last.Fetching, last.Err)
	}
	if !last.UpdatedAt.Equal(u.Now()) {
		t.Errorf("UpdatedAt = %v, want the time of the success %v", last.UpdatedAt, u.Now())
	}
	if n := log.calls.Load(); n != 1 {
		t.Errorf("fetched %d times, want 1", n)
	}
}

// A failed fetch keeps its error, and a later success clears it.
func TestQueryErrorThenSuccess(t *testing.T) {
	var last ui.QueryResult[int]
	var fail atomic.Bool
	fail.Store(true)
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		last = ui.Query(ctx, "count", func(context.Context) (int, error) {
			if fail.Load() {
				return 0, errors.New("the server is down")
			}
			return 7, nil
		})
		return ui.Div().Key("root")
	})
	u.WaitFor("the error", func() bool { return last.Err != nil })
	if last.Loading || last.Fetching || last.Data != 0 {
		t.Errorf("after a failure: Loading %v, Fetching %v, Data %d", last.Loading, last.Fetching, last.Data)
	}
	fail.Store(false)
	last.Refetch()
	u.WaitFor("the data", func() bool { return last.Data == 7 })
	if last.Err != nil {
		t.Errorf("Err = %v after a success, want nil", last.Err)
	}
}

// QueryRetry runs a failed fetch again after 1 s, then 2 s, on the UI's clock, and keeps
// the error only when no retry is left.
func TestQueryRetryBackoff(t *testing.T) {
	var last ui.QueryResult[int]
	var log fetchLog
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		last = ui.Query(ctx, "flaky", func(c context.Context) (int, error) {
			log.record(c)
			return 0, errors.New("unavailable")
		}, ui.QueryRetry(2), ui.QueryRetryDelay(time.Second))
		return ui.Div().Key("root")
	})
	u.WaitFor("the first attempt", func() bool { return log.calls.Load() == 1 })
	// The first failure is applied within the first frames, so the first retry is due
	// between 1 s and 1.9 s on the clock, and the second is due 2 s after that.
	u.Advance(900 * time.Millisecond)
	if n := log.calls.Load(); n != 1 {
		t.Fatalf("%d attempts before 900 ms, want 1: a retry ran before its delay", n)
	}
	u.Advance(time.Second)
	u.WaitFor("the second attempt", func() bool { return log.calls.Load() == 2 })
	if last.Err != nil || !last.Fetching {
		t.Errorf("during a retry: Err %v, Fetching %v; want no error and Fetching true", last.Err, last.Fetching)
	}
	u.Advance(time.Second)
	if n := log.calls.Load(); n != 2 {
		t.Fatalf("%d attempts by 2.9 s, want 2: the second retry waits 2 s", n)
	}
	u.Advance(1100 * time.Millisecond)
	u.WaitFor("the third attempt", func() bool { return log.calls.Load() == 3 })
	u.WaitFor("the error after the last retry", func() bool { return last.Err != nil })
	if last.Fetching {
		t.Error("still Fetching after the last retry failed")
	}
}

// Two components that use one key share one fetch.
func TestQueryDedupesComponents(t *testing.T) {
	var a, b ui.QueryResult[string]
	var log fetchLog
	fetch := func(c context.Context) (string, error) {
		log.record(c)
		return "shared", nil
	}
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Children(
			ui.Component(func(ctx *ui.Context) ui.Element {
				a = ui.Query(ctx, "shared", fetch)
				return ui.Div().Key("a")
			}),
			ui.Component(func(ctx *ui.Context) ui.Element {
				b = ui.Query(ctx, "shared", fetch)
				return ui.Div().Key("b")
			}),
		)
	})
	u.WaitFor("both components to show the data", func() bool { return a.Data == "shared" && b.Data == "shared" })
	u.Advance(time.Second)
	if n := log.calls.Load(); n != 1 {
		t.Errorf("fetched %d times for two users of one key, want 1", n)
	}
}

// When a component moves to another key, the first key's late result never shows, and
// its fetch is canceled. The second key's data shows at once.
func TestQueryKeyChangeDropsOldResult(t *testing.T) {
	var shown []string
	var current ui.QueryResult[string]
	var log fetchLog
	release := make(chan struct{})
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		which := ui.Local(ctx, "which", "slow")
		k := which.Get()
		current = ui.Query(ctx, k, func(c context.Context) (string, error) {
			log.record(c)
			if k == "slow" {
				<-release
				return "late", nil // ignores its cancel, so the result must still be dropped
			}
			return "fast", nil
		})
		shown = append(shown, current.Data)
		return ui.Div().Column().Child(ui.Button("switch").Key("switch").OnClick(func() {
			which.Set("fast")
		}))
	})
	u.Click("switch")
	u.WaitFor("the new key's data", func() bool { return current.Data == "fast" })
	u.WaitFor("both fetches to start", func() bool { return log.calls.Load() == 2 })
	if log.ctx(0).Err() == nil {
		t.Error("the old key's fetch was not canceled when the component moved on")
	}
	close(release)
	u.Advance(time.Second)
	if current.Data != "fast" {
		t.Errorf("Data = %q after the old result arrived, want fast", current.Data)
	}
	for _, s := range shown {
		if s == "late" {
			t.Error("the old key's result was shown")
		}
	}
}

// QueryKeepPrevious keeps the previous key's data, with Fetching, until the new data arrives.
func TestQueryKeepPrevious(t *testing.T) {
	var current ui.QueryResult[string]
	release := make(chan struct{})
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		which := ui.Local(ctx, "which", "a")
		k := which.Get()
		current = ui.Query(ctx, k, func(c context.Context) (string, error) {
			if k == "a" {
				return "A", nil
			}
			<-release
			return "B", nil
		}, ui.QueryKeepPrevious())
		return ui.Div().Column().Child(ui.Button("next").Key("next").OnClick(func() { which.Set("b") }))
	})
	u.WaitFor("key a's data", func() bool { return current.Data == "A" })
	u.Click("next")
	if current.Data != "A" || current.Loading || !current.Fetching {
		t.Errorf("while key b loads: Data %q, Loading %v, Fetching %v; want A, false, true",
			current.Data, current.Loading, current.Fetching)
	}
	close(release)
	u.WaitFor("key b's data", func() bool { return current.Data == "B" })
	if current.Fetching {
		t.Error("still Fetching after key b's data arrived")
	}
}

// Without QueryKeepPrevious a key change with no data shows no data at all.
func TestQueryKeyChangeWithoutKeepPreviousShowsNothing(t *testing.T) {
	var current ui.QueryResult[string]
	release := make(chan struct{})
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		which := ui.Local(ctx, "which", "a")
		k := which.Get()
		current = ui.Query(ctx, k, func(c context.Context) (string, error) {
			if k == "a" {
				return "A", nil
			}
			<-release
			return "B", nil
		})
		return ui.Div().Column().Child(ui.Button("next").Key("next").OnClick(func() { which.Set("b") }))
	})
	u.WaitFor("key a's data", func() bool { return current.Data == "A" })
	u.Click("next")
	if current.Data != "" || !current.Loading {
		t.Errorf("while key b loads: Data %q, Loading %v; want no data and Loading", current.Data, current.Loading)
	}
	close(release)
	u.WaitFor("key b's data", func() bool { return current.Data == "B" })
}

// InvalidateQueries refetches the matching keys in use and leaves the others alone.
func TestQueryInvalidate(t *testing.T) {
	var one, two ui.QueryResult[int]
	var calls1, calls2 atomic.Int32
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		one = ui.Query(ctx, "one", func(context.Context) (int, error) { return int(calls1.Add(1)), nil })
		two = ui.Query(ctx, "two", func(context.Context) (int, error) { return int(calls2.Add(1)), nil })
		return ui.Div().Column().Child(ui.Button("invalidate").Key("invalidate").OnClick(func() {
			ui.InvalidateQueries(ctx, func(key string) bool { return key == "one" })
		}))
	})
	u.WaitFor("both loaded", func() bool { return one.Data == 1 && two.Data == 1 })
	u.Click("invalidate")
	u.WaitFor("key one refetched", func() bool { return one.Data == 2 })
	u.Advance(time.Second)
	if n := calls2.Load(); n != 1 {
		t.Errorf("key two was fetched %d times, want 1", n)
	}
}

// A component's fetch is canceled when the component goes away.
func TestQueryCanceledOnUnmount(t *testing.T) {
	var log fetchLog
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		visible := ui.Local(ctx, "visible", true)
		return ui.Div().Column().Children(
			ui.If(visible.Get(), ui.Component(func(ctx *ui.Context) ui.Element {
				ui.Query(ctx, "slow", func(c context.Context) (int, error) {
					log.record(c)
					<-c.Done()
					return 0, c.Err()
				})
				return ui.Div().Key("slow-user")
			})),
			ui.Button("hide").Key("hide").OnClick(func() { visible.Set(false) }))
	})
	u.WaitFor("the fetch to start", func() bool { return log.calls.Load() == 1 })
	if log.ctx(0).Err() != nil {
		t.Fatal("the fetch was canceled before anything happened")
	}
	u.Click("hide")
	if log.ctx(0).Err() == nil {
		t.Error("the fetch was not canceled when its component was removed")
	}
}

// Closing the window cancels every fetch and starts none.
func TestQueryCanceledOnWindowClose(t *testing.T) {
	var log fetchLog
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		ui.Query(ctx, "slow", func(c context.Context) (int, error) {
			log.record(c)
			<-c.Done()
			return 0, c.Err()
		})
		return ui.Div().Key("root")
	})
	u.WaitFor("the fetch to start", func() bool { return log.calls.Load() == 1 })
	u.Close()
	if log.ctx(0).Err() == nil {
		t.Error("the fetch was not canceled when the window closed")
	}
}

// A panic in fetch becomes the query's error, and the UI keeps running.
func TestQueryPanicBecomesError(t *testing.T) {
	var last ui.QueryResult[int]
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		last = ui.Query(ctx, "panics", func(context.Context) (int, error) { panic("boom") })
		return ui.Div().Key("root")
	})
	u.WaitFor("the error", func() bool { return last.Err != nil })
	if !strings.Contains(last.Err.Error(), "boom") {
		t.Errorf("Err = %v, want it to name the panic", last.Err)
	}
	if last.Fetching {
		t.Error("still Fetching after the panic")
	}
}

// QueryEnabled(false) starts no fetch, not even on Refetch.
func TestQueryDisabled(t *testing.T) {
	var last ui.QueryResult[int]
	var log fetchLog
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		last = ui.Query(ctx, "off", func(c context.Context) (int, error) {
			log.record(c)
			return 1, nil
		}, ui.QueryEnabled(false))
		return ui.Div().Key("root")
	})
	u.Advance(time.Second)
	last.Refetch()
	u.Advance(time.Second)
	if n := log.calls.Load(); n != 0 || last.Fetching || last.Loading {
		t.Errorf("disabled: %d fetches, Fetching %v, Loading %v; want none", n, last.Fetching, last.Loading)
	}
}

// QueryStaleTime keeps fresh data: a component that starts using the key later does not
// fetch again until the data is older than QueryStaleTime.
func TestQueryStaleTime(t *testing.T) {
	var log fetchLog
	var second ui.QueryResult[int]
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		visible := ui.Local(ctx, "visible", false)
		ui.Query(ctx, "fresh", func(c context.Context) (int, error) {
			log.record(c)
			return 5, nil
		}, ui.QueryStaleTime(time.Minute))
		return ui.Div().Column().Children(
			ui.Button("show").Key("show").OnClick(func() { visible.Set(true) }),
			ui.If(visible.Get(), ui.Component(func(ctx *ui.Context) ui.Element {
				second = ui.Query(ctx, "fresh", func(c context.Context) (int, error) {
					log.record(c)
					return 6, nil
				}, ui.QueryStaleTime(time.Minute))
				return ui.Div().Key("second")
			})))
	})
	u.WaitFor("the first fetch", func() bool { return log.calls.Load() == 1 })
	u.Advance(10 * time.Second)
	u.Click("show")
	u.Advance(time.Second)
	if n := log.calls.Load(); n != 1 {
		t.Errorf("%d fetches, want 1: fresh data is not fetched again", n)
	}
	if second.Data != 5 {
		t.Errorf("second user shows %d, want the cached 5", second.Data)
	}
	u.Advance(time.Minute)
	if n := log.calls.Load(); n != 1 {
		t.Errorf("%d fetches after a minute, want 1: the second user does not start one", n)
	}
}

// SetQueryData shows the value at once, as the data of a successful fetch, with no fetch.
func TestQuerySetQueryData(t *testing.T) {
	var last ui.QueryResult[string]
	var log fetchLog
	var ctxOf *ui.Context
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		ctxOf = ctx
		last = ui.Query(ctx, "item", func(c context.Context) (string, error) {
			log.record(c)
			return "server", nil
		})
		return ui.Div().Key("root")
	})
	u.WaitFor("the data", func() bool { return last.Data == "server" })
	ui.SetQueryData(ctxOf, "item", "optimistic")
	u.Frame()
	if last.Data != "optimistic" || !last.UpdatedAt.Equal(u.Now()) {
		t.Errorf("after SetQueryData: Data %q, UpdatedAt %v", last.Data, last.UpdatedAt)
	}
	u.Advance(time.Second)
	if n := log.calls.Load(); n != 1 {
		t.Errorf("%d fetches, want 1", n)
	}
}

// A key holds one type; a call with another type panics with the key's name.
func TestQueryKeyHoldsOneType(t *testing.T) {
	var ctxOf *ui.Context
	var last ui.QueryResult[int]
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		ctxOf = ctx
		last = ui.Query(ctx, "typed", func(context.Context) (int, error) { return 1, nil })
		return ui.Div().Key("root")
	})
	u.WaitFor("the data", func() bool { return last.Data == 1 })
	defer func() {
		r := recover()
		if s, _ := r.(string); !strings.Contains(s, `"typed"`) {
			t.Errorf("recover() = %v, want a panic naming the key", r)
		}
	}()
	ui.SetQueryData(ctxOf, "typed", "text")
	t.Error("SetQueryData with another type did not panic")
}
