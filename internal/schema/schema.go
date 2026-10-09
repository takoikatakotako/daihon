// Package schema defines the movie.yml document model.
package schema

import "gopkg.in/yaml.v3"

// Movie is a parsed movie.yml (with imports already merged).
type Movie struct {
	Version    int                   `yaml:"version"`
	Meta       Meta                  `yaml:"meta"`
	Imports    []string              `yaml:"imports"`
	Video      Video                 `yaml:"video"`
	Export     Export                `yaml:"export"`
	Fonts      map[string]string     `yaml:"fonts"`
	Audio      Audio                 `yaml:"audio"`
	Subtitle   Subtitle              `yaml:"subtitle"`
	Characters map[string]*Character `yaml:"characters"`
	Scenes     []Scene               `yaml:"scenes"`
}

type Meta struct {
	Title  string `yaml:"title"`
	Output string `yaml:"output"`
	Credit string `yaml:"credit"`
}

type Video struct {
	Width      int        `yaml:"width"`
	Height     int        `yaml:"height"`
	FPS        int        `yaml:"fps"`
	Background Background `yaml:"background"`
}

// Export controls the encoded file, not the layout.
//
// Sizes and positions in movie.yml are authored against video.width/height.
// Export.Height renders the frame smaller and scales everything with it, the
// same way a draft does, so a 720p export looks like the 1080p one shrunk.
//
//	export: { height: 720, crf: 28, audio_bitrate: 64k }
type Export struct {
	Height       int    `yaml:"height"`        // 0 = video.height
	CRF          int    `yaml:"crf"`           // libx264 constant quality, 0 = 18. Higher is smaller
	AudioBitrate string `yaml:"audio_bitrate"` // AAC, e.g. "64k"; "" = 192k
}

// Background is an image with optional treatment applied when it is drawn.
//
// A photo behind two figures and a speech bubble competes with them; dimming
// and blurring it pushes it back. The treatment belongs to the image, not to
// the movie: a slide set as a scene background must stay readable, so it is
// written as a plain path and gets none of it.
//
//	background: assets/bg/office.png            # plain
//	background: { src: assets/bg/office.png, dim: 0.35, blur: 6 }
type Background struct {
	Src  string  `yaml:"src"`
	Dim  float64 `yaml:"dim"`  // 0 (untouched) .. 1 (black)
	Blur float64 `yaml:"blur"` // gaussian radius in px at video.height
}

// UnmarshalYAML accepts either a bare path or the mapping form.
func (b *Background) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		b.Src = node.Value
		return nil
	}
	type plain Background
	var v plain
	if err := node.Decode(&v); err != nil {
		return err
	}
	*b = Background(v)
	return nil
}

// IsZero reports whether no background was given.
func (b Background) IsZero() bool { return b.Src == "" }

type Audio struct {
	PauseBetweenLinesMS int  `yaml:"pause_between_lines_ms"`
	BGM                 *BGM `yaml:"bgm"`
}

type BGM struct {
	Src       string  `yaml:"src"`
	Volume    float64 `yaml:"volume"`
	FadeOutMS int     `yaml:"fade_out_ms"`
}

type Subtitle struct {
	Font            string `yaml:"font"`
	Mode            string `yaml:"mode"` // bubble | telop | none
	Size            int    `yaml:"size"`
	Color           string `yaml:"color"`
	Stroke          Stroke `yaml:"stroke"`
	MaxCharsPerLine int    `yaml:"max_chars_per_line"`
}

type Stroke struct {
	Color string `yaml:"color"`
	Width int    `yaml:"width"`
}

type Character struct {
	Name              string            `yaml:"name"`
	Voice             Voice             `yaml:"voice"`
	Stand             Stand             `yaml:"stand"`
	Expressions       map[string]string `yaml:"expressions"`
	DefaultExpression string            `yaml:"default_expression"`
	Lipsync           *Lipsync          `yaml:"lipsync"`
	Bubble            Bubble            `yaml:"bubble"`
}

type Voice struct {
	Engine    string  `yaml:"engine"`
	SpeakerID int     `yaml:"speaker_id"`
	Speed     float64 `yaml:"speed"`
	Pitch     float64 `yaml:"pitch"`
}

type Stand struct {
	Position string  `yaml:"position"` // left | center | right | none (voice only)
	Scale    float64 `yaml:"scale"`
	Offset   Offset  `yaml:"offset"`
}

// StandNone is the position of a character who is heard but never seen: a
// narrator. It is kept off stage for the whole movie, so no figure, mouth or
// placeholder is drawn and centred images get the full width.
const StandNone = "none"

type Offset struct {
	X int `yaml:"x"`
	Y int `yaml:"y"`
}

type Lipsync struct {
	Close string `yaml:"close"`
	Open  string `yaml:"open"`
}

type Bubble struct {
	Color string `yaml:"color"`
}

type Scene struct {
	ID         string       `yaml:"id"`
	Background Background   `yaml:"background"`
	Script     []ScriptItem `yaml:"script"`
}

// ApplyDefaults fills in values that the author may omit.
func (m *Movie) ApplyDefaults() {
	if m.Video.Width == 0 {
		m.Video.Width = 1920
	}
	if m.Video.Height == 0 {
		m.Video.Height = 1080
	}
	if m.Video.FPS == 0 {
		m.Video.FPS = 30
	}
	if m.Audio.PauseBetweenLinesMS == 0 {
		m.Audio.PauseBetweenLinesMS = 400
	}
	if m.Subtitle.Mode == "" {
		m.Subtitle.Mode = "bubble"
	}
	if m.Subtitle.Size == 0 {
		m.Subtitle.Size = 48
	}
	if m.Subtitle.Color == "" {
		m.Subtitle.Color = "#ffffff"
	}
	if m.Subtitle.MaxCharsPerLine == 0 {
		m.Subtitle.MaxCharsPerLine = 22
	}
	if m.Meta.Output == "" {
		m.Meta.Output = "out/movie.mp4"
	}
	for _, c := range m.Characters {
		if c.Voice.Engine == "" {
			c.Voice.Engine = "voicevox"
		}
		if c.Voice.Speed == 0 {
			c.Voice.Speed = 1.0
		}
		if c.Stand.Position == "" {
			c.Stand.Position = "right"
		}
		if c.Stand.Scale == 0 {
			c.Stand.Scale = 1.0
		}
		if c.DefaultExpression == "" {
			c.DefaultExpression = "normal"
		}
		if c.Bubble.Color == "" {
			c.Bubble.Color = "#4a5568"
		}
	}
}
