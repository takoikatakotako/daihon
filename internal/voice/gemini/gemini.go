// Package gemini synthesizes speech through Vertex AI's Gemini TTS API.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/takoikatakotako/daihon/internal/voice"
	"github.com/takoikatakotako/daihon/internal/wav"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	defaultModel = "gemini-3.8-flash-tts"
	defaultVoice = "Kore"
	endpoint     = "https://aiplatform.googleapis.com"
	scope        = "https://www.googleapis.com/auth/cloud-platform"
)

// Engine uses Application Default Credentials at synthesis time. A cache hit
// never needs a token or a network connection.
type Engine struct {
	Project     string
	Endpoint    string
	Client      *http.Client
	TokenSource oauth2.TokenSource
}

func New(project string) *Engine {
	return &Engine{Project: project, Endpoint: endpoint, Client: &http.Client{Timeout: 2 * time.Minute}}
}

func (e *Engine) ID() string { return "gemini" }

func (e *Engine) Version(context.Context) (string, error) {
	return "vertex-gemini-tts-v1", nil
}

func (e *Engine) Synth(ctx context.Context, text string, o voice.Options) ([]byte, error) {
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("Gemini TTS: 読み上げテキストが空です")
	}
	model := o.Model
	if model == "" {
		model = defaultModel
	}
	if model != defaultModel && model != "gemini-3.8-flash-lite-tts" {
		return nil, fmt.Errorf("Gemini TTS: 未対応の model %q です", model)
	}
	if o.Speed != 0 && o.Speed != 1 || o.Pitch != 0 {
		return nil, errors.New("Gemini TTS: speed と pitch は対応していません。読み上げ速度は voice.style で指定してください")
	}
	voiceName := o.VoiceName
	if voiceName == "" {
		voiceName = defaultVoice
	}
	project, tokens, err := e.credentials(ctx)
	if err != nil {
		return nil, err
	}
	part := map[string]any{"text": text}
	if o.Style != "" {
		part["speechMetadata"] = map[string]string{"style": o.Style}
	}
	speech := map[string]any{"voiceConfig": map[string]string{"voice": voiceName}}
	if o.Language != "" {
		speech["languageCode"] = o.Language
	}
	payload, err := json.Marshal(map[string]any{
		"contents": []any{map[string]any{"role": "user", "parts": []any{part}}},
		"generationConfig": map[string]any{
			"responseModalities": []string{"AUDIO"},
			"speechConfig":       speech,
		},
	})
	if err != nil {
		return nil, err
	}
	base := e.Endpoint
	if base == "" {
		base = endpoint
	}
	u := strings.TrimRight(base, "/") + "/v1/projects/" + url.PathEscape(project) +
		"/locations/global/publishers/google/models/" + model + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	token, err := tokens.Token()
	if err != nil {
		return nil, fmt.Errorf("Gemini TTS: ADC のアクセストークンを取得できません: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	client := e.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Gemini TTS API への接続に失敗しました: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("Gemini TTS API の応答を読めません: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &apiErr)
		msg := apiErr.Error.Message
		if msg == "" {
			msg = strings.TrimSpace(string(body))
		}
		if len(msg) > 300 {
			msg = msg[:300] + "…"
		}
		return nil, fmt.Errorf("Gemini TTS API HTTP %d (%s): %s", resp.StatusCode, apiErr.Error.Status, msg)
	}
	var result struct {
		Candidates []struct {
			FinishReason string `json:"finishReason"`
			Content      struct {
				Parts []struct {
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     []byte `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		PromptFeedback json.RawMessage `json:"promptFeedback"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("Gemini TTS API の JSON が不正です: %w", err)
	}
	for _, candidate := range result.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData == nil || len(part.InlineData.Data) == 0 {
				continue
			}
			if part.InlineData.MimeType != "" && !strings.HasPrefix(part.InlineData.MimeType, "audio/wav") {
				return nil, fmt.Errorf("Gemini TTS API の音声形式 %q は未対応です（WAV を要求してください）", part.InlineData.MimeType)
			}
			f, err := wav.Decode(part.InlineData.Data)
			if err != nil {
				return nil, fmt.Errorf("Gemini TTS API の WAV が不正です: %w", err)
			}
			if f.Format.Channels != 1 || f.DurationMS() == 0 || !hasSignal(f.Data) {
				return nil, errors.New("Gemini TTS API が空音声を返しました")
			}
			return wav.PCM16(f.Data, f.Format.SampleRate)
		}
	}
	reason := ""
	if len(result.Candidates) > 0 {
		reason = result.Candidates[0].FinishReason
	}
	return nil, fmt.Errorf("Gemini TTS API が音声を返しませんでした（finishReason=%q, promptFeedback=%s）", reason, string(result.PromptFeedback))
}

func (e *Engine) credentials(ctx context.Context) (string, oauth2.TokenSource, error) {
	if e.TokenSource != nil && e.Project != "" {
		return e.Project, e.TokenSource, nil
	}
	creds, err := google.FindDefaultCredentials(ctx, scope)
	if err != nil {
		return "", nil, fmt.Errorf("Gemini TTS: Application Default Credentials を取得できません: %w", err)
	}
	project := e.Project
	if project == "" {
		project = creds.ProjectID
	}
	if project == "" {
		return "", nil, errors.New("Gemini TTS: Google Cloud の project が不明です。GOOGLE_CLOUD_PROJECT を設定してください")
	}
	return project, creds.TokenSource, nil
}

func hasSignal(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return true
		}
	}
	return false
}
