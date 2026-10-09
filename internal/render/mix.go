package render

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/takoikatakotako/daihon/internal/timeline"
	"github.com/takoikatakotako/daihon/internal/wav"
)

// mixVoice flattens the voice track into a single WAVE.
//
// Passing one file per line to ffmpeg would mean a filter graph with as many
// inputs as there are lines, which stops being debuggable quickly. Mixing here
// is a few lines of arithmetic and leaves ffmpeg with one audio input.
//
// This is derived at render time and never cached: caching a mixed file would
// undo the per-line cache, since editing one line would invalidate the whole mix.
func mixVoice(tl *timeline.Timeline, path string, tailMS int) error {
	const sampleRate = 24000

	// The tail must cover anything drawn after the last line (the credit card):
	// ffmpeg is run with -shortest, so a short audio track truncates the video.
	total := sampleRate * (tl.DurationMS + tailMS) / 1000
	acc := make([]int32, total)

	for _, tr := range tl.Tracks {
		if tr.Type != timeline.TrackVoice {
			continue
		}
		for _, ev := range tr.Events {
			b, err := os.ReadFile(ev.Src)
			if err != nil {
				return fmt.Errorf("音声を読み込めません %s: %w", ev.Src, err)
			}
			f, err := wav.Decode(b)
			if err != nil {
				return fmt.Errorf("%s: %w", ev.Src, err)
			}
			if err := addAt(acc, f, ev.StartMS, sampleRate); err != nil {
				return fmt.Errorf("%s: %w", ev.Src, err)
			}
		}
	}

	out := wav.Silence(tl.DurationMS+tailMS, sampleRate)
	body := out[44:]
	for i, v := range acc {
		if 2*i+1 >= len(body) {
			break
		}
		binary.LittleEndian.PutUint16(body[2*i:], uint16(int16(clamp16(v))))
	}
	return os.WriteFile(path, out, 0o644)
}

// addAt sums one clip into the accumulator at the given offset.
func addAt(acc []int32, f *wav.File, startMS, rate int) error {
	if f.Format.SampleRate != rate {
		return fmt.Errorf("サンプリングレートが %dHz ではありません (%dHz)", rate, f.Format.SampleRate)
	}
	step := f.Format.Channels * 2
	offset := rate * startMS / 1000
	for i := 0; i+step <= len(f.Data); i += step {
		j := offset + i/step
		if j >= len(acc) {
			break
		}
		acc[j] += int32(int16(binary.LittleEndian.Uint16(f.Data[i:])))
	}
	return nil
}

func clamp16(v int32) int32 {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return v
}
