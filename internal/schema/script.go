package schema

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Kind discriminates the entries of a scene's script.
type Kind string

const (
	KindLine Kind = "line" // a spoken line: `- zunda: "..."`
	KindShow Kind = "show"
	KindHide Kind = "hide"
	KindBG   Kind = "bg"
	KindWait Kind = "wait"
	KindSE   Kind = "se"
	KindNote Kind = "note"
	// Stage directions for characters. A figure is on stage from the start;
	// `exit` takes it off, `enter` (or speaking) brings it back.
	KindEnter Kind = "enter"
	KindExit  Kind = "exit"
)

// Reserved keys cannot be used as character IDs. Keeping this set closed is what
// lets us decide "is this line speech or a directive?" without ambiguity.
var reserved = map[string]Kind{
	"show": KindShow,
	"hide": KindHide,
	"bg":   KindBG,
	"wait": KindWait,
	"se":   KindSE,
	"note": KindNote,
	// "enter" is also a modifier of `show` (how an image enters). The two
	// never meet: the head key of an item decides its kind, and modifiers are
	// only read after that. `- enter: metan` is a direction; `enter: fade`
	// under a show is a modifier.
	"enter": KindEnter,
	"exit":  KindExit,
}

// IsReserved reports whether name is a directive keyword.
func IsReserved(name string) bool {
	_, ok := reserved[name]
	return ok
}

// ScriptItem is one entry of a scene's script. The first key of the mapping
// decides what the item is: a reserved word makes it a directive, anything else
// is read as a character ID and the item becomes a spoken line.
type ScriptItem struct {
	Kind Kind
	Line int // source line, for diagnostics

	// KindLine
	Speaker string
	Text    string
	// Read overrides what is sent to the speech engine while Text stays on
	// screen. Needed whenever a name is spelled one way and read another
	// ("au PAY" is read エーユーペイ, not エイユウペイ).
	Read  string
	Expr  string
	Wait  *int
	Speed *float64

	// KindShow
	Src   string
	As    string
	At    string
	Enter string

	// KindHide (image handle), KindEnter / KindExit (character ID)
	Target string

	// KindBG / KindSE
	Path string

	// KindWait
	WaitMS int

	// KindNote
	Note string
}

// modifiers are keys allowed to follow the head key of an item.
var modifiers = map[string]bool{
	"expr": true, "wait": true, "speed": true, "read": true,
	"as": true, "at": true, "enter": true,
}

func (s *ScriptItem) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: script の各要素はマッピングである必要があります", node.Line)
	}
	if len(node.Content) < 2 {
		return fmt.Errorf("line %d: 空の script 要素です", node.Line)
	}
	s.Line = node.Line

	head := node.Content[0].Value
	headVal := node.Content[1]

	kind, isDirective := reserved[head]
	if isDirective {
		s.Kind = kind
		switch kind {
		case KindShow:
			s.Src = headVal.Value
		case KindHide, KindEnter, KindExit:
			s.Target = headVal.Value
		case KindBG, KindSE:
			s.Path = headVal.Value
		case KindNote:
			s.Note = headVal.Value
		case KindWait:
			if err := headVal.Decode(&s.WaitMS); err != nil {
				return fmt.Errorf("line %d: wait はミリ秒の整数で指定してください", headVal.Line)
			}
		}
	} else {
		s.Kind = KindLine
		s.Speaker = head
		if headVal.Kind != yaml.ScalarNode {
			return fmt.Errorf("line %d: セリフは文字列で指定してください", headVal.Line)
		}
		s.Text = headVal.Value
	}

	// Remaining keys are modifiers on the head.
	for i := 2; i < len(node.Content); i += 2 {
		key, val := node.Content[i].Value, node.Content[i+1]
		if !modifiers[key] {
			return fmt.Errorf("line %d: 不明なキー %q です（1要素に命令を2つ書いていませんか）", node.Content[i].Line, key)
		}
		switch key {
		case "expr":
			s.Expr = val.Value
		case "read":
			s.Read = val.Value
		case "as":
			s.As = val.Value
		case "at":
			s.At = val.Value
		case "enter":
			s.Enter = val.Value
		case "wait":
			var ms int
			if err := val.Decode(&ms); err != nil {
				return fmt.Errorf("line %d: wait はミリ秒の整数で指定してください", val.Line)
			}
			s.Wait = &ms
		case "speed":
			var sp float64
			if err := val.Decode(&sp); err != nil {
				return fmt.Errorf("line %d: speed は数値で指定してください", val.Line)
			}
			s.Speed = &sp
		}
	}
	return nil
}
