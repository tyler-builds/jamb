// Package ease has easing functions for animation. Each maps progress t in
// [0, 1] to eased progress in [0, 1], with 0 mapping to 0 and 1 to 1 (except
// Bounce, which returns to 0). It imports nothing from jamb, so any package
// can use it:
//
//	ui.Animate("open", ui.Animation{Duration: 300 * time.Millisecond, Easing: ease.OutCubic}, ...)
package ease

// Linear moves at a constant speed.
func Linear(t float32) float32 { return t }

// In starts slowly and speeds up (quadratic).
func In(t float32) float32 { return t * t }

// Out starts quickly and slows down (quadratic).
func Out(t float32) float32 { return 1 - (1-t)*(1-t) }

// InOut starts and ends slowly (quadratic).
func InOut(t float32) float32 {
	if t < 0.5 {
		return 2 * t * t
	}
	u := -2*t + 2
	return 1 - u*u/2
}

// InCubic starts slowly and speeds up more sharply than In.
func InCubic(t float32) float32 { return t * t * t }

// OutCubic starts quickly and slows down gently; a good default for motion
// that should feel responsive.
func OutCubic(t float32) float32 {
	u := 1 - t
	return 1 - u*u*u
}

// InOutCubic starts and ends slowly, with a faster middle.
func InOutCubic(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	u := -2*t + 2
	return 1 - u*u*u/2
}

// Bounce plays f there and back: it runs f from 0 to 1 over the first half
// of the progress and from 1 back to 0 over the second, so the value ends
// where it started. Bounce(Out) rises quickly and settles back.
func Bounce(f func(float32) float32) func(float32) float32 {
	return func(t float32) float32 {
		if t < 0.5 {
			return f(t * 2)
		}
		return f((1 - t) * 2)
	}
}

// Reverse plays f backwards: Reverse(Out) starts slowly and ends quickly,
// which is how an exit should feel when Out is how the entrance feels.
func Reverse(f func(float32) float32) func(float32) float32 {
	return func(t float32) float32 { return 1 - f(1-t) }
}
