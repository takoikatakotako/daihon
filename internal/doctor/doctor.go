// Package doctor implements `daihon doctor`: it checks the things this tool needs
// from the machine it runs on.
//
// A single Go binary is easy to ship, but ffmpeg is an external dependency and
// a speech engine is a running server, so "it does not work" has to be turned
// into an actionable message rather than a stack trace.
package doctor

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/takoikatakotako/daihon/internal/voice/voicevox"
)

// Result is one checked item.
type Result struct {
	Name     string
	OK       bool
	Required bool
	Detail   string
	Hint     string
}

func (r Result) String() string {
	mark := "✔"
	if !r.OK {
		mark = "✘"
		if !r.Required {
			mark = "•"
		}
	}
	s := fmt.Sprintf("%s %-24s %s", mark, r.Name, r.Detail)
	if !r.OK && r.Hint != "" {
		s += "\n    → " + r.Hint
	}
	return s
}

// Run performs every check. voicevoxURL may be empty for the default.
func Run(ctx context.Context, voicevoxURL string) []Result {
	if voicevoxURL == "" {
		voicevoxURL = "http://127.0.0.1:50021"
	}
	out := []Result{checkFFmpeg(ctx)}
	out = append(out, checkEncoders(ctx)...)
	out = append(out, checkVoicevox(ctx, voicevoxURL))
	out = append(out, checkVoicevoxBinary())
	out = append(out, checkMacSay())
	return out
}

func checkMacSay() Result {
	r := Result{Name: "macOS say", Required: false}
	if runtime.GOOS != "darwin" {
		r.Detail = "macOS でのみ利用できます"
		return r
	}
	path, err := exec.LookPath("say")
	if err != nil {
		r.Detail = "見つかりません"
		return r
	}
	r.OK, r.Detail = true, path
	return r
}

var versionRe = regexp.MustCompile(`ffmpeg version (\S+)`)

func checkFFmpeg(ctx context.Context) Result {
	r := Result{Name: "ffmpeg", Required: true}
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		r.Detail = "見つかりません"
		r.Hint = "brew install ffmpeg （macOS） / apt install ffmpeg （Debian系）"
		return r
	}
	b, err := exec.CommandContext(ctx, "ffmpeg", "-version").Output()
	if err != nil {
		r.Detail = path + " を実行できません"
		r.Hint = "ffmpeg -version が通るか確認してください"
		return r
	}
	v := "unknown"
	if m := versionRe.FindSubmatch(b); m != nil {
		v = string(m[1])
	}
	r.OK, r.Detail = true, fmt.Sprintf("%s (%s)", v, path)
	return r
}

// checkEncoders reports which H.264 encoder will be used. videotoolbox is
// hardware-accelerated but macOS-only, so libx264 is the portable fallback.
func checkEncoders(ctx context.Context) []Result {
	b, err := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-encoders").Output()
	if err != nil {
		return nil
	}
	list := string(b)
	has := func(name string) bool { return strings.Contains(list, " "+name+" ") }

	var chosen string
	switch {
	case has("h264_videotoolbox"):
		chosen = "h264_videotoolbox （ハードウェア）"
	case has("libx264"):
		chosen = "libx264 （ソフトウェア）"
	}
	r := Result{Name: "H.264 エンコーダ", Required: true}
	if chosen == "" {
		r.Detail = "利用可能な H.264 エンコーダがありません"
		r.Hint = "libx264 を含む ffmpeg を入れてください"
	} else {
		r.OK, r.Detail = true, chosen
	}

	// Not fatal, but worth surfacing: we never rely on ffmpeg for text.
	t := Result{Name: "テキスト描画", OK: true, Detail: "Go 側で描画（ffmpeg の drawtext/libass は使いません）"}
	return []Result{r, t}
}

func checkVoicevox(ctx context.Context, base string) Result {
	r := Result{Name: "VOICEVOX ENGINE", Required: false}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/version", nil)
	if err != nil {
		r.Detail = err.Error()
		return r
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		r.Detail = base + " で待ち受けていません"
		r.Hint = "docker compose up -d で起動できます。ネイティブ版があれば daihon が自動起動します"
		return r
	}
	defer resp.Body.Close()
	r.OK, r.Detail = true, base+" に接続できました"
	return r
}

// checkVoicevoxBinary reports whether a local engine can be started on demand.
func checkVoicevoxBinary() Result {
	r := Result{Name: "VOICEVOX (ローカル)", Required: false}
	bin := voicevox.Locate()
	if bin == "" {
		r.Detail = "ネイティブ版なし（Docker を使う場合はこれで問題ありません）"
		r.Hint = "自動起動させたい場合は https://github.com/VOICEVOX/voicevox_engine/releases の " +
			"macos-arm64 版を置き、DAIHON_VOICEVOX_BIN でパスを指定してください"
		return r
	}
	r.OK, r.Detail = true, bin+" （必要になれば自動起動します）"
	return r
}
