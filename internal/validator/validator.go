// Package validator implements `daihon check`: the static analysis pass over a
// script. It exists as its own command so a script can be verified in CI,
// before anyone spends minutes rendering it.
package validator

import (
	"fmt"
	"os"
	"regexp"
	"sort"

	"github.com/takoikatakotako/daihon/internal/parser"
	"github.com/takoikatakotako/daihon/internal/schema"
)

type Severity string

const (
	Error Severity = "error"
	Warn  Severity = "warn"
	Note  Severity = "note"
)

// Diag is one finding, anchored to a source line where we know it.
type Diag struct {
	Severity Severity
	Line     int
	Scene    string
	Message  string
}

func (d Diag) String() string {
	loc := ""
	if d.Line > 0 {
		loc = fmt.Sprintf(":%d", d.Line)
	}
	scene := ""
	if d.Scene != "" {
		scene = fmt.Sprintf(" [%s]", d.Scene)
	}
	return fmt.Sprintf("movie.yml%s %s%s: %s", loc, d.Severity, scene, d.Message)
}

// LongLineRunes is the point at which a single spoken line is called out. Long
// lines read badly and overflow the subtitle box; this is advice, not an error.
const LongLineRunes = 60

// Check validates a loaded project for the engine that will actually be used.
func Check(p *parser.Project, engineOverride string) []Diag {
	var ds []Diag
	m := p.Movie

	if m.Version != 1 {
		ds = append(ds, Diag{Severity: Error, Message: fmt.Sprintf("version は 1 である必要があります (got %d)", m.Version)})
	}
	if len(m.Scenes) == 0 {
		ds = append(ds, Diag{Severity: Error, Message: "scenes が空です"})
	}

	ds = append(ds, checkCharacters(p, engineOverride)...)
	ds = append(ds, checkFonts(p)...)
	ds = append(ds, checkBackground(p, m.Video.Background, "", "video.background")...)
	ds = append(ds, checkExport(m)...)
	if m.Audio.BGM != nil {
		ds = append(ds, checkAsset(p, m.Audio.BGM.Src, 0, "", "audio.bgm.src")...)
	}
	ds = append(ds, checkScenes(p, engineOverride)...)

	sort.SliceStable(ds, func(i, j int) bool { return rank(ds[i].Severity) < rank(ds[j].Severity) })
	return ds
}

// audioBitrate is what ffmpeg's -b:a accepts and a person would write.
var audioBitrate = regexp.MustCompile(`^[1-9][0-9]*k$`)

func checkExport(m *schema.Movie) []Diag {
	var ds []Diag
	e := m.Export
	if e.Height < 0 {
		ds = append(ds, Diag{Severity: Error, Message: fmt.Sprintf("export.height は正の数である必要があります (got %d)", e.Height)})
	} else if e.Height > m.Video.Height {
		ds = append(ds, Diag{Severity: Warn, Message: fmt.Sprintf("export.height (%d) が video.height (%d) より大きいので、拡大して書き出します", e.Height, m.Video.Height)})
	}
	if e.CRF < 0 || e.CRF > 51 {
		ds = append(ds, Diag{Severity: Error, Message: fmt.Sprintf("export.crf は 0〜51 である必要があります (got %d)", e.CRF)})
	}
	if e.AudioBitrate != "" && !audioBitrate.MatchString(e.AudioBitrate) {
		ds = append(ds, Diag{Severity: Error, Message: fmt.Sprintf("export.audio_bitrate は 64k のように書いてください (got %q)", e.AudioBitrate)})
	}
	return ds
}

func rank(s Severity) int {
	switch s {
	case Error:
		return 0
	case Warn:
		return 1
	default:
		return 2
	}
}

func checkCharacters(p *parser.Project, engineOverride string) []Diag {
	var ds []Diag
	if len(p.Movie.Characters) == 0 {
		ds = append(ds, Diag{Severity: Error, Message: "characters が空です（imports で読み込むか movie.yml に直接定義してください）"})
	}
	for _, id := range sortedKeys(p.Movie.Characters) {
		c := p.Movie.Characters[id]
		engine := effectiveEngine(c.Voice.Engine, engineOverride)
		switch engine {
		case "voicevox", "say", "estimate":
		case "gemini":
			if c.Voice.Model != "" && c.Voice.Model != "gemini-3.8-flash-tts" && c.Voice.Model != "gemini-3.8-flash-lite-tts" {
				ds = append(ds, Diag{Severity: Error, Message: fmt.Sprintf("%s.voice.model: 未対応の Gemini TTS モデル %q です", id, c.Voice.Model)})
			}
			if c.Voice.Speed != 1 || c.Voice.Pitch != 0 {
				ds = append(ds, Diag{Severity: Error, Message: fmt.Sprintf("%s.voice: Gemini TTS は speed/pitch に対応していません。style で指定してください", id)})
			}
		default:
			ds = append(ds, Diag{Severity: Error, Message: fmt.Sprintf("%s.voice.engine: 未知の音声エンジン %q です", id, engine)})
		}
		if engine != "gemini" && c.Voice.Style != "" {
			ds = append(ds, Diag{Severity: Warn, Message: fmt.Sprintf("%s.voice.style: %s では使われません", id, engine)})
		}
		if schema.IsReserved(id) {
			ds = append(ds, Diag{Severity: Error, Message: fmt.Sprintf("キャラクター ID %q は予約語です", id)})
		}
		if c.Name == "" {
			ds = append(ds, Diag{Severity: Warn, Message: fmt.Sprintf("キャラクター %q に name がありません", id)})
		}
		if c.Stand.Position == schema.StandNone {
			// A narrator: no figure is drawn, so there is nothing to warn about.
		} else if len(c.Expressions) == 0 {
			ds = append(ds, Diag{Severity: Warn, Message: fmt.Sprintf("キャラクター %q に立ち絵がありません。プレースホルダで描画します", id)})
		} else if _, ok := c.Expressions[c.DefaultExpression]; !ok {
			ds = append(ds, Diag{Severity: Error, Message: fmt.Sprintf("キャラクター %q の default_expression %q が expressions にありません", id, c.DefaultExpression)})
		}
		for _, name := range sortedKeys(c.Expressions) {
			ds = append(ds, checkAsset(p, c.Expressions[name], 0, "", fmt.Sprintf("%s.expressions.%s", id, name))...)
		}
		if c.Lipsync != nil {
			ds = append(ds, checkAsset(p, c.Lipsync.Open, 0, "", id+".lipsync.open")...)
			ds = append(ds, checkAsset(p, c.Lipsync.Close, 0, "", id+".lipsync.close")...)
		}
	}
	return ds
}

func checkFonts(p *parser.Project) []Diag {
	var ds []Diag
	m := p.Movie
	for _, name := range sortedKeys(m.Fonts) {
		ds = append(ds, checkAsset(p, m.Fonts[name], 0, "", "fonts."+name)...)
	}
	if m.Subtitle.Mode != "none" {
		name := m.Subtitle.Font
		if name == "" {
			name = "default"
		}
		if _, ok := m.Fonts[name]; !ok {
			ds = append(ds, Diag{Severity: Warn, Message: fmt.Sprintf("字幕フォント %q が fonts に定義されていません。既定フォントで描画します", name)})
		}
	}
	return ds
}

func checkScenes(p *parser.Project, engineOverride string) []Diag {
	var ds []Diag
	seenScene := map[string]bool{}

	for _, sc := range p.Movie.Scenes {
		if sc.ID == "" {
			ds = append(ds, Diag{Severity: Error, Message: "id のないシーンがあります"})
		} else if seenScene[sc.ID] {
			ds = append(ds, Diag{Severity: Error, Message: fmt.Sprintf("シーン ID %q が重複しています", sc.ID)})
		}
		seenScene[sc.ID] = true
		ds = append(ds, checkBackground(p, sc.Background, sc.ID, "background")...)

		// `show` handles are scene-scoped: a scene cannot hide another's image.
		open := map[string]int{}
		for _, it := range sc.Script {
			switch it.Kind {
			case schema.KindLine:
				ds = append(ds, checkLine(p, sc, it, engineOverride)...)
			case schema.KindShow:
				if it.Src == "" {
					ds = append(ds, Diag{Severity: Error, Line: it.Line, Scene: sc.ID, Message: "show にパスがありません"})
				}
				ds = append(ds, checkAsset(p, it.Src, it.Line, sc.ID, "show")...)
				if it.As != "" {
					if _, dup := open[it.As]; dup {
						ds = append(ds, Diag{Severity: Error, Line: it.Line, Scene: sc.ID, Message: fmt.Sprintf("名前 %q は既に表示中です", it.As)})
					}
					open[it.As] = it.Line
				}
				if it.At != "" && !validAt(it.At) {
					ds = append(ds, Diag{Severity: Error, Line: it.Line, Scene: sc.ID, Message: fmt.Sprintf("at は left/center/right のいずれかです (got %q)", it.At)})
				}
			case schema.KindHide:
				if _, ok := open[it.Target]; !ok {
					ds = append(ds, Diag{Severity: Error, Line: it.Line, Scene: sc.ID, Message: fmt.Sprintf("表示されていない %q を hide しています", it.Target)})
				}
				delete(open, it.Target)
			case schema.KindEnter, schema.KindExit:
				if _, ok := p.Movie.Characters[it.Target]; !ok {
					ds = append(ds, Diag{Severity: Error, Line: it.Line, Scene: sc.ID,
						Message: fmt.Sprintf("%s: 未定義のキャラクター %q です", it.Kind, it.Target)})
				}
			case schema.KindBG:
				ds = append(ds, checkAsset(p, it.Path, it.Line, sc.ID, "bg")...)
			case schema.KindSE:
				ds = append(ds, checkAsset(p, it.Path, it.Line, sc.ID, "se")...)
			case schema.KindWait:
				if it.WaitMS <= 0 {
					ds = append(ds, Diag{Severity: Warn, Line: it.Line, Scene: sc.ID, Message: "wait が 0 以下です"})
				}
			case schema.KindNote:
				ds = append(ds, Diag{Severity: Note, Line: it.Line, Scene: sc.ID, Message: it.Note})
			}
		}
		for name, line := range open {
			ds = append(ds, Diag{Severity: Warn, Line: line, Scene: sc.ID,
				Message: fmt.Sprintf("%q が hide されないままシーンが終わります", name)})
		}
	}
	return ds
}

func checkLine(p *parser.Project, sc schema.Scene, it schema.ScriptItem, engineOverride string) []Diag {
	var ds []Diag
	c, ok := p.Movie.Characters[it.Speaker]
	if !ok {
		return append(ds, Diag{Severity: Error, Line: it.Line, Scene: sc.ID,
			Message: fmt.Sprintf("未定義のキャラクター %q です", it.Speaker)})
	}
	if it.Text == "" {
		ds = append(ds, Diag{Severity: Error, Line: it.Line, Scene: sc.ID, Message: "セリフが空です"})
	}
	if n := len([]rune(it.Text)); n > LongLineRunes {
		ds = append(ds, Diag{Severity: Warn, Line: it.Line, Scene: sc.ID,
			Message: fmt.Sprintf("セリフが長すぎます (%d文字)。分割を検討してください", n)})
	}
	if it.Read != "" && it.Read == it.Text {
		ds = append(ds, Diag{Severity: Warn, Line: it.Line, Scene: sc.ID,
			Message: "read が text と同一です。不要なら削除してください"})
	}
	engine := effectiveEngine(c.Voice.Engine, engineOverride)
	if engine == "gemini" && it.Speed != nil && *it.Speed != 1 {
		ds = append(ds, Diag{Severity: Error, Line: it.Line, Scene: sc.ID,
			Message: "Gemini TTS は speed に対応していません。style で指定してください"})
	}
	if engine != "gemini" && it.Style != "" {
		ds = append(ds, Diag{Severity: Warn, Line: it.Line, Scene: sc.ID,
			Message: fmt.Sprintf("style は %s では使われません", engine)})
	}
	if it.Expr != "" && len(c.Expressions) > 0 {
		if _, ok := c.Expressions[it.Expr]; !ok {
			ds = append(ds, Diag{Severity: Error, Line: it.Line, Scene: sc.ID,
				Message: fmt.Sprintf("%s に表情 %q がありません", it.Speaker, it.Expr)})
		}
	}
	return ds
}

func effectiveEngine(configured, override string) string {
	if override != "" {
		return override
	}
	return configured
}

// checkAsset reports a missing file as a warning, not an error: a missing asset
// is drawn as a placeholder so that a script still builds. See DESIGN.md §6.4.
func checkBackground(p *parser.Project, bg schema.Background, scene, field string) []Diag {
	ds := checkAsset(p, bg.Src, 0, scene, field)
	if bg.Dim < 0 || bg.Dim > 1 {
		ds = append(ds, Diag{Severity: Error, Scene: scene,
			Message: fmt.Sprintf("%s.dim は 0〜1 で指定してください (got %g)", field, bg.Dim)})
	}
	if bg.Blur < 0 {
		ds = append(ds, Diag{Severity: Error, Scene: scene,
			Message: fmt.Sprintf("%s.blur は 0 以上で指定してください (got %g)", field, bg.Blur)})
	}
	return ds
}

func checkAsset(p *parser.Project, rel string, line int, scene, field string) []Diag {
	if rel == "" {
		return nil
	}
	if _, err := os.Stat(p.Abs(rel)); err != nil {
		return []Diag{{Severity: Warn, Line: line, Scene: scene,
			Message: fmt.Sprintf("%s: ファイルがありません %s（プレースホルダで描画します）", field, rel)}}
	}
	return nil
}

func validAt(at string) bool {
	return at == "left" || at == "center" || at == "right"
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// HasErrors reports whether any diagnostic is fatal.
func HasErrors(ds []Diag) bool {
	for _, d := range ds {
		if d.Severity == Error {
			return true
		}
	}
	return false
}
