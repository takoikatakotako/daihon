package render

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"sync"

	// Photos from the web increasingly arrive as WebP, and the decoder is in
	// x/image, which is already a dependency for fonts. Registering it here
	// saves a conversion step for every background.
	_ "golang.org/x/image/webp"

	"github.com/takoikatakotako/daihon/internal/parser"
	"github.com/takoikatakotako/daihon/internal/render/text"
)

// assets decodes image files once and hands out the results. A missing file is
// not an error: it becomes a labelled placeholder (see placeholder.go).
type assets struct {
	project *parser.Project
	face    *text.Face

	mu     sync.Mutex
	cache  map[string]image.Image
	missed map[string]bool
}

func newAssets(p *parser.Project, face *text.Face) *assets {
	return &assets{
		project: p,
		face:    face,
		cache:   map[string]image.Image{},
		missed:  map[string]bool{},
	}
}

// load returns the decoded image for a project-relative path, or nil when the
// file is missing or unreadable. Callers substitute a placeholder.
func (a *assets) load(rel string) image.Image {
	if rel == "" {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if img, ok := a.cache[rel]; ok {
		return img
	}

	f, err := os.Open(a.project.Abs(rel))
	if err != nil {
		a.cache[rel] = nil
		a.missed[rel] = true
		return nil
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		a.cache[rel] = nil
		a.missed[rel] = true
		return nil
	}
	a.cache[rel] = img
	return img
}

// missing lists the assets that had to be replaced by placeholders.
func (a *assets) missing() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, 0, len(a.missed))
	for p := range a.missed {
		out = append(out, p)
	}
	return out
}
