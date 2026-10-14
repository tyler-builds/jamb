package text

import "golang.org/x/image/font/sfnt"

// FaceName returns the family name that the font file of the face drawn for st declares in its
// own name table, such as "Consolas" for the monospace family on Windows. It is the name a
// platform knows the face by, unlike the family key in st. It returns "" when the face has no
// name or cannot be read.
func (sys *System) FaceName(st Style) string {
	f := sys.reg.Font(st.Family, st.Weight, st.Italic)
	if f == nil || f.sf == nil {
		return ""
	}
	var buf sfnt.Buffer
	name, err := f.sf.Name(&buf, sfnt.NameIDFamily)
	if err != nil {
		return ""
	}
	return name
}
