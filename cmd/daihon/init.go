package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// The scaffold deliberately ships no character assets: the tool is
// character-agnostic and redistributing artwork is not ours to do. The
// generated project builds as-is, drawn with placeholders.
const scaffoldMovie = `version: 1

meta:
  title: "サンプル"
  output: out/movie.mp4
  # 使用した音声エンジン・キャラクターのクレジットをここに書くと動画末尾に焼き込まれます。
  credit: ""

imports:
  - characters/cast.yml

video:
  width: 1920
  height: 1080
  fps: 30

audio:
  pause_between_lines_ms: 400

subtitle:
  mode: bubble
  size: 48

scenes:
  - id: intro
    script:
      - a: "はじめまして。これはサンプルの台本です。"
      - b: "movie.yml を編集すると動画が変わります。"
        expr: smile
      - wait: 600
      - a: "daihon check で台本を検査できます。"
`

const scaffoldCast = `# キャラクター定義。立ち絵のパスはこのファイルからの相対で書けます。
# expressions を空のままにするとプレースホルダの立ち絵で描画されます。
characters:
  a:
    name: "話者A"
    voice:
      engine: voicevox
      speaker_id: 3
      speed: 1.0
    stand:
      position: right
    expressions: {}
    bubble:
      color: "#4c9a2a"

  b:
    name: "話者B"
    voice:
      engine: voicevox
      speaker_id: 2
    stand:
      position: left
    expressions: {}
    bubble:
      color: "#a2416b"
`

const scaffoldGitignore = "out/\n.daihon-cache/\n"

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}

	if _, err := os.Stat(filepath.Join(dir, "movie.yml")); err == nil {
		return fmt.Errorf("%s には既に movie.yml があります", dir)
	}
	for _, sub := range []string{"characters", "assets", "out"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return err
		}
	}
	files := map[string]string{
		"movie.yml":           scaffoldMovie,
		"characters/cast.yml": scaffoldCast,
		".gitignore":          scaffoldGitignore,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			return err
		}
	}

	fmt.Printf("✔ %s に雛形を作りました\n\n次の手順:\n", dir)
	fmt.Println("  daihon check                    # 台本を検査")
	fmt.Println("  daihon timeline --engine estimate -o -   # 音声エンジンなしで尺を確認")
	fmt.Println("  daihon doctor                   # ffmpeg / VOICEVOX の状態を確認")
	return nil
}
