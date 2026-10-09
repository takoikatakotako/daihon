package text

import "os"

// fallbacks are system fonts with Japanese coverage, tried in order when the
// project does not name a font.
//
// DESIGN.md §6.3 requires an explicit font path so that output does not vary
// with the machine, and that remains the rule: `daihon check` warns when no font
// is configured. But refusing to draw at all would break the promise that a
// project with no assets still builds, so a best-effort fallback exists.
var fallbacks = []string{
	"/System/Library/Fonts/ヒラギノ角ゴシック W6.ttc",
	"/System/Library/Fonts/ヒラギノ角ゴシック W3.ttc",
	"/System/Library/Fonts/Hiragino Sans GB.ttc",
	"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
	"/Library/Fonts/Arial Unicode.ttf",
	"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/truetype/fonts-japanese-gothic.ttf",
}

// Fallback returns a usable system font path, or "" if none was found.
func Fallback() string {
	for _, p := range fallbacks {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}
