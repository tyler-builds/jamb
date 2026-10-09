// Package assets embeds the demo's images.
package assets

import _ "embed"

//go:embed avatar.png
var AvatarPNG []byte

//go:embed banner.png
var BannerPNG []byte
