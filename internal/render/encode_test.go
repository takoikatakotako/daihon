package render

import (
	"slices"
	"testing"
)

func TestQualityArgsCRF(t *testing.T) {
	for _, tc := range []struct {
		crf  int
		want string
	}{
		{0, "18"}, // unset falls back to the default
		{28, "28"},
	} {
		args := qualityArgs(encodeOptions{VideoEncoder: "libx264", CRF: tc.crf})
		i := slices.Index(args, "-crf")
		if i < 0 || args[i+1] != tc.want {
			t.Errorf("CRF %d: got %v, want -crf %s", tc.crf, args, tc.want)
		}
	}
}
