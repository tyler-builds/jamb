# Loading data

`ui.Query` loads data for a component: it keeps the data of each key in a cache, fetches
it off the UI thread, and shows the data, the error and the state of the fetch in the
component's render. It is the core of what TanStack Query does for web apps, without
mutations. The cache belongs to one window, so two windows never share an entry.

## A complete example

A commit's details, loaded when the commit is selected:

```go
func (v *View) detail(ctx *ui.Context, hash string) ui.Element {
	res := ui.Query(ctx, "commit:"+hash, func(c context.Context) (repo.Details, error) {
		return repo.Show(c, v.root, hash)
	}, ui.QueryKeepPrevious())

	switch {
	case res.Loading:
		return ui.Skeleton().Height(ui.Px(120))
	case res.Err != nil:
		return ui.Empty("Could not read the commit").Description(res.Err.Error())
	}
	return ui.Div().Column().Gap(8).
		Child(ui.Text(res.Data.Subject).FontSize(15)).
		Child(ui.Button("Refresh").OnClick(res.Refetch))
}
```

`repo.Show` runs on the fetch's goroutine, so it may block. It gets a `context.Context`
that is canceled when no component uses the key any more and when the window closes.
Pass the context to anything that takes one, such as `exec.CommandContext`, and a
fetch that is no longer wanted stops.

The result has these fields:

| Field | Is |
|---|---|
| `Data` | the last data fetched successfully for the key, or the zero value |
| `Err` | the error of the last fetch that failed; a success clears it |
| `Loading` | true when there is no data yet and a fetch is in flight |
| `Fetching` | true while any fetch of the key is in flight, including a refetch with data shown |
| `UpdatedAt` | the time of the last success, or of `SetQueryData` |
| `Refetch()` | fetches the key again now, cancelling a fetch in flight |

## Options

Each option is a `ui.QueryOption` passed after the fetch function:

| Option | Default | Sets |
|---|---|---|
| `ui.QueryRetry(n)` | 0 | how many times a failed fetch runs again before its error is kept |
| `ui.QueryRetryDelay(d)` | 1 s | how long the first retry waits; each later one waits twice as long, up to 30 s |
| `ui.QueryStaleTime(d)` | 0 | how long fetched data counts as fresh |
| `ui.QueryCacheTime(d)` | 5 min | how long the data of a key that no component uses is kept |
| `ui.QueryKeepPrevious()` | off | on a key change, keeps showing the previous key's data, with `Fetching`, until the new data arrives |
| `ui.QueryEnabled(false)` | on | no fetch starts: not on use, not on invalidation, not on `Refetch` |

Desktop data is mostly local, so a failure is shown at once unless `QueryRetry` says otherwise.

## Rules

- Components that use one key share one entry and one fetch. While a fetch is in flight,
  no other starts for the key.
- A fetch starts when a component starts using a key whose data is missing or stale, when an
  invalidated key has an enabled user, and on `Refetch`. A component that keeps using the
  key does not start a fetch on each render.
- A result reaches the UI thread through `ctx.Post`.
- The fetch's context is canceled when the last component using the key goes away, when a
  refetch of the key starts while the fetch runs, and when the window closes. A canceled
  fetch's result is dropped, even if the fetch returns data anyway, so the cache holds only
  the data of fetches that finished while they were wanted. The next use fetches again.
- A component re-renders when its key's entry changes.
- A panic in the fetch becomes the query's error.
- A key holds one type. A `Query` and a `SetQueryData` for the same key with different types
  panic with the key's name.

## Invalidation and optimistic updates

`ui.InvalidateQueries` marks the keys that a function matches as stale. An invalidated key
with a user is fetched again at once; the others are fetched when a component next uses them.
Call it from a handler, after a change that makes some data out of date:

```go
ui.Button("Save").OnClick(func() {
	save(draft)
	ui.InvalidateQueries(ctx, func(key string) bool {
		return strings.HasPrefix(key, "commit:")
	})
})
```

`ui.SetQueryData` writes a value, as if a fetch had returned it, so the data is fresh from
now. Use it for an optimistic update, which shows the change before the work behind it has
finished. A fetch in flight is not canceled, so its result replaces the value when it arrives.

```go
ui.SetQueryData(ctx, "commit:"+hash, updated)
```

Both calls run on the UI thread, such as in a handler.

## Keys that change

A component that renders a different key shows the new key's data, and the old key's fetch
is canceled when nothing else uses that key. Without `QueryKeepPrevious`, a key with no data
shows no data and `Loading` is true until its data arrives. With it, the previous key's data
stays on screen, with `Fetching` set. A component should make its `Query` calls in the same
order on each render, as it would with hooks, so that `QueryKeepPrevious` can tell the previous
render's queries apart.

## Testing

A test reads a query's result as it reads any state. Wait for the data with
`u.WaitFor`, since the fetch runs off the UI thread, and move the clock with `u.Advance` for
retries and cache times:

```go
var res ui.QueryResult[string]
u := uitest.New(t, func(ctx *ui.Context) ui.Element {
	res = ui.Query(ctx, "greeting", func(context.Context) (string, error) {
		return "hello", nil
	})
	return ui.Div()
})
u.WaitFor("the data", func() bool { return res.Data == "hello" })
```

`u.Close()` closes the window, so a test can check that the fetches are canceled.
