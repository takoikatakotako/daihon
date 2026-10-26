package voice

import "testing"

func TestCacheKeyIncludesVoiceName(t *testing.T) {
	base := Options{Speed: 1.0, VoiceName: "Kyoko"}
	other := Options{Speed: 1.0, VoiceName: "Otoya"}
	if cacheKey("say", "macos-say-15", "こんにちは", base) == cacheKey("say", "macos-say-15", "こんにちは", other) {
		t.Fatal("different macOS voices must not reuse cached speech")
	}
}

func TestGeminiCacheKeyInputs(t *testing.T) {
	base := Options{Model: "gemini-3.8-flash-tts", VoiceName: "Kore", Language: "ja-JP", Style: "明るく", Speed: 1}
	key := cacheKey("gemini", "v1", "今日は", base)
	variants := []Options{
		{Model: "gemini-3.8-flash-lite-tts", VoiceName: base.VoiceName, Language: base.Language, Style: base.Style, Speed: 1},
		{Model: base.Model, VoiceName: "Aoede", Language: base.Language, Style: base.Style, Speed: 1},
		{Model: base.Model, VoiceName: base.VoiceName, Language: "en-US", Style: base.Style, Speed: 1},
		{Model: base.Model, VoiceName: base.VoiceName, Language: base.Language, Style: "静かに", Speed: 1},
	}
	for _, o := range variants {
		if cacheKey("gemini", "v1", "今日は", o) == key {
			t.Errorf("cache collision: %+v", o)
		}
	}
	if cacheKey("gemini", "v1", "明日は", base) == key {
		t.Fatal("read change reused cache")
	}
}
