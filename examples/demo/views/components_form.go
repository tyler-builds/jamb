package views

import (
	"fmt"
	"math"

	"github.com/tyler-builds/jamb/ui"
)

// formControlsSection shows each form control in its states, and a notification
// preferences form that uses several of them together. The values are kept in
// the section, so only the section re-renders when one changes.
func formControlsSection() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		// The states of the samples.
		on := ui.Local(ctx, "form.on", true)
		off := ui.Local(ctx, "form.off", false)
		mixed := ui.Local(ctx, "form.mixed", ui.CheckMixed)
		sync := ui.Local(ctx, "form.sync", false)
		freq := ui.Local(ctx, "form.freq", "daily")
		channel := ui.Local(ctx, "form.channel", "email")
		picked := ui.Local(ctx, "form.picked", "red")
		volume := ui.Local(ctx, "form.volume", 40.0)
		quiet := ui.Local(ctx, "form.quiet", 22)
		// The preferences form.
		email := ui.Local(ctx, "form.email", true)
		text := ui.Local(ctx, "form.text", false)
		push := ui.Local(ctx, "form.push", true)
		all := ui.Local(ctx, "form.all", ui.CheckMixed)
		seen := ui.Local(ctx, "form.seen", ui.CheckMixed)
		fromHour := ui.Local(ctx, "form.from", 22)
		toHour := ui.Local(ctx, "form.to", 7)
		prefFreq := ui.Local(ctx, "form.prefFreq", "daily")
		prefChannel := ui.Local(ctx, "form.prefChannel", "email")
		prefVolume := ui.Local(ctx, "form.prefVolume", 60.0)

		// The "all" box is the parent of the three alerts. A click on it sets all of
		// them; a change to one of them sets it to on, off or mixed.
		if all.Peek() != seen.Peek() {
			set := all.Peek() == ui.CheckOn
			email.Set(set)
			text.Set(set)
			push.Set(set)
		}
		all.Set(checkAll(email.Get(), text.Get(), push.Get()))
		seen.Set(all.Peek())

		samples := []ui.Element{
			formSample(t, "Checkbox", ui.Checkbox("Email alerts", on).Key("form-checkbox"), onOff(on.Get())),
			formSample(t, "Checkbox, disabled", ui.Checkbox("Unavailable", off).Disabled(true), onOff(off.Get())),
			formSample(t, "Checkbox, mixed", ui.CheckboxTri("Some channels", mixed), checkName(mixed.Get())),
			formSample(t, "Switch", ui.Switch("Sync settings", sync), onOff(sync.Get())),
			formSample(t, "Switch, disabled", ui.Switch("Unavailable sync", off).Disabled(true), onOff(off.Get())),
			formSample(t, "Radio group", radioSample(freq), freq.Get()),
			formSample(t, "Select", selectSample(channel, false), channelName(channel.Get())),
			formSample(t, "Select, disabled", selectSample(picked, true), channelName(picked.Get())),
			formSample(t, "Slider", ui.Slider(volume, 0, 100, 5).Label("Volume").Format(percentOf).Width(200), percentOf(volume.Get())),
			formSample(t, "Slider, disabled", ui.Slider(volume, 0, 100, 5).Label("Volume, disabled").Format(percentOf).Width(200).Disabled(true), percentOf(volume.Get())),
			formSample(t, "Number field", ui.NumberField(quiet, 0, 23, 1).Label("Sample quiet hours").Buttons(), fmt.Sprintf("%02d:00", quiet.Get())),
			formSample(t, "Number field, disabled", ui.NumberField(quiet, 0, 23, 1).Label("Sample quiet hours, disabled").Disabled(true), fmt.Sprintf("%02d:00", quiet.Get())),
		}
		row := ui.Div().Row().Wrap().Align(ui.Start).Gap(22)
		for _, s := range samples {
			row.Child(s)
		}

		return Card(t).Gap(14).
			Child(SectionTitle(t, "Form controls")).
			Child(ui.Text("Each control in its normal and disabled states, with the value it holds. Type a number out of range or text that is not a number, then press Enter or leave the field.").
				Size(13).Color(t.TextMuted)).
			Child(row).
			Child(ui.Divider()).
			Child(SectionTitle(t, "Notification preferences")).
			Child(ui.Div().Row().Wrap().Align(ui.Start).Gap(28).
				Child(ui.Div().Gap(10).Width(ui.Px(240)).
					Child(ui.Text("Send alerts by").Size(13).Medium().Color(t.TextMuted)).
					Child(ui.CheckboxTri("All alerts", all).Key("form-all")).
					Child(ui.Div().Gap(8).MarginX(24).
						Child(ui.Checkbox("Email", email)).
						Child(ui.Checkbox("Text message", text)).
						Child(ui.Checkbox("Push notification", push)))).
				Child(ui.Div().Gap(10).Width(ui.Px(240)).
					Child(ui.Text("How often").Size(13).Medium().Color(t.TextMuted)).
					Child(ui.RadioGroup(prefFreq,
						ui.Option[string]{Value: "immediate", Label: "As they happen"},
						ui.Option[string]{Value: "daily", Label: "Daily digest"},
						ui.Option[string]{Value: "weekly", Label: "Weekly digest", Disabled: true},
					).Label("How often"))).
				Child(ui.Div().Gap(10).Width(ui.Px(240)).
					Child(ui.Text("Preferred channel").Size(13).Medium().Color(t.TextMuted)).
					Child(ui.Select(prefChannel,
						ui.Option[string]{Value: "email", Label: "Email"},
						ui.Option[string]{Value: "sms", Label: "Text message"},
						ui.Option[string]{Value: "phone", Label: "Phone call"},
					).Label("Preferred channel").Width(240)).
					Child(ui.Text("Alert volume").Size(13).Medium().Color(t.TextMuted)).
					Child(ui.Slider(prefVolume, 0, 100, 5).Label("Alert volume").Format(percentOf).Width(240))).
				Child(ui.Div().Gap(10).
					Child(ui.Text("Quiet hours").Size(13).Medium().Color(t.TextMuted)).
					Child(ui.Div().Row().Align(ui.Center).Gap(10).
						Child(ui.NumberField(fromHour, 0, 23, 1).Label("Quiet hours from").Buttons()).
						Child(ui.Text("to").Color(t.TextMuted)).
						Child(ui.NumberField(toHour, 0, 23, 1).Label("Quiet hours to").Buttons()))).
				Child(ui.Div().Gap(4).Width(ui.Px(240)).
					Child(ui.Textf("Sends by %s, %s, at volume %s.",
						channelsOf(email.Get(), text.Get(), push.Get()), prefFreq.Get(), percentOf(prefVolume.Get())).Size(13)).
					Child(ui.Textf("Quiet from %02d:00 to %02d:00.", fromHour.Get(), toHour.Get()).Size(13)).
					Child(ui.Text(fmt.Sprintf("Preferred channel: %s.", channelName(prefChannel.Get()))).Size(13))))
	})
}

// checkAll is the state of the parent box of three alerts: on when all are on,
// off when none is, and mixed otherwise.
func checkAll(a, b, c bool) ui.CheckState {
	switch {
	case a && b && c:
		return ui.CheckOn
	case !a && !b && !c:
		return ui.CheckOff
	}
	return ui.CheckMixed
}

// formSample is one control with a readout of its value, under it.
func formSample(t *ui.Theme, label string, control ui.Element, readout string) ui.Element {
	return ui.Div().Gap(8).Width(ui.Px(240)).
		Child(ui.Text(label).Size(11).Color(t.TextSubtle)).
		Child(control).
		Child(ui.Text("Value: " + readout).Size(12).Color(t.TextMuted))
}

// radioSample is a radio group with a disabled option.
func radioSample(value *ui.Signal[string]) ui.Element {
	return ui.RadioGroup(value,
		ui.Option[string]{Value: "immediate", Label: "As they happen"},
		ui.Option[string]{Value: "daily", Label: "Daily digest"},
		ui.Option[string]{Value: "weekly", Label: "Weekly digest (unavailable)", Disabled: true},
	).Label("Frequency")
}

// selectSample is a select of channels, or a disabled one.
func selectSample(value *ui.Signal[string], disabled bool) ui.Element {
	return ui.Select(value,
		ui.Option[string]{Value: "email", Label: "Email"},
		ui.Option[string]{Value: "sms", Label: "Text message"},
		ui.Option[string]{Value: "phone", Label: "Phone call", Disabled: true},
		ui.Option[string]{Value: "red", Label: "Red"},
	).Label(channelLabel(disabled)).Placeholder("Choose a channel").Width(240).Disabled(disabled)
}

// channelName is the text of a channel's value, such as "Text message".
func channelName(v string) string {
	switch v {
	case "email":
		return "Email"
	case "sms":
		return "Text message"
	case "phone":
		return "Phone call"
	case "red":
		return "Red"
	}
	return "none"
}

// checkName is the text of a check box's state.
func checkName(s ui.CheckState) string {
	switch s {
	case ui.CheckOn:
		return "on"
	case ui.CheckMixed:
		return "mixed"
	}
	return "off"
}

// onOff is the text of a boolean state.
func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// percentOf is a value from 0 to 100 shown as a percent.
func percentOf(v float64) string { return fmt.Sprintf("%d%%", int(math.Round(v))) }

// channelsOf lists the alerts that are on, such as "email and push".
func channelsOf(email, text, push bool) string {
	var names []string
	if email {
		names = append(names, "email")
	}
	if text {
		names = append(names, "text message")
	}
	if push {
		names = append(names, "push")
	}
	switch len(names) {
	case 0:
		return "no channel"
	case 1:
		return names[0]
	}
	return fmt.Sprint(names[0], " and ", names[1])
}

// channelLabel names the channel select: the disabled one has its own name, so
// that the two are told apart.
func channelLabel(disabled bool) string {
	if disabled {
		return "Channel, disabled"
	}
	return "Channel"
}
