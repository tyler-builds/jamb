package views

import (
	"image"
	"strings"
	"sync"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// iconPixels is the size the programs' icons are read at: twice the 16 pixels
// they are drawn at, so they stay sharp on a display at 200 percent.
const iconPixels = 32

// iconCache holds the icon of each executable, read once, by one goroutine, so
// the first sample's few hundred processes do not hold up the table. A read that
// finishes bumps ver, which renders the table again.
type iconCache struct {
	load  func(path string, size int) (image.Image, error)
	ver   *ui.Signal[int]
	once  sync.Once
	queue chan string

	mu      sync.Mutex
	srcs    map[string]*ui.ImageSource // nil for an executable that has no icon
	pending map[string]bool
}

func newIconCache(load func(string, int) (image.Image, error)) *iconCache {
	return &iconCache{load: load, ver: ui.NewSignal(0), queue: make(chan string, 1024),
		srcs: map[string]*ui.ImageSource{}, pending: map[string]bool{}}
}

// get returns the icon of the executable at path, or nil while it is being read
// or when it has none. Call it in a render, which it subscribes to the reads.
func (c *iconCache) get(path string) *ui.ImageSource {
	if c == nil || c.load == nil || path == "" {
		return nil
	}
	c.ver.Get()
	key := strings.ToLower(path)
	c.mu.Lock()
	defer c.mu.Unlock()
	if src, ok := c.srcs[key]; ok {
		return src
	}
	if !c.pending[key] {
		c.pending[key] = true
		c.once.Do(func() { go c.work() })
		select {
		case c.queue <- path:
		default:
			delete(c.pending, key) // asked again on a later render
		}
	}
	return nil
}

func (c *iconCache) work() {
	for path := range c.queue {
		var src *ui.ImageSource
		if img, err := c.load(path, iconPixels); err == nil {
			src = ui.ImageFromImage(img)
		}
		key := strings.ToLower(path)
		c.mu.Lock()
		c.srcs[key] = src
		delete(c.pending, key)
		c.mu.Unlock()
		c.ver.Update(func(v int) int { return v + 1 })
	}
}

// nameCell is a row's name with its program's icon, as Task Manager shows it. A
// process without an executable, and one whose icon is still being read, shows a
// plain window; the group of Windows' processes shows a monitor.
func (a *App) nameCell(t *ui.Theme, r row) ui.Element {
	var mark ui.Element
	switch src := a.icons.get(r.Path); {
	case src != nil:
		mark = ui.Image(src).Size(ui.Px(16), ui.Px(16))
	case r.key == groupWindows:
		mark = ui.Icon(icons.MonitorCog).Size(16).Color(t.TextMuted)
	default:
		mark = ui.Icon(icons.AppWindow).Size(16).Color(t.TextMuted)
	}
	return ui.Div().Row().Size(ui.Full, ui.Full).Align(ui.Center).Gap(8).MinWidth(ui.Px(0)).
		Child(ui.Div().Shrink(0).Child(mark)).
		Child(ui.Div().Shrink(1).MinWidth(ui.Px(0)).Child(ui.Text(groupName(r)).Truncate()))
}
