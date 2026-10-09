package wav

import "testing"

func TestSilenceRoundTrip(t *testing.T) {
	for _, ms := range []int{0, 1, 250, 1000, 3333} {
		b := Silence(ms, 24000)
		got, err := DurationMS(b)
		if err != nil {
			t.Fatalf("%dms: decode: %v", ms, err)
		}
		if got != ms {
			t.Errorf("Silence(%d) round-tripped as %dms", ms, got)
		}
	}
}

func TestSilenceHasClosedMouth(t *testing.T) {
	f, err := Decode(Silence(1000, 24000))
	if err != nil {
		t.Fatal(err)
	}
	spans := f.Mouth(30, DefaultMouthRatio)
	if len(spans) != 1 {
		t.Fatalf("silence should collapse to one span, got %d", len(spans))
	}
	if spans[0].Open {
		t.Error("silence should not open the mouth")
	}
	if spans[0].StartMS != 0 || spans[0].EndMS != 1000 {
		t.Errorf("span = %+v, want 0..1000", spans[0])
	}
}

func TestMouthOpensOnLoudAudio(t *testing.T) {
	const rate = 24000
	b := Silence(1000, rate)
	// Fill the second half with a loud square wave.
	for i := rate / 2; i < rate; i++ {
		off := 44 + i*2
		v := int16(12000)
		if i%40 < 20 {
			v = -12000
		}
		b[off] = byte(uint16(v))
		b[off+1] = byte(uint16(v) >> 8)
	}
	f, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	spans := f.Mouth(30, DefaultMouthRatio)
	if len(spans) != 2 {
		t.Fatalf("want a closed then an open span, got %d: %+v", len(spans), spans)
	}
	if spans[0].Open || !spans[1].Open {
		t.Errorf("spans = %+v", spans)
	}
	if spans[1].StartMS != 500 {
		t.Errorf("mouth should open at 500ms, got %d", spans[1].StartMS)
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	if _, err := Decode([]byte("not a wave file at all")); err == nil {
		t.Error("expected an error")
	}
}
