package gemini

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/takoikatakotako/daihon/internal/voice"
	"github.com/takoikatakotako/daihon/internal/wav"
	"golang.org/x/oauth2"
)

func fakeEngine(server *httptest.Server) *Engine {
	e := New("test-project")
	e.Endpoint = server.URL
	e.Client = server.Client()
	e.TokenSource = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})
	return e
}

func testAudio(t *testing.T) []byte {
	t.Helper()
	pcm := make([]byte, 4800)
	pcm[100] = 42
	b, err := wav.PCM16(pcm, 24000)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSynthRequestUsesVerbatimReadAndSeparateStyle(t *testing.T) {
	audio := testAudio(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/test-project/locations/global/publishers/google/models/gemini-3.8-flash-lite-tts:generateContent" {
			t.Errorf("path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("authorization missing")
		}
		var body struct {
			Contents []struct {
				Parts []struct {
					Text           string `json:"text"`
					SpeechMetadata struct {
						Style string `json:"style"`
					} `json:"speechMetadata"`
				} `json:"parts"`
			} `json:"contents"`
			GenerationConfig struct {
				ResponseModalities []string `json:"responseModalities"`
				SpeechConfig       struct {
					LanguageCode string `json:"languageCode"`
					VoiceConfig  struct {
						Voice string `json:"voice"`
					} `json:"voiceConfig"`
				} `json:"speechConfig"`
			} `json:"generationConfig"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if len(body.Contents) != 1 || len(body.Contents[0].Parts) != 1 {
			t.Fatalf("unexpected parts: %+v", body.Contents)
		}
		part := body.Contents[0].Parts[0]
		if part.Text != "シー" || part.SpeechMetadata.Style != "明るく、ゆっくり" {
			t.Errorf("text/style: %+v", part)
		}
		if body.GenerationConfig.SpeechConfig.LanguageCode != "ja-JP" || body.GenerationConfig.SpeechConfig.VoiceConfig.Voice != "Kore" {
			t.Errorf("speech config: %+v", body.GenerationConfig.SpeechConfig)
		}
		if len(body.GenerationConfig.ResponseModalities) != 1 || body.GenerationConfig.ResponseModalities[0] != "AUDIO" {
			t.Errorf("modalities: %v", body.GenerationConfig.ResponseModalities)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"inlineData": map[string]any{"mimeType": "audio/wav", "data": audio}}}}}}})
	}))
	defer server.Close()
	b, err := fakeEngine(server).Synth(context.Background(), "シー", voice.Options{Model: "gemini-3.8-flash-lite-tts", VoiceName: "Kore", Language: "ja-JP", Style: "明るく、ゆっくり", Speed: 1})
	if err != nil {
		t.Fatal(err)
	}
	d, err := wav.DurationMS(b)
	if err != nil || d != 100 {
		t.Fatalf("WAV duration=%d err=%v", d, err)
	}
}

func TestSynthReportsAPIErrorsAndEmptyAudio(t *testing.T) {
	silent, err := wav.PCM16(make([]byte, 4800), 24000)
	if err != nil {
		t.Fatal(err)
	}
	silentResponse, err := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"inlineData": map[string]any{"mimeType": "audio/wav", "data": silent}}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	wrongRate := testAudio(t)
	binary.LittleEndian.PutUint32(wrongRate[24:28], 16000)
	stereo := testAudio(t)
	binary.LittleEndian.PutUint16(stereo[22:24], 2)
	response := func(mimeType string, audio []byte) string {
		b, err := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"inlineData": map[string]any{"mimeType": mimeType, "data": audio}}}}}}})
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"api", 403, `{"error":{"status":"PERMISSION_DENIED","message":"Vertex AI access denied"}}`, "Vertex AI access denied"},
		{"empty", 200, `{"candidates":[{"finishReason":"SAFETY","content":{"parts":[]}}]}`, "音声を返しません"},
		{"invalid", 200, `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"audio/wav","data":"YmFk"}}]}}]}`, "WAV が不正"},
		{"silent", 200, string(silentResponse), "空音声"},
		{"wav rate", 200, response("audio/wav", wrongRate), "24000Hz"},
		{"stereo", 200, response("audio/wav", stereo), "モノラル"},
		{"pcm rate", 200, response("audio/L16;codec=pcm;rate=16000", testAudio(t)[44:]), "24000Hz"},
		{"pcm odd bytes", 200, response("audio/L16;codec=pcm;rate=24000", testAudio(t)[44:199]), "PCM が不正"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			_, err := fakeEngine(server).Synth(context.Background(), "こんにちは", voice.Options{Speed: 1})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestSynthWrapsRawPCM(t *testing.T) {
	pcm := testAudio(t)[44:]
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"inlineData": map[string]any{"mimeType": "audio/L16;codec=pcm;rate=24000", "data": pcm}}}}}}})
	}))
	defer server.Close()
	b, err := fakeEngine(server).Synth(context.Background(), "こんにちは", voice.Options{Speed: 1})
	if err != nil {
		t.Fatal(err)
	}
	f, err := wav.Decode(b)
	if err != nil || f.Format.SampleRate != 24000 || f.Format.Channels != 1 || string(f.Data) != string(pcm) {
		t.Fatalf("PCM was not preserved in 24kHz mono WAV: %v, %v", f, err)
	}
}

type countingSource struct{ calls atomic.Int32 }

func (s *countingSource) Token() (*oauth2.Token, error) {
	s.calls.Add(1)
	return &oauth2.Token{AccessToken: "test-token", Expiry: time.Now().Add(time.Hour)}, nil
}

func TestCredentialsReuseTokenAndReportMissingProject(t *testing.T) {
	pcm := testAudio(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"inlineData": map[string]any{"mimeType": "audio/wav", "data": pcm}}}}}}})
	}))
	defer server.Close()
	e := fakeEngine(server)
	source := &countingSource{}
	e.TokenSource = source
	for range 2 {
		if _, err := e.Synth(context.Background(), "こんにちは", voice.Options{Speed: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if got := source.calls.Load(); got != 1 {
		t.Fatalf("token requests = %d, want 1", got)
	}
	missing := New("")
	missing.TokenSource = source
	if _, _, err := missing.credentials(context.Background()); err == nil || !strings.Contains(err.Error(), "project") {
		t.Fatalf("injected token without project: %v", err)
	}
}

func TestAPIErrorTruncatesAtRuneBoundary(t *testing.T) {
	message := strings.Repeat("あ", 301)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": message}})
	}))
	defer server.Close()
	_, err := fakeEngine(server).Synth(context.Background(), "こんにちは", voice.Options{Speed: 1})
	if err == nil || !utf8.ValidString(err.Error()) || !strings.Contains(err.Error(), strings.Repeat("あ", 300)+"…") {
		t.Fatalf("rune truncation: %v", err)
	}
}

func TestSynthRejectsUnsupportedModelBeforeRequest(t *testing.T) {
	_, err := New("project").Synth(context.Background(), "こんにちは", voice.Options{Model: "gemini-pro", Speed: 1})
	if err == nil || !strings.Contains(err.Error(), "未対応") {
		t.Fatal(err)
	}
}
