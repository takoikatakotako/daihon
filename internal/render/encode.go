package render

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/takoikatakotako/daihon/internal/timeline"
)

// encoder runs ffmpeg with raw frames on stdin.
//
// ffmpeg is given exactly two jobs: encode the frames we hand it, and mix the
// audio. Everything visual happens in Go, so there is no filter_complex full of
// enable='between(t,..)' clauses to debug. See DESIGN.md §6.1.
type encoder struct {
	cmd    *exec.Cmd
	frames io.WriteCloser
	stderr *strings.Builder
}

// encodeOptions describes one ffmpeg invocation.
type encodeOptions struct {
	Width, Height, FPS int
	VoicePath          string
	BGM                *timeline.BGM
	BGMPath            string
	DurationMS         int
	VideoEncoder       string
	CRF                int    // libx264 only; 0 = defaultCRF
	AudioBitrate       string // "" = defaultAudioBitrate
	Output             string
}

func startEncoder(ctx context.Context, o encodeOptions) (*encoder, error) {
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		// Video: raw RGBA frames arriving on stdin.
		"-f", "rawvideo",
		"-pixel_format", "rgba",
		"-video_size", fmt.Sprintf("%dx%d", o.Width, o.Height),
		"-framerate", strconv.Itoa(o.FPS),
		"-i", "pipe:0",
		"-i", o.VoicePath,
	}

	filter := ""
	audioMap := "1:a"
	if o.BGMPath != "" {
		// Loop the track. BGM is typically a minute or two while a talk runs
		// much longer, so without this the music simply stops partway and the
		// rest of the video is dry. amix's duration=first trims the surplus.
		args = append(args, "-stream_loop", "-1", "-i", o.BGMPath)
		vol := 0.15
		if o.BGM != nil && o.BGM.Volume > 0 {
			vol = o.BGM.Volume
		}
		bgm := fmt.Sprintf("[2:a]volume=%.3f", vol)
		if o.BGM != nil && o.BGM.FadeOutMS > 0 {
			st := float64(o.DurationMS-o.BGM.FadeOutMS) / 1000
			bgm += fmt.Sprintf(",afade=t=out:st=%.3f:d=%.3f", st, float64(o.BGM.FadeOutMS)/1000)
		}
		// normalize=0 keeps the voice at its original level; amix would
		// otherwise halve everything just because a second input exists.
		filter = bgm + "[bgm];[1:a][bgm]amix=inputs=2:duration=first:normalize=0[a]"
		audioMap = "[a]"
	}
	if filter != "" {
		args = append(args, "-filter_complex", filter)
	}

	args = append(args,
		"-map", "0:v", "-map", audioMap,
		"-c:v", o.VideoEncoder,
	)
	args = append(args, qualityArgs(o)...)
	ab := o.AudioBitrate
	if ab == "" {
		ab = defaultAudioBitrate
	}
	args = append(args,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", ab,
		"-shortest",
		o.Output,
	)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ffmpeg を起動できません: %w", err)
	}
	return &encoder{cmd: cmd, frames: in, stderr: &stderr}, nil
}

// write sends one frame of RGBA pixels.
func (e *encoder) write(pix []byte) error {
	_, err := e.frames.Write(pix)
	return err
}

// finish closes the pipe and waits for ffmpeg to flush the file.
func (e *encoder) finish() error {
	if err := e.frames.Close(); err != nil {
		return err
	}
	if err := e.cmd.Wait(); err != nil {
		msg := strings.TrimSpace(e.stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("ffmpeg が失敗しました: %s", msg)
	}
	return nil
}

// bitsPerPixel sets the video bitrate relative to the frame size and rate.
//
// Subtitles are the hardest thing in this output to encode: large flat areas
// with hard high-contrast edges. h264_videotoolbox's default rate control lands
// near 3 Mbps at 1080p30, which visibly softens those edges, so the rate is set
// explicitly instead of left to the encoder.
const bitsPerPixel = 0.15

// defaultCRF is visually lossless for this kind of material.
const defaultCRF = 18

// defaultAudioBitrate leaves room for music. Speech alone is fine at 64k.
const defaultAudioBitrate = "192k"

// qualityArgs picks rate control appropriate to the encoder.
func qualityArgs(o encodeOptions) []string {
	if o.VideoEncoder == "libx264" {
		// x264 has proper constant-quality mode; use it and let the size fall
		// where it will.
		crf := o.CRF
		if crf == 0 {
			crf = defaultCRF
		}
		return []string{"-crf", strconv.Itoa(crf), "-preset", "medium"}
	}
	// videotoolbox has no constant-quality mode, and in practice it undershoots
	// an explicit bitrate rather than honouring it. That is acceptable: it is
	// only used for drafts, where timing and layout are what matter.
	rate := int(float64(o.Width*o.Height*o.FPS) * bitsPerPixel)
	return []string{
		"-b:v", strconv.Itoa(rate),
		"-maxrate", strconv.Itoa(rate * 3 / 2),
		"-bufsize", strconv.Itoa(rate * 2),
	}
}
