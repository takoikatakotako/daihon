// Command daihon compiles a movie.yml script into a video.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
)

// version is set by the release build (-ldflags "-X main.version=v0.1.0").
var version = ""

// buildVersion falls back to the version the Go toolchain recorded: the tag for
// `go install ...@v0.1.0`, or a pseudo-version naming the commit when built from
// a git checkout. "dev" is left for builds with no VCS information at all.
func buildVersion() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

const usage = `daihon - movie.yml から解説動画をビルドする

使い方:
  daihon <command> [flags]

コマンド:
  init <dir>   movie.yml とディレクトリ雛形を作る
  doctor       ffmpeg と音声エンジンの状態を検査する
  check        台本を静的検査する（CI 向け）
  voice        音声だけ合成してキャッシュを温める
  timeline     timeline.json を出力する（映像は作らない）
  build        mp4 を書き出す
  version      バージョンを表示する

共通フラグ:
  -f <path>        movie.yml のパス (既定: movie.yml)
  --engine <name>  音声エンジンを上書きする (voicevox | say | gemini | estimate)
  --scene <id>     指定シーンだけを対象にする
  --no-autostart   VOICEVOX ENGINE を自動起動しない

音声エンジン:
  VOICEVOX ENGINE が起動していなければ、ローカルにインストールされたものを
  自動で起動し、コマンド終了時に停止します。場所は DAIHON_VOICEVOX_BIN で指定できます。

'daihon <command> -h' で各コマンドのフラグを表示します。
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var err error
	switch os.Args[1] {
	case "init":
		err = runInit(os.Args[2:])
	case "doctor":
		err = runDoctor(ctx, os.Args[2:])
	case "check":
		err = runCheck(os.Args[2:])
	case "voice":
		err = runVoice(ctx, os.Args[2:])
	case "timeline":
		err = runTimeline(ctx, os.Args[2:])
	case "build":
		err = runBuild(ctx, os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("daihon", buildVersion())
		return
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "不明なコマンド %q です\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}

	if err != nil {
		if errors.Is(err, errCheckFailed) {
			os.Exit(1) // diagnostics are already on stdout
		}
		fmt.Fprintln(os.Stderr, "エラー:", err)
		os.Exit(1)
	}
}

// commonFlags are shared by every command that loads a project.
type commonFlags struct {
	file        string
	engine      string
	scene       string
	noAutostart bool
}

func (c *commonFlags) bind(fs *flag.FlagSet) {
	fs.StringVar(&c.file, "f", "movie.yml", "movie.yml のパス")
	fs.StringVar(&c.engine, "engine", "", "音声エンジンを上書きする (voicevox | say | gemini | estimate)")
	fs.StringVar(&c.scene, "scene", "", "指定シーンだけを対象にする")
	fs.BoolVar(&c.noAutostart, "no-autostart", false, "VOICEVOX ENGINE を自動起動しない")
}
