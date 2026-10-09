// Package render turns a timeline into an mp4.
//
// Frames are composed in Go and piped to ffmpeg as raw video (DESIGN.md §6.1):
// ffmpeg only encodes and mixes. Every visual decision is ordinary Go code that
// can be stepped through and unit tested.
package render

import (
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/takoikatakotako/daihon/internal/parser"
	"github.com/takoikatakotako/daihon/internal/render/text"
	"github.com/takoikatakotako/daihon/internal/timeline"
)

// creditMS is how long the credit card stays on screen at the end.
const creditMS = 3000

// Options controls one render.
type Options struct {
	// Draft halves the resolution and frame rate for quick iteration. The tool
	// has no browser preview, so this is where iteration speed comes from.
	Draft  bool
	Output string
}

// Stats describes what a render produced.
type Stats struct {
	Frames  int
	Width   int
	Height  int
	FPS     int
	Output  string
	Missing []string
	Encoder string
}

type Renderer struct {
	Project  *parser.Project
	Timeline *timeline.Timeline
	Opts     Options
}

func (r *Renderer) Render(ctx context.Context) (*Stats, error) {
	tl := r.Timeline
	w, h, fps := tl.Video.Width, tl.Video.Height, tl.Video.FPS
	if eh := r.Project.Movie.Export.Height; eh > 0 && eh != h {
		w, h = w*eh/h, eh
		w, h = w-w%2, h-h%2 // H.264 needs even dimensions
	}
	if r.Opts.Draft {
		w, h, fps = w/2, h/2, max(fps/2, 1)
		w, h = w-w%2, h-h%2 // H.264 needs even dimensions
	}

	fontPath, err := r.resolveFont()
	if err != nil {
		return nil, err
	}
	size := r.subtitleSize(h)
	face, err := text.Load(fontPath, float64(size))
	if err != nil {
		return nil, err
	}
	defer face.Close()

	small, err := text.Load(fontPath, float64(size)*0.6)
	if err != nil {
		return nil, err
	}
	defer small.Close()

	out := r.Opts.Output
	if out == "" {
		out = r.Project.Movie.Meta.Output
	}
	out = r.Project.Abs(out)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return nil, err
	}

	// Always leave a little audio past the last line so the final frame is not
	// clipped; extend it further when a credit card follows.
	tail := 1000
	if tl.Credit != "" {
		tail += creditMS
	}
	voicePath := filepath.Join(r.Project.Root, cacheDirName, "voice-mix.wav")
	if err := os.MkdirAll(filepath.Dir(voicePath), 0o755); err != nil {
		return nil, err
	}
	if err := mixVoice(tl, voicePath, tail); err != nil {
		return nil, err
	}

	// The credit card is part of the video, so the BGM fade is measured from
	// the end of the whole thing. Fading at the end of the last line instead
	// leaves the credits in dead silence, which reads as a broken file.
	totalMS := tl.DurationMS
	if tl.Credit != "" {
		totalMS += creditMS
	}

	enc := pickEncoder(ctx, r.Opts.Draft)
	e, err := startEncoder(ctx, encodeOptions{
		Width: w, Height: h, FPS: fps,
		VoicePath:    voicePath,
		BGM:          tl.BGM,
		BGMPath:      bgmPath(r.Project, tl),
		DurationMS:   totalMS,
		VideoEncoder: enc,
		CRF:          r.Project.Movie.Export.CRF,
		AudioBitrate: r.Project.Movie.Export.AudioBitrate,
		Output:       out,
	})
	if err != nil {
		return nil, err
	}

	c := &composer{
		project: r.Project,
		tl:      tl,
		assets:  newAssets(r.Project, face),
		face:    face,
		small:   small,
		size:    size,
		mouths:  map[string]*image.RGBA{},
		w:       w, h: h,
	}

	frames := totalMS * fps / 1000
	for i := range frames {
		select {
		case <-ctx.Done():
			e.finish()
			return nil, ctx.Err()
		default:
		}
		t := i * 1000 / fps
		img := c.frame(t)
		if err := e.write(img.Pix); err != nil {
			return nil, fmt.Errorf("フレーム %d の書き込みに失敗しました: %w", i, err)
		}
	}
	if err := e.finish(); err != nil {
		return nil, err
	}

	return &Stats{
		Frames: frames, Width: w, Height: h, FPS: fps,
		Output: out, Missing: c.assets.missing(), Encoder: enc,
	}, nil
}

// cacheDirName mirrors the CLI's cache location.
const cacheDirName = ".daihon-cache"

func (r *Renderer) subtitleSize(h int) int {
	size := r.Project.Movie.Subtitle.Size
	if size == 0 {
		size = 48
	}
	// Scale with the frame so that a draft render looks like the real thing.
	return max(size*h/r.Timeline.Video.Height, 8)
}

// resolveFont prefers the path named in movie.yml; DESIGN.md §6.3 wants output
// to be independent of the machine. A system fallback keeps a project with no
// font configured renderable, and `daihon check` warns when that happens.
func (r *Renderer) resolveFont() (string, error) {
	m := r.Project.Movie
	name := m.Subtitle.Font
	if name == "" {
		name = "default"
	}
	if p, ok := m.Fonts[name]; ok && p != "" {
		abs := r.Project.Abs(p)
		if _, err := os.Stat(abs); err == nil {
			return abs, nil
		}
	}
	if p := text.Fallback(); p != "" {
		return p, nil
	}
	return "", fmt.Errorf("使用できるフォントがありません。movie.yml の fonts にパスを指定してください")
}

func bgmPath(p *parser.Project, tl *timeline.Timeline) string {
	if tl.BGM == nil || tl.BGM.Src == "" {
		return ""
	}
	abs := p.Abs(tl.BGM.Src)
	if _, err := os.Stat(abs); err != nil {
		return "" // missing BGM is not worth failing a render over
	}
	return abs
}

// pickEncoder trades speed against text quality depending on the build.
//
// Subtitles are hard-edged text on flat colour, which is the worst case for a
// hardware encoder: h264_videotoolbox ignores an explicit bitrate here and
// settles near 2-3 Mbps at 1080p30, visibly softening the edges. libx264 has a
// real constant-quality mode, so it is used for final renders. Drafts are for
// checking timing and layout, where speed matters more than crisp glyphs.
func pickEncoder(ctx context.Context, draft bool) string {
	b, err := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-encoders").Output()
	if err != nil {
		return "libx264"
	}
	list := string(b)
	has := func(name string) bool { return strings.Contains(list, " "+name+" ") }

	if draft && has("h264_videotoolbox") {
		return "h264_videotoolbox"
	}
	if has("libx264") {
		return "libx264"
	}
	if has("h264_videotoolbox") {
		return "h264_videotoolbox"
	}
	return "libx264"
}
