package ui

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"time"
)

// maxRetryDelay caps the delay between retries, unless QueryRetryDelay asks for more.
const maxRetryDelay = 30 * time.Second

// queryOptions are what the options of Query set.
type queryOptions struct {
	retry        int
	retryDelay   time.Duration
	staleTime    time.Duration
	cacheTime    time.Duration
	keepPrevious bool
	enabled      bool
}

// QueryOption configures Query. See QueryRetry, QueryRetryDelay, QueryStaleTime, QueryCacheTime,
// QueryKeepPrevious and QueryEnabled.
type QueryOption func(*queryOptions)

// QueryRetry sets how many times a failed fetch is run again before its error is
// kept. The default is 0: desktop data is mostly local, so a failure is shown at
// once. Each retry waits longer than the one before it (see QueryRetryDelay).
func QueryRetry(n int) QueryOption { return func(o *queryOptions) { o.retry = max(n, 0) } }

// QueryRetryDelay sets how long the first retry waits. Each later retry waits twice as
// long as the one before, up to 30 seconds, or up to d when d is longer. The
// default is one second, so retries wait 1 s, 2 s, 4 s and so on.
func QueryRetryDelay(d time.Duration) QueryOption { return func(o *queryOptions) { o.retryDelay = d } }

// QueryStaleTime sets how long fetched data counts as fresh. A component that starts
// using a key fetches again when its data is older than this, and an invalidated
// key is fetched again whatever its age. The default is 0: data is stale at once,
// so each new user of a key refetches it.
func QueryStaleTime(d time.Duration) QueryOption { return func(o *queryOptions) { o.staleTime = d } }

// QueryCacheTime sets how long the data of a key that no component uses is kept. The
// default is five minutes. Zero or less drops it at once.
func QueryCacheTime(d time.Duration) QueryOption { return func(o *queryOptions) { o.cacheTime = d } }

// QueryKeepPrevious makes a key change keep showing the data of the previous key, with
// Fetching set, until the new key's data arrives. Without it a key with no data
// shows no data. The previous key is the one the Query call at the same position
// used in the component's previous render, so make the calls in the same order on
// each render.
func QueryKeepPrevious() QueryOption { return func(o *queryOptions) { o.keepPrevious = true } }

// QueryEnabled sets whether the query fetches. With false, no fetch starts: not when a
// component starts using the key, not on invalidation, and not on Refetch, which
// does nothing for a key with no enabled user. Refetch is blocked on purpose, so
// that a disabled query never fetches. The default is true.
func QueryEnabled(on bool) QueryOption { return func(o *queryOptions) { o.enabled = on } }

// QueryResult is what Query returns for one render: the data of the key and the
// state of its fetch.
type QueryResult[T any] struct {
	// Data is the last data fetched successfully for the key, or the zero value.
	Data T
	// Err is the error of the last fetch that failed, and nil once a fetch succeeds.
	Err error
	// Loading is true when there is no data yet and a fetch is in flight.
	Loading bool
	// Fetching is true while any fetch of the key is in flight, including a refetch
	// while Data is shown.
	Fetching bool
	// UpdatedAt is the time of the last successful fetch, or of SetQueryData.
	UpdatedAt time.Time

	refetch func()
}

// Refetch fetches the key again now, cancelling a fetch that is in flight. It runs
// on the UI thread, such as from a click handler.
func (r QueryResult[T]) Refetch() {
	if r.refetch != nil {
		r.refetch()
	}
}

// Query reads the data of key, fetching it when needed, and returns it for the
// component's render. Components that use the same key share one entry and one
// fetch: while a fetch is in flight, no other starts for the key. The cache is
// per window.
//
//	res := ui.Query(ctx, "commit:"+hash, func(c context.Context) (repo.Details, error) {
//		return repo.Show(c, root, hash)
//	}, ui.QueryKeepPrevious())
//	if res.Loading {
//		return ui.Skeleton()
//	}
//
// The fetch runs off the UI thread and gets a context. That context is canceled
// when no component uses the key any more, when a refetch of the key starts while
// the fetch runs, and when the window closes. A panic in fetch becomes an error.
// The result reaches the UI thread through Post. A canceled fetch's result is
// dropped, even when the fetch returns data anyway, so the cache holds only the
// data of fetches that finished while they were wanted. A component that moves to
// another key never shows the old key's result.
//
// A fetch starts when a component starts using a key whose data is missing or
// stale (see QueryStaleTime), when an invalidated key has an enabled user, and on
// Refetch. The component re-renders when the entry changes. The fetch function of
// the latest render is the one that runs.
//
// QueryKeepPrevious compares each Query call with the call at the same position in the
// component's previous render, so a component should make its Query calls in the
// same order on every render, as it would with hooks.
//
// The key holds one type: a Query and a SetQueryData for the same key must use the
// same T, or the call panics.
func Query[T any](ctx *Context, key string, fetch func(context.Context) (T, error), opts ...QueryOption) QueryResult[T] {
	inst := ctx.inst
	if inst == nil {
		panic("ui.Query(" + key + "): no component is rendering; call it inside a ui.Component render function")
	}
	o := queryOptions{retryDelay: time.Second, cacheTime: 5 * time.Minute, enabled: true}
	for _, opt := range opts {
		opt(&o)
	}
	rt := ctx.rt
	e := rt.queryCache().entry(key, reflect.TypeFor[T]())
	e.run = func(c context.Context) (any, error) { return fetch(c) }
	e.opts = o
	now := rt.now()

	if inst.queries == nil {
		inst.queries = &queryUses{}
	}
	q := inst.queries
	slot := len(q.cur)
	q.cur = append(q.cur, e)
	// The first use of the key by this component, or its first enabled render, starts
	// a fetch when the data is missing or stale. A fetch in flight is not started again.
	on, using := e.users[inst]
	e.users[inst] = o.enabled
	if !using {
		e.cancelGC()
	}
	if o.enabled && (!using || !on) && e.stale(now) && !e.fetching {
		e.start()
	}
	e.ver.Get() // re-render the component when the entry changes

	res := QueryResult[T]{Err: e.err, Fetching: e.fetching, UpdatedAt: e.updatedAt, refetch: e.refetchNow}
	if prev := q.previous(slot); o.keepPrevious && prev != nil && prev != e && !e.hasData && prev.hasData {
		// The key changed and the new one has no data yet: the previous key's data stays.
		res.Data, res.UpdatedAt = valueOf[T](prev.data), prev.updatedAt
		return res
	}
	res.Data = valueOf[T](e.data)
	res.Loading = !e.hasData && e.fetching
	return res
}

// valueOf returns the data as type T, or the zero value when there is none.
func valueOf[T any](data any) T {
	v, _ := data.(T)
	return v
}

// InvalidateQueries marks each key that match reports as stale. A key with an
// enabled user is fetched again at once, and a fetch in flight for it is canceled.
// The others are fetched when a component next uses them. It runs on the UI thread.
func InvalidateQueries(ctx *Context, match func(key string) bool) {
	c := ctx.rt.queries
	if c == nil {
		return
	}
	for key, e := range c.entries {
		if !match(key) {
			continue
		}
		e.invalid = true
		if e.inUse() {
			e.start()
		}
	}
}

// SetQueryData writes the data of key, as if a fetch had returned it, and so the
// data is fresh from now (see QueryStaleTime). It is how an optimistic update shows a
// change before the server confirms it. A fetch in flight is not canceled, so its
// result, when it arrives, replaces the value. It runs on the UI thread.
func SetQueryData[T any](ctx *Context, key string, value T) {
	e := ctx.rt.queryCache().entry(key, reflect.TypeFor[T]())
	e.data, e.hasData, e.err = value, true, nil
	e.updatedAt, e.invalid = ctx.rt.now(), false
	if len(e.users) == 0 {
		e.unused()
	}
	e.notify()
}

// queryCache holds the entries of the Query calls of one runtime. Its entries are
// read and changed on the UI thread only; a fetch's goroutine touches none of them.
type queryCache struct {
	rt      *Runtime
	ctx     context.Context // the parent of every fetch's context; canceled when the window closes
	cancel  context.CancelFunc
	entries map[string]*queryEntry
}

// queryCache returns the runtime's cache, making it on first use.
func (rt *Runtime) queryCache() *queryCache {
	if rt.queries == nil {
		ctx, cancel := context.WithCancel(context.Background())
		rt.queries = &queryCache{rt: rt, ctx: ctx, cancel: cancel, entries: map[string]*queryEntry{}}
	}
	return rt.queries
}

// entry returns the entry of key, making it when it is missing. It panics when the
// key holds another type.
func (c *queryCache) entry(key string, typ reflect.Type) *queryEntry {
	if e := c.entries[key]; e != nil {
		if e.typ != typ {
			panic(fmt.Sprintf("ui.Query(%q): the key holds %v, not %v; a key holds one type", key, e.typ, typ))
		}
		return e
	}
	e := &queryEntry{cache: c, key: key, typ: typ, users: map[*componentInstance]bool{},
		ver: NewSignal(0), opts: queryOptions{retryDelay: time.Second, cacheTime: 5 * time.Minute, enabled: true}}
	c.entries[key] = e
	return e
}

// close is called when the window closes: every fetch's context is canceled, and
// no fetch starts again.
func (c *queryCache) close() {
	c.cancel()
	for _, e := range c.entries {
		e.stopFetch()
	}
}

// queryEntry is the cached state of one key.
type queryEntry struct {
	cache *queryCache
	key   string
	typ   reflect.Type
	run   func(context.Context) (any, error) // the latest render's fetch
	opts  queryOptions                       // the latest render's options

	data      any
	hasData   bool
	err       error
	updatedAt time.Time
	invalid   bool // marked stale by InvalidateQueries, so the next use fetches it

	// users are the components that use the key, each with whether its latest
	// render enabled the query.
	users map[*componentInstance]bool
	// ver changes when the entry does, so that the components that use it re-render.
	ver *Signal[int]

	fetching bool
	gen      uint64 // counts the fetches started, so that a result of an old one is dropped
	cancel   func() // cancels the fetch in flight
	attempt  int    // the failed attempts of the fetch cycle, for QueryRetry
	retry    *timer // the wait before the next attempt
	gc       *timer // drops the entry once it has been unused for QueryCacheTime
}

// stale reports whether the data must be fetched again: there is none, it was
// marked invalid, or it is older than QueryStaleTime.
func (e *queryEntry) stale(now time.Time) bool {
	return !e.hasData || e.invalid || now.Sub(e.updatedAt) >= e.opts.staleTime
}

// inUse reports whether a component that enables the key uses it.
func (e *queryEntry) inUse() bool {
	for _, on := range e.users {
		if on {
			return true
		}
	}
	return false
}

// notify tells the components that use the entry that it changed.
func (e *queryEntry) notify() { e.ver.Set(e.ver.Peek() + 1) }

// start fetches the key now, from the first attempt, cancelling a fetch in flight.
func (e *queryEntry) start() {
	e.stopFetch()
	e.attempt = 0
	e.launch()
}

// refetchNow is the Refetch of a result. A key with no enabled user does nothing.
func (e *queryEntry) refetchNow() {
	if e.inUse() {
		e.start()
	}
}

// launch runs one attempt of the fetch in a goroutine. Its result comes back through
// Post, and finish drops it when a newer fetch has started or the fetch was stopped.
func (e *queryEntry) launch() {
	c := e.cache
	if c.ctx.Err() != nil {
		return // the window has closed
	}
	e.gen++
	gen := e.gen
	ctx, cancel := context.WithCancel(c.ctx)
	e.cancel = cancel
	e.fetching = true
	run, rt := e.run, c.rt
	go func() {
		v, err := callFetch(ctx, run)
		rt.Post(func() { e.finish(gen, v, err) })
	}()
	e.notify()
}

// finish applies the result of the attempt that started at gen. A failed attempt
// is retried when QueryRetry allows, and its error is kept only when no attempt is left.
func (e *queryEntry) finish(gen uint64, v any, err error) {
	if gen != e.gen || !e.fetching {
		return
	}
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	if err != nil && e.attempt < e.opts.retry {
		e.retry = e.cache.rt.at(e.cache.rt.now().Add(retryDelay(e.opts.retryDelay, e.attempt)), func() {
			e.retry = nil
			e.launch()
		})
		e.attempt++
		return
	}
	e.fetching = false
	if err != nil {
		e.err = err
	} else {
		e.data, e.hasData, e.err = v, true, nil
		e.updatedAt, e.invalid = e.cache.rt.now(), false
	}
	e.notify()
}

// stopFetch cancels the fetch in flight and the wait before a retry. A result that
// is still on its way is dropped.
func (e *queryEntry) stopFetch() {
	e.gen++
	e.retry.cancel()
	e.retry = nil
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	e.fetching = false
}

// drop removes the component from the users of the entry. A key that nothing uses
// stops fetching and is kept for its QueryCacheTime.
func (e *queryEntry) drop(inst *componentInstance) {
	if _, ok := e.users[inst]; !ok {
		return
	}
	delete(e.users, inst)
	if len(e.users) == 0 {
		e.unused()
	}
}

// unused stops the fetch of an entry that no component uses, and schedules its
// removal after its QueryCacheTime.
func (e *queryEntry) unused() {
	e.stopFetch()
	e.cancelGC()
	c := e.cache
	if e.opts.cacheTime <= 0 {
		delete(c.entries, e.key)
		return
	}
	e.gc = c.rt.at(c.rt.now().Add(e.opts.cacheTime), func() {
		e.gc = nil
		if len(e.users) == 0 && c.entries[e.key] == e {
			delete(c.entries, e.key)
		}
	})
}

// cancelGC keeps the entry from being removed, because a component uses it again.
func (e *queryEntry) cancelGC() {
	e.gc.cancel()
	e.gc = nil
}

// callFetch runs the fetch, and turns a panic into an error.
func callFetch(ctx context.Context, run func(context.Context) (any, error)) (v any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ui.Query: the fetch panicked: %v", r)
		}
	}()
	return run(ctx)
}

// retryDelay is how long the retry after attempt (counted from 0) waits: the base
// delay doubled for each earlier attempt, capped at maxRetryDelay or the base when
// that is longer.
func retryDelay(base time.Duration, attempt int) time.Duration {
	limit := max(base, maxRetryDelay)
	d := base
	for range attempt {
		if d >= limit {
			break
		}
		d *= 2
	}
	return min(d, limit)
}

// queryUses are the entries a component's render used, in the order of its Query
// calls: cur for the render in progress, prev for the one before.
type queryUses struct {
	prev, cur []*queryEntry
}

// previous returns the entry that the Query at slot used in the previous render, or nil.
func (q *queryUses) previous(slot int) *queryEntry {
	if slot < len(q.prev) {
		return q.prev[slot]
	}
	return nil
}

// beginQueries starts a render of the component: the uses of the last render become
// the previous ones.
func (inst *componentInstance) beginQueries() {
	if q := inst.queries; q != nil {
		q.prev, q.cur = q.cur, q.prev[:0]
	}
}

// endQueries ends a render: the entries that the render no longer uses lose the
// component as a user.
func (inst *componentInstance) endQueries() {
	q := inst.queries
	if q == nil {
		return
	}
	for _, e := range q.prev {
		if !slices.Contains(q.cur, e) {
			e.drop(inst)
		}
	}
}

// releaseQueries drops every entry the disposed component uses.
func (inst *componentInstance) releaseQueries() {
	q := inst.queries
	if q == nil {
		return
	}
	for _, e := range slices.Concat(q.prev, q.cur) {
		e.drop(inst)
	}
	q.prev, q.cur = nil, nil
}

// hostClose is called when the window closes: the fetches are canceled.
func (rt *Runtime) hostClose() {
	if rt.queries != nil {
		rt.queries.close()
	}
}
