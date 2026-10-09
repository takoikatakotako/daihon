package voicevox

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takoikatakotako/daihon/internal/voice"
)

// fakeEngine answers the three endpoints daihon uses, recording what the
// synthesis call received.
func fakeEngine(t *testing.T, got *map[string]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `"0.25.2"`)
	})
	mux.HandleFunc("POST /audio_query", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("text") != "こんにちは" || r.URL.Query().Get("speaker") != "3" {
			http.Error(w, "bad query "+r.URL.RawQuery, http.StatusUnprocessableEntity)
			return
		}
		io.WriteString(w, `{"accent_phrases":[],"speedScale":1.0,"pitchScale":0.0,"outputSamplingRate":24000}`)
	})
	mux.HandleFunc("POST /synthesis", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("speaker") != "3" {
			http.Error(w, "no speaker", http.StatusUnprocessableEntity)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(got); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		io.WriteString(w, "RIFF....WAVE")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestSynthAppliesSpeedAndPitch(t *testing.T) {
	var got map[string]any
	srv := fakeEngine(t, &got)
	e := New(srv.URL + "/")

	wav, err := e.Synth(context.Background(), "こんにちは", voice.Options{SpeakerID: 3, Speed: 1.2, Pitch: 0.05})
	if err != nil {
		t.Fatal(err)
	}
	if string(wav) != "RIFF....WAVE" {
		t.Errorf("wav = %q", wav)
	}
	if got["speedScale"] != 1.2 || got["pitchScale"] != 0.05 {
		t.Errorf("scales not applied: %v", got)
	}
	// Fields daihon does not touch must reach /synthesis unchanged.
	if got["outputSamplingRate"] != 24000.0 {
		t.Errorf("query not passed through: %v", got)
	}
}

func TestVersion(t *testing.T) {
	srv := fakeEngine(t, new(map[string]any))
	v, err := New(srv.URL).Version(context.Background())
	if err != nil || v != "0.25.2" {
		t.Errorf("Version() = %q, %v", v, err)
	}
}

func TestErrorStatusIsReported(t *testing.T) {
	srv := fakeEngine(t, new(map[string]any))
	_, err := New(srv.URL).Synth(context.Background(), "別の文", voice.Options{SpeakerID: 3})
	if err == nil {
		t.Fatal("want an error for a 422 response")
	}
}
