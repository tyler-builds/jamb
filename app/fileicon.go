package app

import "image"

// FileIcon returns the icon that Windows shows for the file at path, size pixels
// square, as File Explorer and Task Manager show it: the file's own icon when it
// holds one (an executable, a library or an .ico file), at that size, and otherwise
// the icon of its type, scaled to size. Pass the size in pixels of the display: a
// 16-pixel icon on a display at 150 percent is 24 pixels square. Show it with
// ui.ImageFromImage.
//
//	img, err := app.FileIcon(`C:\Windows\explorer.exe`, 32)
//	if err == nil {
//		icon := ui.Image(ui.ImageFromImage(img)).Size(ui.Px(16), ui.Px(16))
//	}
//
// It reads the file, so call it off the UI thread for many files, and keep the
// images by path. It is safe from any goroutine. A file Windows has no icon for,
// a path that does not exist and a size of zero give an error. It is a Windows
// feature, and gives an error on other platforms.
func FileIcon(path string, size int) (image.Image, error) { return fileIcon(path, size) }
