package icons

// More icons in the same 24×24, 2-unit-stroke style.
var (
	Tag = newIcon("tag", func(b *builder) {
		b.M(3, 3).L(11, 3).L(21, 13).L(13, 21).L(3, 11).Z()
		b.Dot(7.5, 7.5, 1.5)
	})

	Hash = newIcon("hash", func(b *builder) {
		b.Line(4, 9, 20, 9).Line(4, 15, 20, 15).Line(10, 3, 8, 21).Line(16, 3, 14, 21)
	})

	Pin = newIcon("pin", func(b *builder) {
		b.Line(12, 17, 12, 22)
		b.M(9, 3).L(15, 3)
		b.M(10, 3).L(10, 10).L(6, 14).L(6, 17).L(18, 17).L(18, 14).L(14, 10).L(14, 3)
	})

	Pencil = newIcon("pencil", func(b *builder) {
		b.M(17, 3).L(21, 7).L(8, 20).L(3, 21).L(4, 16).Z()
		b.Line(14, 6, 18, 10)
	})

	// The caption buttons of a custom title bar (see ui.TitleBarButtons).
	Minimize = newIcon("minimize", func(b *builder) {
		b.Line(4, 12, 20, 12)
	})

	Maximize = newIcon("maximize", func(b *builder) {
		b.RoundRect(4, 4, 16, 16, 1)
	})

	Restore = newIcon("restore", func(b *builder) {
		b.M(8, 4).L(20, 4).L(20, 16)
		b.RoundRect(4, 8, 12, 12, 1)
	})

	Copy = newIcon("copy", func(b *builder) {
		b.RoundRect(9, 9, 12, 12, 2)
		b.M(5, 15).L(4, 15).Arc(4, 13, 2, 90, 180).L(2, 4).Arc(4, 4, 2, 180, 270).L(13, 2).Arc(13, 4, 2, 270, 360).L(15, 5)
	})

	Link = newIcon("link", func(b *builder) {
		b.M(9, 8).L(7, 8).Arc(7, 12, 4, -90, -270).L(9, 16)
		b.M(15, 8).L(17, 8).Arc(17, 12, 4, -90, 90).L(15, 16)
		b.Line(8, 12, 16, 12)
	})

	Lock = newIcon("lock", func(b *builder) {
		b.RoundRect(4, 11, 16, 10, 2)
		b.M(8, 11).L(8, 7).Arc(12, 7, 4, 180, 360).L(16, 11)
	})

	Eye = newIcon("eye", func(b *builder) {
		b.M(2, 12).Arc(12, 15.64, 10.64, -160, -20)
		b.M(2, 12).Arc(12, 8.36, 10.64, 160, 20)
		b.Circle(12, 12, 3)
	})

	Menu = newIcon("menu", func(b *builder) {
		b.Line(4, 6, 20, 6).Line(4, 12, 20, 12).Line(4, 18, 20, 18)
	})

	Home = newIcon("home", func(b *builder) {
		b.M(3, 10).L(12, 3).L(21, 10)
		b.M(5, 9).L(5, 21).L(19, 21).L(19, 9)
		b.M(10, 21).L(10, 15).L(14, 15).L(14, 21)
	})

	Info = newIcon("info", func(b *builder) {
		b.Circle(12, 12, 10).Line(12, 16, 12, 11.5).Dot(12, 8, 1.2)
	})

	Alert = newIcon("alert", func(b *builder) {
		b.M(12, 3).L(22, 20).L(2, 20).Z()
		b.Line(12, 9, 12, 13.5).Dot(12, 16.8, 1.2)
	})

	Minus = newIcon("minus", func(b *builder) { b.Line(5, 12, 19, 12) })

	ChevronLeft = newIcon("chevron-left", func(b *builder) { b.M(15, 18).L(9, 12).L(15, 6) })

	ChevronUp = newIcon("chevron-up", func(b *builder) { b.M(18, 15).L(12, 9).L(6, 15) })

	ArrowLeft = newIcon("arrow-left", func(b *builder) {
		b.Line(19, 12, 5, 12).M(12, 19).L(5, 12).L(12, 5)
	})

	ArrowRight = newIcon("arrow-right", func(b *builder) {
		b.Line(5, 12, 19, 12).M(12, 5).L(19, 12).L(12, 19)
	})

	Refresh = newIcon("refresh", func(b *builder) {
		// A circle open at the top, ending in an arrowhead pointing clockwise.
		b.M(19.5, 9.3).Arc(12, 12, 8, -20, 280)
		b.M(10, 1.4).L(13.4, 4.1).L(10.4, 7.2)
	})

	File = newIcon("file", func(b *builder) {
		b.M(14, 2).L(5, 2).L(5, 22).L(19, 22).L(19, 7).Z()
		b.M(14, 2).L(14, 7).L(19, 7)
	})

	Heart = newIcon("heart", func(b *builder) {
		b.M(12, 20).L(4.8, 12.2).Arc(8.5, 9, 4.5, 139, 315)
		b.Arc(15.5, 9, 4.5, 225, 401).L(12, 20)
	})

	Bookmark = newIcon("bookmark", func(b *builder) {
		b.M(6, 3).L(18, 3).L(18, 21).L(12, 16).L(6, 21).Z()
	})

	Archive = newIcon("archive", func(b *builder) {
		b.RoundRect(3, 3, 18, 5, 1)
		b.M(5, 8).L(5, 21).L(19, 21).L(19, 8)
		b.Line(10, 12, 14, 12)
	})

	Download = newIcon("download", func(b *builder) {
		b.Line(12, 3, 12, 15).M(7, 10).L(12, 15).L(17, 10)
		b.M(4, 17).L(4, 21).L(20, 21).L(20, 17)
	})

	Upload = newIcon("upload", func(b *builder) {
		b.Line(12, 15, 12, 3).M(7, 8).L(12, 3).L(17, 8)
		b.M(4, 17).L(4, 21).L(20, 21).L(20, 17)
	})

	// Pause and Play are the two looks of a control that stops and starts something
	// that moves by itself, such as a carousel's AutoPlay.
	Pause = newIcon("pause", func(b *builder) {
		b.RoundRect(6, 4, 4, 16, 1)
		b.RoundRect(14, 4, 4, 16, 1)
	})

	Play = newIcon("play", func(b *builder) {
		b.M(6, 4).L(20, 12).L(6, 20).Z()
	})
)

// All lists every built-in icon, for discovery (print their names) and for
// icon pickers.
var All = []*Icon{
	ArrowLeft, ArrowRight, ArrowUpRight, Activity, Alert, Archive, Bell, Bookmark, Box,
	Calendar, Check, ChevronDown, ChevronLeft, ChevronRight, ChevronUp, Circle, Clock, Copy, Maximize, Minimize, Restore,
	Dashboard, Download, Eye, File, Filter, Folder, GitBranch, Hash, Heart, Home, Inbox, Info,
	Layers, Link, Lock, Menu, Minus, Moon, MoreHorizontal, Pause, Pencil, Pin, Play, Plus, Refresh, Search,
	Settings, Star, StarFilled, Sun, Tag, Trash, Upload, Users, X, Zap,
}
