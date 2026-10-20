// Package ease has easing functions for animation. Each maps progress t in
// [0, 1] to eased progress, with 0 mapping to 0 and 1 to 1 (except Bounce,
// which returns to 0). The values stay in [0, 1], except a Spring with a
// bounce, which overshoots 1 on the way. It imports nothing from jamb, so any
// package can use it:
//
//	ui.Animate("open", ui.Animation{Duration: 300 * time.Millisecond, Easing: ease.OutCubic}, ...)
//
// Spring is an easing function, not a spring engine: it gives a timed motion
// (Enter, Exit, Animate, a keyframe) the shape of a spring, and still ends at
// the animation's Duration. For motion that follows changing targets and keeps
// its velocity, see ui.Spring.
package ease

import "math"

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

// Spring is the step response of a damped spring: it rises like a mass
// pulled to 1 and, with a bounce, overshoots and swings back before it
// settles. bounce is clamped to [0, 0.9]: 0 is critically damped and never
// overshoots, and larger values swing more (the damping ratio is 1-bounce).
// The spring completes one natural period over the progress, and what is left
// of its motion fades out towards the end, so Spring(b)(1) is exactly 1.
//
//	ui.Div().Enter(ui.Grow(0.9), ui.Animation{Duration: 400 * time.Millisecond, Easing: ease.Spring(0.3)})
func Spring(bounce float32) func(float32) float32 {
	zeta := 1 - float64(min(max(bounce, 0), 0.9))
	const omega = 2 * math.Pi
	return func(t float32) float32 {
		if t <= 0 {
			return 0
		}
		if t >= 1 {
			return 1
		}
		x := float64(t)
		var rest float64 // how far the spring is from 1 at x
		if zeta >= 1 {
			rest = math.Exp(-omega*x) * (1 + omega*x)
		} else {
			wd := omega * math.Sqrt(1-zeta*zeta)
			rest = math.Exp(-zeta*omega*x) * (math.Cos(wd*x) + zeta*omega/wd*math.Sin(wd*x))
		}
		// The remaining displacement fades out linearly, so the curve reaches 1
		// at t = 1 without a jump.
		return float32(1 - rest*(1-x))
	}
}
