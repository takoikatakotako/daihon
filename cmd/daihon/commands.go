package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/takoikatakotako/daihon/internal/doctor"
	"github.com/takoikatakotako/daihon/internal/parser"
	"github.com/takoikatakotako/daihon/internal/render"
	"github.com/takoikatakotako/daihon/internal/timeline"
	"github.com/takoikatakotako/daihon/internal/validator"
)

func runCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	var c commonFlags
	c.bind(fs)
	quiet := fs.Bool("quiet", false, "エラーのみ表示する")
	if err := fs.Parse(args); err != nil {
		return err
	}

	p, err := load(c.file)
	if err != nil {
		return err
	}
	ds := validator.Check(p)

	var errs, warns int
	for _, d := range ds {
		switch d.Severity {
		case validator.Error:
			errs++
		case validator.Warn:
			warns++
		}
		if *quiet && d.Severity != validator.Error {
			continue
		}
		fmt.Println(d)
	}

	if errs == 0 && warns == 0 {
		fmt.Println("✔ 問題は見つかりませんでした")
		return nil
	}
	fmt.Printf("\n%d error, %d warning\n", errs, warns)
	if errs > 0 {
		return errCheckFailed
	}
	return nil
}

func runVoice(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("voice", flag.ExitOnError)
	var c commonFlags
	c.bind(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	tl, syn, err := buildTimeline(ctx, c)
	if err != nil {
		return err
	}
	fmt.Printf("✔ 音声 %d 行（キャッシュ %d / 新規 %d）, 合計 %s\n",
		syn.Hits+syn.Misses, syn.Hits, syn.Misses, formatMS(tl.DurationMS))
	return nil
}

func runTimeline(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("timeline", flag.ExitOnError)
	var c commonFlags
	c.bind(fs)
	out := fs.String("o", "", "出力先 (既定: .daihon-cache/timeline.json, '-' で標準出力)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	p, err := load(c.file)
	if err != nil {
		return err
	}
	tl, syn, err := buildTimelineFor(ctx, p, c)
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(tl, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	dest := *out
	if dest == "" {
		dest = filepath.Join(p.Root, cacheDir, "timeline.json")
	}
	if dest == "-" {
		_, err := os.Stdout.Write(b)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dest, b, 0o644); err != nil {
		return err
	}
	fmt.Printf("✔ %s (%s, %d トラック, 音声キャッシュ %d/%d)\n",
		dest, formatMS(tl.DurationMS), len(tl.Tracks), syn.Hits, syn.Hits+syn.Misses)
	return nil
}

func runBuild(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	var c commonFlags
	c.bind(fs)
	draft := fs.Bool("draft", false, "低解像度・低fpsで書き出す（反復用）")
	out := fs.String("o", "", "出力先 (既定: meta.output)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	p, err := load(c.file)
	if err != nil {
		return err
	}
	tl, syn, err := buildTimelineFor(ctx, p, c)
	if err != nil {
		return err
	}
	fmt.Printf("音声 %d 行（キャッシュ %d / 新規 %d）, 尺 %s\n",
		syn.Hits+syn.Misses, syn.Hits, syn.Misses, formatMS(tl.DurationMS))

	r := &render.Renderer{
		Project:  p,
		Timeline: tl,
		Opts:     render.Options{Draft: *draft, Output: *out},
	}
	start := time.Now()
	st, err := r.Render(ctx)
	if err != nil {
		return err
	}

	for _, m := range st.Missing {
		fmt.Printf("  ! %s が無いためプレースホルダで描画しました\n", m)
	}
	fmt.Printf("✔ %s (%dx%d @%dfps, %d フレーム, %s, %s)\n",
		st.Output, st.Width, st.Height, st.FPS, st.Frames, st.Encoder,
		time.Since(start).Round(time.Millisecond))
	return nil
}

func runDoctor(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	results := doctor.Run(ctx, os.Getenv("DAIHON_VOICEVOX_URL"))
	ok := true
	for _, r := range results {
		fmt.Println(r)
		if !r.OK && r.Required {
			ok = false
		}
	}
	if !ok {
		return fmt.Errorf("必須の依存が足りていません")
	}
	return nil
}

// buildTimeline is the shared path for commands that need synthesised audio.
func buildTimeline(ctx context.Context, c commonFlags) (*timeline.Timeline, *voiceStats, error) {
	p, err := load(c.file)
	if err != nil {
		return nil, nil, err
	}
	return buildTimelineFor(ctx, p, c)
}

type voiceStats struct{ Hits, Misses int }

func buildTimelineFor(ctx context.Context, p *parser.Project, c commonFlags) (*timeline.Timeline, *voiceStats, error) {
	stop, err := ensureEngine(ctx, p, c.engine, !c.noAutostart)
	if err != nil {
		return nil, nil, err
	}
	defer stop()

	if ds := validator.Check(p); validator.HasErrors(ds) {
		for _, d := range ds {
			fmt.Println(d)
		}
		return nil, nil, errCheckFailed
	}

	syn := newSynth(p)
	b := &timeline.Builder{
		Project:        p,
		Synth:          syn,
		EngineOverride: c.engine,
		SceneFilter:    c.scene,
	}
	tl, err := b.Build(ctx)
	if err != nil {
		return nil, nil, err
	}
	return tl, &voiceStats{Hits: syn.Hits, Misses: syn.Misses}, nil
}

func formatMS(ms int) string {
	return fmt.Sprintf("%02d:%02d.%03d", ms/60000, ms/1000%60, ms%1000)
}
