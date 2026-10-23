package icons

// The icons below are not in Lucide, or keep the drawing and name of the set
// that came before Lucide.

var (
	// Maximize, Minimize and Restore are the caption buttons of a custom title
	// bar (see ui.TitleBar). Their drawings are Windows-style boxes, which Lucide
	// does not have; Lucide's maximize and minimize are LucideMaximize and
	// LucideMinimize.
	Maximize = &Icon{Name: "window-maximize", src: `<rect x="4" y="4" width="16" height="16" rx="1"/>`}
	Minimize = &Icon{Name: "window-minimize", src: `<path d="M4 12h16"/>`}
	Restore  = &Icon{Name: "window-restore", src: `<path d="M8 4h12v12"/><rect x="4" y="8" width="12" height="12" rx="1"/>`}

	// StarFilled is Star with its inside filled. Starred and pinned states use
	// it, so the state shows in the shape and not only in the color.
	StarFilled = &Icon{Name: "star-filled", src: `<g fill="currentColor">` + starSrc + `</g>`}
)

// The names of the icons before Lucide that Lucide names differently. Each is
// the same icon as the one it points to.
var (
	Alert          = TriangleAlert
	Dashboard      = LayoutDashboard
	Filter         = Funnel
	Home           = House
	MoreHorizontal = Ellipsis
	Refresh        = RotateCw
)

// All lists every icon: the Lucide set and then the icons of this file. Calling
// All keeps every icon in the binary, so an app that uses a few icons should
// name them instead.
func All() []*Icon {
	return append(lucideIcons(), Maximize, Minimize, Restore, StarFilled)
}
