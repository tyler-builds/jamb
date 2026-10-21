package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// treeBenchApp is a Tree of roots, each with children, with every root open:
// the window shows roots*(children+1) rows. The tree is focused.
func treeBenchApp(roots, children int) (*Runtime, *FocusHandle) {
	nodes := make([]TreeNode, roots)
	open := make([]string, roots)
	for i := range nodes {
		key := fmt.Sprintf("root-%d", i)
		open[i] = key
		nodes[i] = TreeNode{Key: key, Label: fmt.Sprintf("Folder %d", i), Children: func() []TreeNode {
			kids := make([]TreeNode, children)
			for j := range kids {
				kids[j] = TreeNode{Key: fmt.Sprintf("%s/item-%d", key, j), Label: fmt.Sprintf("File %d", j)}
			}
			return kids
		}}
	}
	selected := NewSignal("")
	expanded := NewSignal(open)
	h := NewFocusHandle()
	rt := NewRuntime(func(ctx *Context) Element {
		return Div().Size(Full, Full).Child(Tree(nodes, selected).Expanded(expanded).FocusHandle(h).Key("tree").Grow(1))
	}, DarkTheme(), benchText)
	rt.SetViewport(400, 800, 1)
	now := time.Unix(1_000_000, 0)
	rt.Frame(now)
	rt.Frame(now.Add(time.Second))
	h.Focus()
	return rt, h
}

// BenchmarkTreeExpanded is a Tree with every row open, of about 1,100 and about
// 10,100 rows. "frame" is a full rebuild and frame with the tree in view. "arrow-down"
// moves the selection one row down with the keyboard, and runs a frame.
func BenchmarkTreeExpanded(b *testing.B) {
	for _, c := range []struct {
		name            string
		roots, children int
	}{
		{"rows=1100", 100, 10},
		{"rows=10100", 100, 100},
	} {
		b.Run(c.name, func(b *testing.B) {
			rt, _ := treeBenchApp(c.roots, c.children)
			now := time.Unix(1_000_000, 0)
			b.Run("frame", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					rt.hostInvalidateAll()
					now = now.Add(16 * time.Millisecond)
					rt.Frame(now)
				}
				b.ReportMetric(float64(rt.Stats().Nodes), "nodes")
			})
			b.Run("arrow-down", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					rt.KeyDown(input.KeyDown, 0, false)
					rt.KeyUp(input.KeyDown, 0)
					now = now.Add(16 * time.Millisecond)
					rt.Frame(now)
				}
			})
		})
	}
}
