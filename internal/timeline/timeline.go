// Package timeline turns a parsed script plus synthesised audio into the
// intermediate representation the renderer consumes.
//
// This is where the central rule of the tool lives: the author never writes a
// timestamp. A line occupies exactly as long as its audio turned out to be, and
// everything else is anchored to the cursor that advances past it.
package timeline

import "github.com/takoikatakotako/daihon/internal/wav"

// Timeline is the intermediate representation. Times are milliseconds, never
// frame numbers, so changing fps does not invalidate it.
type Timeline struct {
	Version    int     `json:"version"`
	Title      string  `json:"title,omitempty"`
	Video      Video   `json:"video"`
	DurationMS int     `json:"duration_ms"`
	BGM        *BGM    `json:"bgm,omitempty"`
	Credit     string  `json:"credit,omitempty"`
	Tracks     []Track `json:"tracks"`
}

type Video struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	FPS    int `json:"fps"`
}

type BGM struct {
	Src       string  `json:"src"`
	Volume    float64 `json:"volume"`
	FadeOutMS int     `json:"fade_out_ms,omitempty"`
}

// Track types.
const (
	TrackBackground = "background"
	TrackImage      = "image"
	TrackCharacter  = "character"
	TrackSubtitle   = "subtitle"
	TrackVoice      = "voice"
	TrackSE         = "se"
)

type Track struct {
	Type   string  `json:"type"`
	ID     string  `json:"id,omitempty"`
	Events []Event `json:"events"`
}

// Event is deliberately one wide struct rather than a type per track: the JSON
// stays readable, and a renderer can switch on the track type it already knows.
type Event struct {
	StartMS int `json:"start"`
	EndMS   int `json:"end"`

	Src   string `json:"src,omitempty"`   // background / image / se / voice
	Rect  *Rect  `json:"rect,omitempty"`  // image
	Enter string `json:"enter,omitempty"` // image

	Dim  float64 `json:"dim,omitempty"`  // background: 0..1 darkening
	Blur float64 `json:"blur,omitempty"` // background: radius in px at Video.Height

	Expression string          `json:"expression,omitempty"` // character
	Speaking   bool            `json:"speaking,omitempty"`   // character
	Mouth      []wav.MouthSpan `json:"mouth,omitempty"`      // character, relative to StartMS

	Speaker string `json:"speaker,omitempty"` // subtitle
	Text    string `json:"text,omitempty"`    // subtitle
	Bubble  string `json:"bubble,omitempty"`  // subtitle: colour
	Anchor  string `json:"anchor,omitempty"`  // subtitle: which side the speaker stands on
}

type Rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}
