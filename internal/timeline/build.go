package timeline

import (
	"context"
	"fmt"

	"github.com/takoikatakotako/daihon/internal/parser"
	"github.com/takoikatakotako/daihon/internal/schema"
	"github.com/takoikatakotako/daihon/internal/voice"
	"github.com/takoikatakotako/daihon/internal/wav"
)

// Builder assembles a Timeline from a project.
type Builder struct {
	Project *parser.Project
	Synth   *voice.Synthesizer
	// EngineOverride forces every character onto one engine, which is how
	// `--engine estimate` produces a timeline without a speech server.
	EngineOverride string
	// SceneFilter, when non-empty, restricts the build to one scene.
	SceneFilter string
}

// Build walks the script, synthesising each line and advancing a cursor.
func (b *Builder) Build(ctx context.Context) (*Timeline, error) {
	m := b.Project.Movie

	tl := &Timeline{
		Version: 1,
		Title:   m.Meta.Title,
		Video:   Video{Width: m.Video.Width, Height: m.Video.Height, FPS: m.Video.FPS},
		Credit:  m.Meta.Credit,
	}
	if m.Audio.BGM != nil {
		tl.BGM = &BGM{Src: m.Audio.BGM.Src, Volume: m.Audio.BGM.Volume, FadeOutMS: m.Audio.BGM.FadeOutMS}
	}

	bg := newLayer()
	images := newImageLayer(m.Video.Width, m.Video.Height)
	chars := newCharLayer(m.Characters)
	var subtitles, voices, ses []Event

	cursor := 0
	for _, sc := range m.Scenes {
		if b.SceneFilter != "" && sc.ID != b.SceneFilter {
			continue
		}

		background := sc.Background
		if background.IsZero() {
			background = m.Video.Background
		}
		bg.set(cursor, background)

		for _, it := range sc.Script {
			switch it.Kind {
			case schema.KindLine:
				dur, err := b.speak(ctx, &subtitles, &voices, chars, cursor, it)
				if err != nil {
					return nil, fmt.Errorf("%s:%d: %w", sc.ID, it.Line, err)
				}
				pause := m.Audio.PauseBetweenLinesMS
				if it.Wait != nil {
					pause = *it.Wait
				}
				cursor += dur + pause

			case schema.KindShow:
				images.show(cursor, it, chars.occupiedSides(m.Characters))
			case schema.KindHide:
				images.hide(cursor, it.Target)
			case schema.KindBG:
				bg.set(cursor, schema.Background{Src: it.Path})
			case schema.KindEnter:
				chars.setVisible(it.Target, cursor, true)
			case schema.KindExit:
				chars.setVisible(it.Target, cursor, false)
			case schema.KindSE:
				ses = append(ses, Event{StartMS: cursor, EndMS: cursor, Src: it.Path})
			case schema.KindWait:
				cursor += it.WaitMS
			case schema.KindNote:
				// Authoring aid only; `daihon check` surfaces these.
			}
		}
		images.closeAll(cursor) // `show` handles do not leak across scenes
	}

	tl.DurationMS = cursor
	bg.close(cursor)
	chars.close(cursor)

	// Empty tracks are dropped: a renderer should not have to distinguish
	// "no background in this movie" from "background track present but null".
	add := func(kind, id string, evs []Event) {
		if len(evs) == 0 {
			return
		}
		tl.Tracks = append(tl.Tracks, Track{Type: kind, ID: id, Events: evs})
	}
	add(TrackBackground, "", bg.events)
	add(TrackImage, "", images.events)
	for _, id := range chars.order {
		add(TrackCharacter, id, chars.states[id].events)
	}
	add(TrackSubtitle, "", subtitles)
	add(TrackVoice, "", voices)
	add(TrackSE, "", ses)
	return tl, nil
}

// speak synthesises one line and appends its subtitle, audio and mouth motion.
// It returns how long the audio is; the caller advances the cursor.
func (b *Builder) speak(ctx context.Context, subs, voices *[]Event, chars *charLayer, start int, it schema.ScriptItem) (int, error) {
	c := b.Project.Movie.Characters[it.Speaker]
	if c == nil {
		return 0, fmt.Errorf("未定義のキャラクター %q です", it.Speaker)
	}

	engine := c.Voice.Engine
	if b.EngineOverride != "" {
		engine = b.EngineOverride
	}
	opts := voice.Options{SpeakerID: c.Voice.SpeakerID, VoiceName: c.Voice.VoiceName, Speed: c.Voice.Speed, Pitch: c.Voice.Pitch,
		Model: c.Voice.Model, Language: c.Voice.Language, Style: c.Voice.Style}
	if it.Speed != nil {
		opts.Speed = *it.Speed
	}
	if it.Style != "" {
		opts.Style = it.Style
	}

	// The engine reads Read when given; the subtitle always shows Text.
	spoken := it.Text
	if it.Read != "" {
		spoken = it.Read
	}
	res, err := b.Synth.Synth(ctx, engine, spoken, opts)
	if err != nil {
		return 0, err
	}
	end := start + res.DurationMS

	var mouth []wav.MouthSpan
	if c.Lipsync != nil {
		if f, err := wav.Decode(res.Wav); err == nil {
			mouth = f.Mouth(b.Project.Movie.Video.FPS, wav.DefaultMouthRatio)
		}
	}

	expr := it.Expr
	if expr == "" {
		expr = chars.states[it.Speaker].expr // expressions persist until changed
	}
	// Whoever speaks is on stage. This saves an explicit `enter` before
	// almost every entrance, and it means a hidden speaker can never be heard
	// from nowhere by mistake.
	chars.setVisible(it.Speaker, start, true)
	chars.transition(it.Speaker, start, expr, true, mouth)
	chars.transition(it.Speaker, end, expr, false, nil)

	// The subtitle lingers through the trailing pause: dropping it for 400ms
	// between lines reads as a flicker.
	pause := b.Project.Movie.Audio.PauseBetweenLinesMS
	if it.Wait != nil {
		pause = *it.Wait
	}
	*subs = append(*subs, Event{
		StartMS: start, EndMS: end + pause,
		Speaker: c.Name, Text: it.Text,
		Bubble: c.Bubble.Color, Anchor: c.Stand.Position,
	})
	*voices = append(*voices, Event{StartMS: start, EndMS: end, Src: res.Path})
	return res.DurationMS, nil
}
