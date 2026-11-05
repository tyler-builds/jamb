package ui

import (
	"reflect"
	"testing"
	"time"
)

// TestParseCaptionLayout checks the reading of GNOME's button-layout syntax.
func TestParseCaptionLayout(t *testing.T) {
	all := []captionButton{captionMinimize, captionMaximize, captionClose}
	for _, c := range []struct {
		in         string
		start, end []captionButton
	}{
		{"", nil, all},
		{"appmenu:close", nil, []captionButton{captionClose}},
		{"icon:minimize,maximize,close", nil, all},
		{"close,minimize,maximize:", []captionButton{captionClose, captionMinimize, captionMaximize}, nil},
		{"close:appmenu", []captionButton{captionClose}, nil},
		{"appmenu:", nil, nil},
		{"minimize, close", nil, []captionButton{captionMinimize, captionClose}},
	} {
		start, end := parseCaptionLayout(c.in)
		if !reflect.DeepEqual(start, c.start) || !reflect.DeepEqual(end, c.end) {
			t.Errorf("parseCaptionLayout(%q) = %v, %v; want %v, %v", c.in, start, end, c.start, c.end)
		}
	}
}

// TestRoundCaptionButtons checks GNOME's buttons: 24 px circles, as many as the
// layout names, on the side it names.
func TestRoundCaptionButtons(t *testing.T) {
	rt := NewRuntime(func(*Context) Element {
		return Div().Column().Child(TitleBar().Key("bar").Width(Percent(100)).Child(Div().Key("item").Size(Px(20), Px(20))))
	}, nil, nil)
	rt.SetViewport(600, 300, 1)
	rt.SetSystemSettings(SystemSettings{RoundCaptionButtons: true, CaptionButtons: "appmenu:minimize,close"})
	rt.Frame(time.Now())
	if _, ok := rt.Find("Maximize"); ok {
		t.Error("the layout names no maximize button, but the bar has one")
	}
	cl, ok := rt.Find("Close")
	if !ok || cl.W != 24 || cl.H != 24 {
		t.Fatalf("close button %+v (found %v); want a 24 px circle", cl, ok)
	}
	if cl.X+cl.W != 600-6 {
		t.Errorf("close button ends at %v; want 6 px from the window's edge", cl.X+cl.W)
	}
	if a := rt.hostWindowAreaAt(cl.X+12, cl.Y+12); a != WindowAreaClose {
		t.Errorf("the close button's middle is area %v; want WindowAreaClose", a)
	}

	rt.SetSystemSettings(SystemSettings{RoundCaptionButtons: true, CaptionButtons: "close:"})
	rt.Frame(time.Now())
	cl, _ = rt.Find("Close")
	item, _ := rt.Find("item")
	if cl.X != 6 || item.X < cl.X+cl.W {
		t.Errorf("close button %+v, item %+v; want the button at the start, before the item", cl, item)
	}
	if _, ok := rt.Find("Minimize"); ok {
		t.Error("the layout names no minimize button, but the bar has one")
	}
}

// TestSystemCaptionHidesButtons checks that a title bar draws no caption buttons
// where the desktop's frame has them, as KDE's does, and draws them again when
// it does not.
func TestSystemCaptionHidesButtons(t *testing.T) {
	rt := NewRuntime(func(*Context) Element {
		return Div().Column().Child(TitleBar().Key("bar").Width(Percent(100)))
	}, nil, nil)
	rt.SetViewport(600, 300, 1)
	rt.SetSystemSettings(SystemSettings{RoundCaptionButtons: true, CaptionButtons: "close:minimize"})
	(*runtimeHost)(rt).SetSystemCaption(true)
	rt.Frame(time.Now())
	for _, k := range []string{"Close", "Minimize", "Maximize"} {
		if _, ok := rt.Find(k); ok {
			t.Errorf("the desktop's frame has the buttons, but the bar has a %s button", k)
		}
	}
	(*runtimeHost)(rt).SetSystemCaption(false)
	rt.Frame(time.Now())
	if _, ok := rt.Find("Close"); !ok {
		t.Error("the bar has no close button after the desktop's frame went away")
	}
}
