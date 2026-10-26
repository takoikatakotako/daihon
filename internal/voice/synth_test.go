package voice

import "testing"

func TestCacheKeyIncludesVoiceName(t *testing.T) {
	base := Options{Speed: 1.0, VoiceName: "Kyoko"}
	other := Options{Speed: 1.0, VoiceName: "Otoya"}
	if cacheKey("say", "macos-say-15", "こんにちは", base) == cacheKey("say", "macos-say-15", "こんにちは", other) {
		t.Fatal("different macOS voices must not reuse cached speech")
	}
}
