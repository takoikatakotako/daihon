// Package voicevox synthesises speech with a VOICEVOX ENGINE over HTTP.
//
// The engine API is public and documented at
// https://voicevox.github.io/voicevox_engine/api/ . Synthesis takes two calls:
// /audio_query turns text into an editable query (accents, timings, speed),
// and /synthesis turns that query into a WAVE.
package voicevox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/takoikatakotako/daihon/internal/voice"
)

// DefaultBaseURL is where an engine listens unless told otherwise.
const DefaultBaseURL = "http://127.0.0.1:50021"

// Engine is a voice.Engine backed by a running VOICEVOX ENGINE.
type Engine struct {
	base   string
	client *http.Client
}

// New returns an engine talking to baseURL, or DefaultBaseURL when empty.
func New(baseURL string) *Engine {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Engine{
		base: strings.TrimRight(baseURL, "/"),
		// Long lines take a few seconds on CPU; this only guards against a hung engine.
		client: &http.Client{Timeout: 2 * time.Minute},
	}
}

func (e *Engine) ID() string { return "voicevox" }

// Version reports the engine version, e.g. "0.25.2".
func (e *Engine) Version(ctx context.Context) (string, error) {
	body, err := e.do(ctx, http.MethodGet, "/version", nil, nil)
	if err != nil {
		return "", err
	}
	var v string
	if err := json.Unmarshal(body, &v); err != nil {
		return "", fmt.Errorf("VOICEVOX ENGINE の /version を読めません: %w", err)
	}
	return v, nil
}

// Synth returns a 16-bit PCM WAVE of text read by o.SpeakerID.
func (e *Engine) Synth(ctx context.Context, text string, o voice.Options) ([]byte, error) {
	speaker := url.Values{"speaker": {strconv.Itoa(o.SpeakerID)}}

	q := url.Values{"text": {text}, "speaker": speaker["speaker"]}
	raw, err := e.do(ctx, http.MethodPost, "/audio_query", q, nil)
	if err != nil {
		return nil, err
	}

	// Keep the query as a generic map so fields added by newer engines pass
	// through untouched; only the scales we control are overwritten.
	var query map[string]any
	if err := json.Unmarshal(raw, &query); err != nil {
		return nil, fmt.Errorf("VOICEVOX ENGINE の audio_query を読めません: %w", err)
	}
	speed := o.Speed
	if speed == 0 {
		speed = 1
	}
	query["speedScale"] = speed
	query["pitchScale"] = o.Pitch

	payload, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	return e.do(ctx, http.MethodPost, "/synthesis", speaker, payload)
}

// do sends one request and returns the body of a 2xx response.
func (e *Engine) do(ctx context.Context, method, path string, q url.Values, payload []byte) ([]byte, error) {
	u := e.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VOICEVOX ENGINE (%s) に接続できません: %w", e.base, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		msg := strings.TrimSpace(string(b))
		if len(msg) > 200 {
			msg = msg[:200] + "…"
		}
		return nil, fmt.Errorf("VOICEVOX ENGINE %s %s: %s %s", method, path, resp.Status, msg)
	}
	return b, nil
}
