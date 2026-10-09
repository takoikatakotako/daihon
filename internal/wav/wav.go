// Package wav reads the little we need from RIFF/WAVE files: how long they are,
// and where the speaker's mouth should be open.
//
// This is deliberately a hand-rolled parser rather than a dependency: we only
// ever handle uncompressed PCM produced by a speech engine, and keeping the
// dependency list short matters for a single-binary tool.
package wav

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Format describes the PCM layout of a decoded file.
type Format struct {
	Channels      int
	SampleRate    int
	BitsPerSample int
}

// File is a decoded WAVE file.
type File struct {
	Format Format
	Data   []byte // raw PCM
}

var errShort = errors.New("wav: ファイルが短すぎます")

// Decode parses a RIFF/WAVE byte slice.
func Decode(b []byte) (*File, error) {
	if len(b) < 12 {
		return nil, errShort
	}
	if string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, errors.New("wav: RIFF/WAVE ヘッダではありません")
	}

	var f File
	var sawFmt bool
	pos := 12
	for pos+8 <= len(b) {
		id := string(b[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(b[pos+4 : pos+8]))
		body := pos + 8
		if size < 0 || body+size > len(b) {
			// Some engines write a streaming-style size; take what is there.
			size = len(b) - body
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return nil, errShort
			}
			f.Format.Channels = int(binary.LittleEndian.Uint16(b[body+2 : body+4]))
			f.Format.SampleRate = int(binary.LittleEndian.Uint32(b[body+4 : body+8]))
			f.Format.BitsPerSample = int(binary.LittleEndian.Uint16(b[body+14 : body+16]))
			sawFmt = true
		case "data":
			f.Data = b[body : body+size]
		}
		pos = body + size
		if size%2 == 1 {
			pos++ // chunks are word-aligned
		}
	}
	if !sawFmt {
		return nil, errors.New("wav: fmt チャンクがありません")
	}
	if f.Data == nil {
		return nil, errors.New("wav: data チャンクがありません")
	}
	if f.Format.BitsPerSample != 16 {
		return nil, fmt.Errorf("wav: 16bit PCM のみ対応しています (got %dbit)", f.Format.BitsPerSample)
	}
	if f.Format.Channels < 1 || f.Format.SampleRate < 1 {
		return nil, errors.New("wav: fmt チャンクの値が不正です")
	}
	return &f, nil
}

// DurationMS returns the playback length in milliseconds.
func (f *File) DurationMS() int {
	bytesPerFrame := f.Format.Channels * f.Format.BitsPerSample / 8
	if bytesPerFrame == 0 {
		return 0
	}
	frames := len(f.Data) / bytesPerFrame
	return int(int64(frames) * 1000 / int64(f.Format.SampleRate))
}

// DurationMS decodes b and returns its length in milliseconds.
func DurationMS(b []byte) (int, error) {
	f, err := Decode(b)
	if err != nil {
		return 0, err
	}
	return f.DurationMS(), nil
}

// MouthSpan is a contiguous run of video frames with the mouth in one state.
type MouthSpan struct {
	StartMS int  `json:"start"`
	EndMS   int  `json:"end"`
	Open    bool `json:"open"`
}

// DefaultMouthRatio is the fraction of a clip's peak loudness above which the
// mouth counts as open.
//
// The threshold is relative rather than absolute because loudness varies with
// the engine, the speaker and the per-line speed: a fixed level that suits one
// character leaves another barely moving their mouth. Calibrating against the
// clip's own peak makes the result consistent across voices.
const DefaultMouthRatio = 0.18

// mouthFloor stops a clip of near-silence from being scaled up into chatter.
const mouthFloor = 0.02

// Mouth analyses loudness per video frame and returns the merged open/close
// spans. Driving the mouth from the actual waveform costs about the same as
// alternating on a timer, and looks markedly better.
func (f *File) Mouth(fps int, ratio float64) []MouthSpan {
	if fps <= 0 {
		return nil
	}
	bytesPerFrame := f.Format.Channels * 2
	total := len(f.Data) / bytesPerFrame
	if total == 0 {
		return nil
	}
	step := f.Format.SampleRate / fps
	if step < 1 {
		step = 1
	}

	// First pass: loudness per video frame, and the clip's peak.
	levels := make([]float64, 0, total/step+1)
	peak := 0.0
	for start := 0; start < total; start += step {
		end := min(start+step, total)
		var sum float64
		for i := start; i < end; i++ {
			// Mono-ise by taking the first channel; speech is centred anyway.
			off := i * bytesPerFrame
			s := float64(int16(binary.LittleEndian.Uint16(f.Data[off:off+2]))) / 32768.0
			sum += s * s
		}
		rms := 0.0
		if n := end - start; n > 0 {
			rms = sqrt(sum / float64(n))
		}
		levels = append(levels, rms)
		peak = max(peak, rms)
	}

	threshold := max(peak*ratio, mouthFloor)

	var spans []MouthSpan
	for i, rms := range levels {
		open := rms > threshold
		startMS := i * step * 1000 / f.Format.SampleRate
		endMS := min((i+1)*step, total) * 1000 / f.Format.SampleRate
		if n := len(spans); n > 0 && spans[n-1].Open == open {
			spans[n-1].EndMS = endMS // merge with the previous run
			continue
		}
		spans = append(spans, MouthSpan{StartMS: startMS, EndMS: endMS, Open: open})
	}
	return spans
}

// sqrt avoids pulling in math for a single call site.
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for range 12 {
		z = (z + x/z) / 2
	}
	return z
}

// Silence builds a mono 16-bit WAVE of the given length. The estimate engine
// uses it so that a project with no speech engine available still produces a
// timeline (and a renderable, silent video) of the right shape.
func Silence(ms, sampleRate int) []byte {
	samples := sampleRate * ms / 1000
	dataLen := samples * 2

	b := make([]byte, 44+dataLen)
	copy(b[0:4], "RIFF")
	binary.LittleEndian.PutUint32(b[4:8], uint32(36+dataLen))
	copy(b[8:12], "WAVE")
	copy(b[12:16], "fmt ")
	binary.LittleEndian.PutUint32(b[16:20], 16)
	binary.LittleEndian.PutUint16(b[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(b[22:24], 1) // mono
	binary.LittleEndian.PutUint32(b[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(b[28:32], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(b[32:34], 2)  // block align
	binary.LittleEndian.PutUint16(b[34:36], 16) // bits
	copy(b[36:40], "data")
	binary.LittleEndian.PutUint32(b[40:44], uint32(dataLen))
	return b
}
