//go:build linux

package linux

import (
	"errors"
	"fmt"
	"image"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	xdraw "golang.org/x/image/draw"
)

// Tray icons are StatusNotifierItems (the KDE and freedesktop.org protocol that
// KDE Plasma, the GNOME AppIndicator extension, and most other panels host),
// and their menus are exported with the com.canonical.dbusmenu protocol.
//
// Each icon has a session bus connection of its own, with the well-known name
// org.kde.StatusNotifierItem-<pid>-<id>. The watcher drops an item when its
// name goes, so Remove closes the connection, and the item leaves the panel
// even on watchers that ignore UnregisterStatusNotifierItem (there is none in
// the protocol). The objects live at the paths the protocol expects of a named
// item, which a connection of its own makes unambiguous.

const (
	sniIface       = "org.kde.StatusNotifierItem"
	sniPath        = dbus.ObjectPath("/StatusNotifierItem")
	watcherName    = "org.kde.StatusNotifierWatcher"
	watcherPath    = dbus.ObjectPath("/StatusNotifierWatcher")
	menuIface      = "com.canonical.dbusmenu"
	menuPath       = dbus.ObjectPath("/MenuBar")
	noMenuPath     = dbus.ObjectPath("/NO_DBUSMENU")
	propsIface     = "org.freedesktop.DBus.Properties"
	fallbackIcon   = "application-x-executable"
	doubleClickGap = 400 * time.Millisecond
	// menuWait bounds how long a menu request from the panel waits for the UI
	// thread to build the menu; the menu as last built is served after it.
	menuWait = time.Second
	// otherIDs is the first dbusmenu ID of the items that are never chosen
	// (separators and submenus), above the IDs of the items that are.
	otherIDs = 1 << 24
)

// iconSizes are the sizes of the pixmaps sent for an icon. Panels pick the one
// nearest their size, and scale it when none fits.
var iconSizes = []int{16, 22, 24, 32, 48, 64}

// TrayOptions describes a tray icon.
type TrayOptions struct {
	// ID is unique in the process; it names the icon on the bus.
	ID uint32
	// AppName is the app's name, which panels show in their settings and use
	// to remember the user's choice to show or hide the icon.
	AppName string
	Tooltip string
	// Icon is the image of the icon; nil is a generic program icon.
	Icon image.Image
	// Click runs for a click (Activate).
	Click func()
	// DoubleClick runs for a second click soon after the first.
	DoubleClick func()
	// Menu returns the menu; nil means the icon has none.
	Menu func() []MenuItem
	// Chosen runs with the ID of the menu item the user chose.
	Chosen func(id uint32)
}

// MenuItem is an item of a tray menu. ID is the command that Chosen receives.
// A separator, and an item with a Submenu, is never chosen.
type MenuItem struct {
	ID        uint32
	Label     string
	Disabled  bool
	Checked   bool
	Separator bool
	Submenu   []MenuItem
}

// TrayIcon is a tray icon. Its methods run on the UI thread; the bus calls the
// objects it exports from other goroutines, which read the state under mu.
type TrayIcon struct {
	opts      TrayOptions // UI thread
	conn      *dbus.Conn  // the icon's own connection while shown; UI thread
	lastClick time.Time   // UI thread

	mu       sync.Mutex
	tooltip  string
	pixmaps  []pixmap
	menu     *menuNode // the root, ID 0
	revision uint32
}

// NewTrayIcon returns an icon that is not shown yet; see Show.
func NewTrayIcon(o TrayOptions) *TrayIcon {
	t := &TrayIcon{opts: o, tooltip: o.Tooltip}
	t.pixmaps = pixmaps(o.Icon)
	return t
}

// ErrNoTrayHost is returned by Show when the desktop shows no tray icons: no
// StatusNotifierWatcher runs on the session bus. GNOME has one only with the
// AppIndicator extension.
var ErrNoTrayHost = errors.New("the desktop shows no tray icons (no StatusNotifierWatcher on the session bus; GNOME needs the AppIndicator extension)")

// Show adds the icon to the panel. It is an error when there is no session bus
// or no tray, and the icon is not shown then.
func (t *TrayIcon) Show() error {
	if t.conn != nil {
		return nil
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("no session bus: %w", err)
	}
	var has bool
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, watcherName).Store(&has); err != nil || !has {
		conn.Close()
		return ErrNoTrayHost
	}
	t.rebuildMenu()
	if err := t.export(conn); err != nil {
		conn.Close()
		return err
	}
	name := fmt.Sprintf("org.kde.StatusNotifierItem-%d-%d", os.Getpid(), t.opts.ID)
	if reply, err := conn.RequestName(name, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		conn.Close()
		if err == nil {
			err = fmt.Errorf("the name %s is taken", name)
		}
		return fmt.Errorf("tray icon: %w", err)
	}
	if err := register(conn, name); err != nil {
		conn.Close()
		return err
	}
	t.conn = conn
	t.watchWatcher(conn, name)
	return nil
}

// register adds the item to the watcher.
func register(conn *dbus.Conn, name string) error {
	call := conn.Object(watcherName, watcherPath).Call(watcherName+".RegisterStatusNotifierItem", 0, name)
	if call.Err != nil {
		return fmt.Errorf("the StatusNotifierWatcher refused the icon: %w", call.Err)
	}
	return nil
}

// watchWatcher registers the item again when a new watcher starts, as when the
// panel restarts, so the icon stays shown. It ends when conn closes.
func (t *TrayIcon) watchWatcher(conn *dbus.Conn, name string) {
	conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, watcherName),
	)
	ch := make(chan *dbus.Signal, 4)
	conn.Signal(ch)
	go func() {
		for sig := range ch {
			if sig.Name != "org.freedesktop.DBus.NameOwnerChanged" || len(sig.Body) < 3 {
				continue
			}
			if n, _ := sig.Body[0].(string); n != watcherName {
				continue
			}
			if owner, _ := sig.Body[2].(string); owner != "" {
				if err := register(conn, name); err != nil {
					fmt.Fprintln(os.Stderr, "jamb: tray icon:", err)
				}
			}
		}
	}()
}

// export puts the item and its menu on conn.
func (t *TrayIcon) export(conn *dbus.Conn) error {
	item := &sniItem{t: t}
	menu := &dbusMenu{t: t}
	for _, e := range []struct {
		v     any
		path  dbus.ObjectPath
		iface string
	}{
		{item, sniPath, sniIface},
		{sniProps{t}, sniPath, propsIface},
		{introspect.Introspectable(sniXML), sniPath, "org.freedesktop.DBus.Introspectable"},
		{menu, menuPath, menuIface},
		{menuProps{}, menuPath, propsIface},
		{introspect.Introspectable(menuXML), menuPath, "org.freedesktop.DBus.Introspectable"},
	} {
		if err := conn.Export(e.v, e.path, e.iface); err != nil {
			return fmt.Errorf("tray icon: %w", err)
		}
	}
	return nil
}

// SetTooltip sets the tooltip, which is also the item's title.
func (t *TrayIcon) SetTooltip(s string) {
	t.mu.Lock()
	t.tooltip = s
	t.mu.Unlock()
	t.emit("NewToolTip")
	t.emit("NewTitle")
}

// SetIcon sets the image of the icon; nil is a generic program icon.
func (t *TrayIcon) SetIcon(img image.Image) error {
	p := pixmaps(img)
	t.mu.Lock()
	t.pixmaps = p
	t.mu.Unlock()
	t.emit("NewIcon")
	return nil
}

// Remove takes the icon out of the panel. Show adds it again.
func (t *TrayIcon) Remove() {
	if t.conn != nil {
		t.conn.Close()
		t.conn = nil
	}
}

// emit sends a signal of the item, when it is shown.
func (t *TrayIcon) emit(member string) {
	if t.conn != nil {
		t.conn.Emit(sniPath, sniIface+"."+member)
	}
}

// activate runs on the UI thread for a click. A click soon after another is a
// double click, which runs DoubleClick instead, as Windows reports a double click
// after the click of its first press.
func (t *TrayIcon) activate() {
	now := time.Now()
	if t.opts.DoubleClick != nil && !t.lastClick.IsZero() && now.Sub(t.lastClick) < doubleClickGap {
		t.lastClick = time.Time{}
		t.opts.DoubleClick()
		return
	}
	t.lastClick = now
	if t.opts.Click != nil {
		t.opts.Click()
	}
}

// rebuildMenu builds the menu again, on the UI thread, and reports whether it
// changed. A change takes a new revision, and panels are told of it.
func (t *TrayIcon) rebuildMenu() bool {
	root := &menuNode{children: []*menuNode{}, submenu: true}
	if t.opts.Menu != nil {
		next := int32(otherIDs)
		root.children = menuNodes(t.opts.Menu(), &next)
	}
	t.mu.Lock()
	changed := t.menu == nil || !t.menu.equal(root)
	if changed {
		t.menu = root
		t.revision++
	}
	rev := t.revision
	t.mu.Unlock()
	if changed && t.conn != nil {
		t.conn.Emit(menuPath, menuIface+".LayoutUpdated", rev, int32(0))
	}
	return changed
}

// rebuildFromBus builds the menu on the UI thread for a call from the bus, and
// reports whether it changed. It gives up after menuWait, so a busy or ended UI
// thread leaves the panel with the menu as last built.
func (t *TrayIcon) rebuildFromBus() bool {
	ch := make(chan bool, 1)
	Post(func() { ch <- t.rebuildMenu() })
	select {
	case changed := <-ch:
		return changed
	case <-time.After(menuWait):
		return false
	}
}

// menuNode is an item of a menu as the bus sees it.
type menuNode struct {
	id        int32
	label     string
	disabled  bool
	checked   bool
	separator bool
	submenu   bool
	chosen    bool // the item is chosen with its ID (Chosen)
	children  []*menuNode
}

// menuNodes converts items. Items that are chosen keep their IDs; the others take
// the next ID after *next.
func menuNodes(items []MenuItem, next *int32) []*menuNode {
	out := make([]*menuNode, 0, len(items))
	for _, it := range items {
		n := &menuNode{label: it.Label, disabled: it.Disabled, checked: it.Checked, separator: it.Separator}
		switch {
		case it.Separator || len(it.Submenu) > 0:
			*next++
			n.id = *next
			if !it.Separator {
				n.submenu = true
				n.children = menuNodes(it.Submenu, next)
			}
		default:
			n.id = int32(it.ID)
			n.chosen = true
		}
		out = append(out, n)
	}
	return out
}

func (n *menuNode) equal(o *menuNode) bool {
	if n.id != o.id || n.label != o.label || n.disabled != o.disabled || n.checked != o.checked ||
		n.separator != o.separator || n.submenu != o.submenu || n.chosen != o.chosen || len(n.children) != len(o.children) {
		return false
	}
	for i, c := range n.children {
		if !c.equal(o.children[i]) {
			return false
		}
	}
	return true
}

func (n *menuNode) find(id int32) *menuNode {
	if n.id == id {
		return n
	}
	for _, c := range n.children {
		if f := c.find(id); f != nil {
			return f
		}
	}
	return nil
}

// props returns the dbusmenu properties of the item, all of them when names is
// empty. Properties with their default values are left out, as the protocol
// asks.
func (n *menuNode) props(names []string) map[string]dbus.Variant {
	p := map[string]dbus.Variant{}
	switch {
	case n.separator:
		p["type"] = dbus.MakeVariant("separator")
	case n.id != 0:
		// An underscore marks the access key; a literal one is doubled.
		p["label"] = dbus.MakeVariant(strings.ReplaceAll(n.label, "_", "__"))
	}
	if n.disabled {
		p["enabled"] = dbus.MakeVariant(false)
	}
	if n.checked {
		p["toggle-type"] = dbus.MakeVariant("checkmark")
		p["toggle-state"] = dbus.MakeVariant(int32(1))
	}
	if n.submenu {
		p["children-display"] = dbus.MakeVariant("submenu")
	}
	if len(names) > 0 {
		for k := range p {
			if !slices.Contains(names, k) {
				delete(p, k)
			}
		}
	}
	return p
}

// menuLayout is the (ia{sv}av) layout of an item and its children.
type menuLayout struct {
	ID       int32
	Props    map[string]dbus.Variant
	Children []dbus.Variant
}

// layout returns the item with depth levels of children; -1 is all of them.
func (n *menuNode) layout(depth int32, names []string) menuLayout {
	l := menuLayout{ID: n.id, Props: n.props(names), Children: []dbus.Variant{}}
	if depth != 0 {
		for _, c := range n.children {
			l.Children = append(l.Children, dbus.MakeVariant(c.layout(depth-1, names)))
		}
	}
	return l
}

// menuItemProps is an item's ID and properties, (ia{sv}).
type menuItemProps struct {
	ID    int32
	Props map[string]dbus.Variant
}

// menuEvent is an event of EventGroup, (isvu).
type menuEvent struct {
	ID        int32
	EventID   string
	Data      dbus.Variant
	Timestamp uint32
}

// dbusMenu is the com.canonical.dbusmenu object of an icon.
type dbusMenu struct{ t *TrayIcon }

func (m *dbusMenu) GetLayout(parent, depth int32, names []string) (uint32, menuLayout, *dbus.Error) {
	m.t.mu.Lock()
	defer m.t.mu.Unlock()
	n := m.t.menu.find(parent)
	if n == nil {
		return 0, menuLayout{}, dbus.MakeFailedError(fmt.Errorf("no menu item %d", parent))
	}
	return m.t.revision, n.layout(depth, names), nil
}

func (m *dbusMenu) GetGroupProperties(ids []int32, names []string) ([]menuItemProps, *dbus.Error) {
	m.t.mu.Lock()
	defer m.t.mu.Unlock()
	var out []menuItemProps
	var walk func(n *menuNode)
	walk = func(n *menuNode) {
		if len(ids) == 0 || slices.Contains(ids, n.id) {
			out = append(out, menuItemProps{n.id, n.props(names)})
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(m.t.menu)
	return out, nil
}

func (m *dbusMenu) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	m.t.mu.Lock()
	defer m.t.mu.Unlock()
	if n := m.t.menu.find(id); n != nil {
		if v, ok := n.props(nil)[name]; ok {
			return v, nil
		}
	}
	return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("no property %q of menu item %d", name, id))
}

func (m *dbusMenu) Event(id int32, eventID string, data dbus.Variant, timestamp uint32) *dbus.Error {
	if eventID != "clicked" {
		return nil
	}
	m.t.mu.Lock()
	n := m.t.menu.find(id)
	chosen := n != nil && n.chosen && !n.disabled
	m.t.mu.Unlock()
	if chosen {
		t := m.t
		Post(func() {
			if t.opts.Chosen != nil {
				t.opts.Chosen(uint32(id))
			}
			// The choice may have changed the menu, such as a check mark; panels
			// that do not ask before they open it again see the change.
			t.rebuildMenu()
		})
	}
	return nil
}

func (m *dbusMenu) EventGroup(events []menuEvent) ([]int32, *dbus.Error) {
	for _, e := range events {
		m.Event(e.ID, e.EventID, e.Data, e.Timestamp)
	}
	return []int32{}, nil
}

// AboutToShow builds the menu again before it opens, so it shows the app's state.
func (m *dbusMenu) AboutToShow(id int32) (bool, *dbus.Error) {
	if id != 0 {
		return false, nil // the whole menu was built when the root opened
	}
	return m.t.rebuildFromBus(), nil
}

func (m *dbusMenu) AboutToShowGroup(ids []int32) ([]int32, []int32, *dbus.Error) {
	if slices.Contains(ids, 0) && m.t.rebuildFromBus() {
		return []int32{0}, []int32{}, nil
	}
	return []int32{}, []int32{}, nil
}

// menuProps are the properties of the dbusmenu object.
type menuProps struct{}

func (menuProps) all() map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"Version":       dbus.MakeVariant(uint32(3)),
		"TextDirection": dbus.MakeVariant("ltr"),
		"Status":        dbus.MakeVariant("normal"),
		"IconThemePath": dbus.MakeVariant([]string{}),
	}
}

func (p menuProps) Get(iface, name string) (dbus.Variant, *dbus.Error) {
	return getProp(iface, menuIface, name, p.all())
}

func (p menuProps) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface != menuIface && iface != "" {
		return nil, dbus.MakeFailedError(fmt.Errorf("no interface %s", iface))
	}
	return p.all(), nil
}

func (menuProps) Set(string, string, dbus.Variant) *dbus.Error { return readOnly() }

// sniItem is the org.kde.StatusNotifierItem object of an icon.
type sniItem struct{ t *TrayIcon }

func (s *sniItem) Activate(x, y int32) *dbus.Error {
	Post(s.t.activate)
	return nil
}

// ContextMenu is called only by panels that do not show the dbusmenu
// themselves, and the item has no menu of its own to show.
func (s *sniItem) ContextMenu(x, y int32) *dbus.Error                 { return nil }
func (s *sniItem) SecondaryActivate(x, y int32) *dbus.Error           { return nil }
func (s *sniItem) Scroll(delta int32, orientation string) *dbus.Error { return nil }

// ProvideXdgActivationToken receives the token that would let the app raise a
// window for the click that follows. The Wayland adapter does not use
// xdg-activation yet, so it is dropped.
func (s *sniItem) ProvideXdgActivationToken(token string) *dbus.Error { return nil }

// pixmap is an image of the icon, (iiay): width, height, and ARGB32 pixels in
// network byte order, not premultiplied.
type pixmap struct {
	W, H int32
	Data []byte
}

// pixmaps returns the images of img at iconSizes, or none for nil.
func pixmaps(img image.Image) []pixmap {
	if img == nil || img.Bounds().Empty() {
		return []pixmap{}
	}
	out := make([]pixmap, 0, len(iconSizes))
	for _, s := range iconSizes {
		dst := image.NewNRGBA(image.Rect(0, 0, s, s))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Src, nil)
		data := make([]byte, 0, s*s*4)
		for i := 0; i < len(dst.Pix); i += 4 {
			data = append(data, dst.Pix[i+3], dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2])
		}
		out = append(out, pixmap{int32(s), int32(s), data})
	}
	return out
}

// toolTip is the (sa(iiay)ss) tooltip: icon name, icon, title and description.
type toolTip struct {
	Icon    string
	Pixmaps []pixmap
	Title   string
	Text    string
}

// sniProps are the properties of the item.
type sniProps struct{ t *TrayIcon }

func (p sniProps) all() map[string]dbus.Variant {
	t := p.t
	t.mu.Lock()
	tooltip, pix := t.tooltip, t.pixmaps
	t.mu.Unlock()
	id := t.opts.AppName
	if id == "" {
		id = appID()
	}
	title := tooltip
	if title == "" {
		title = id
	}
	iconName := ""
	if len(pix) == 0 {
		iconName = fallbackIcon
	}
	menu := noMenuPath
	if t.opts.Menu != nil {
		menu = menuPath
	}
	return map[string]dbus.Variant{
		"Category":            dbus.MakeVariant("ApplicationStatus"),
		"Id":                  dbus.MakeVariant(id),
		"Title":               dbus.MakeVariant(title),
		"Status":              dbus.MakeVariant("Active"),
		"WindowId":            dbus.MakeVariant(int32(0)),
		"IconThemePath":       dbus.MakeVariant(""),
		"IconName":            dbus.MakeVariant(iconName),
		"IconPixmap":          dbus.MakeVariant(pix),
		"OverlayIconName":     dbus.MakeVariant(""),
		"OverlayIconPixmap":   dbus.MakeVariant([]pixmap{}),
		"AttentionIconName":   dbus.MakeVariant(""),
		"AttentionIconPixmap": dbus.MakeVariant([]pixmap{}),
		"AttentionMovieName":  dbus.MakeVariant(""),
		"ToolTip":             dbus.MakeVariant(toolTip{Pixmaps: []pixmap{}, Title: tooltip}),
		// With a menu and no click action, a click opens the menu.
		"ItemIsMenu": dbus.MakeVariant(t.opts.Menu != nil && t.opts.Click == nil && t.opts.DoubleClick == nil),
		"Menu":       dbus.MakeVariant(menu),
	}
}

func (p sniProps) Get(iface, name string) (dbus.Variant, *dbus.Error) {
	return getProp(iface, sniIface, name, p.all())
}

func (p sniProps) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface != sniIface && iface != "" {
		return nil, dbus.MakeFailedError(fmt.Errorf("no interface %s", iface))
	}
	return p.all(), nil
}

func (sniProps) Set(string, string, dbus.Variant) *dbus.Error { return readOnly() }

func getProp(iface, want, name string, all map[string]dbus.Variant) (dbus.Variant, *dbus.Error) {
	if iface != want && iface != "" {
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("no interface %s", iface))
	}
	v, ok := all[name]
	if !ok {
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("no property %s", name))
	}
	return v, nil
}

func readOnly() *dbus.Error {
	return dbus.NewError("org.freedesktop.DBus.Error.PropertyReadOnly", []any{"the properties are read only"})
}

const sniXML = `<node>
 <interface name="org.kde.StatusNotifierItem">
  <property name="Category" type="s" access="read"/>
  <property name="Id" type="s" access="read"/>
  <property name="Title" type="s" access="read"/>
  <property name="Status" type="s" access="read"/>
  <property name="WindowId" type="i" access="read"/>
  <property name="IconThemePath" type="s" access="read"/>
  <property name="IconName" type="s" access="read"/>
  <property name="IconPixmap" type="a(iiay)" access="read"/>
  <property name="OverlayIconName" type="s" access="read"/>
  <property name="OverlayIconPixmap" type="a(iiay)" access="read"/>
  <property name="AttentionIconName" type="s" access="read"/>
  <property name="AttentionIconPixmap" type="a(iiay)" access="read"/>
  <property name="AttentionMovieName" type="s" access="read"/>
  <property name="ToolTip" type="(sa(iiay)ss)" access="read"/>
  <property name="ItemIsMenu" type="b" access="read"/>
  <property name="Menu" type="o" access="read"/>
  <method name="ContextMenu"><arg name="x" type="i" direction="in"/><arg name="y" type="i" direction="in"/></method>
  <method name="Activate"><arg name="x" type="i" direction="in"/><arg name="y" type="i" direction="in"/></method>
  <method name="SecondaryActivate"><arg name="x" type="i" direction="in"/><arg name="y" type="i" direction="in"/></method>
  <method name="Scroll"><arg name="delta" type="i" direction="in"/><arg name="orientation" type="s" direction="in"/></method>
  <method name="ProvideXdgActivationToken"><arg name="token" type="s" direction="in"/></method>
  <signal name="NewTitle"/>
  <signal name="NewIcon"/>
  <signal name="NewAttentionIcon"/>
  <signal name="NewOverlayIcon"/>
  <signal name="NewToolTip"/>
  <signal name="NewStatus"><arg name="status" type="s"/></signal>
 </interface>
 <interface name="org.freedesktop.DBus.Properties">
  <method name="Get"><arg name="interface" type="s" direction="in"/><arg name="name" type="s" direction="in"/><arg name="value" type="v" direction="out"/></method>
  <method name="GetAll"><arg name="interface" type="s" direction="in"/><arg name="properties" type="a{sv}" direction="out"/></method>
  <method name="Set"><arg name="interface" type="s" direction="in"/><arg name="name" type="s" direction="in"/><arg name="value" type="v" direction="in"/></method>
 </interface>
 <interface name="org.freedesktop.DBus.Introspectable">
  <method name="Introspect"><arg name="xml" type="s" direction="out"/></method>
 </interface>
</node>`

const menuXML = `<node>
 <interface name="com.canonical.dbusmenu">
  <property name="Version" type="u" access="read"/>
  <property name="TextDirection" type="s" access="read"/>
  <property name="Status" type="s" access="read"/>
  <property name="IconThemePath" type="as" access="read"/>
  <method name="GetLayout">
   <arg name="parentId" type="i" direction="in"/><arg name="recursionDepth" type="i" direction="in"/>
   <arg name="propertyNames" type="as" direction="in"/>
   <arg name="revision" type="u" direction="out"/><arg name="layout" type="(ia{sv}av)" direction="out"/>
  </method>
  <method name="GetGroupProperties">
   <arg name="ids" type="ai" direction="in"/><arg name="propertyNames" type="as" direction="in"/>
   <arg name="properties" type="a(ia{sv})" direction="out"/>
  </method>
  <method name="GetProperty">
   <arg name="id" type="i" direction="in"/><arg name="name" type="s" direction="in"/><arg name="value" type="v" direction="out"/>
  </method>
  <method name="Event">
   <arg name="id" type="i" direction="in"/><arg name="eventId" type="s" direction="in"/>
   <arg name="data" type="v" direction="in"/><arg name="timestamp" type="u" direction="in"/>
  </method>
  <method name="EventGroup">
   <arg name="events" type="a(isvu)" direction="in"/><arg name="idErrors" type="ai" direction="out"/>
  </method>
  <method name="AboutToShow">
   <arg name="id" type="i" direction="in"/><arg name="needUpdate" type="b" direction="out"/>
  </method>
  <method name="AboutToShowGroup">
   <arg name="ids" type="ai" direction="in"/><arg name="updatesNeeded" type="ai" direction="out"/>
   <arg name="idErrors" type="ai" direction="out"/>
  </method>
  <signal name="ItemsPropertiesUpdated"><arg name="updatedProps" type="a(ia{sv})"/><arg name="removedProps" type="a(ias)"/></signal>
  <signal name="LayoutUpdated"><arg name="revision" type="u"/><arg name="parent" type="i"/></signal>
  <signal name="ItemActivationRequested"><arg name="id" type="i"/><arg name="timestamp" type="u"/></signal>
 </interface>
 <interface name="org.freedesktop.DBus.Properties">
  <method name="Get"><arg name="interface" type="s" direction="in"/><arg name="name" type="s" direction="in"/><arg name="value" type="v" direction="out"/></method>
  <method name="GetAll"><arg name="interface" type="s" direction="in"/><arg name="properties" type="a{sv}" direction="out"/></method>
  <method name="Set"><arg name="interface" type="s" direction="in"/><arg name="name" type="s" direction="in"/><arg name="value" type="v" direction="in"/></method>
 </interface>
 <interface name="org.freedesktop.DBus.Introspectable">
  <method name="Introspect"><arg name="xml" type="s" direction="out"/></method>
 </interface>
</node>`
