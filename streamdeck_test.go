package streamdeck

import (
	"bytes"
	"image"
	"reflect"
	"testing"
)

// plusDevice returns a Stream Deck+ device without a hardware connection.
func plusDevice() *Device {
	return &Device{
		Columns:              4,
		Rows:                 2,
		Keys:                 8,
		Pixels:               120,
		Dials:                4,
		ScreenWidth:          800,
		ScreenHeight:         100,
		ScreenSegments:       4,
		keyStateOffset:       4,
		reportTypeOffset:     1,
		translateKeyIndex:    identity,
		screenPageSize:       1024,
		screenPageHeaderSize: 16,
		keyState:             make([]byte, 8),
		dialState:            make([]byte, 4),
	}
}

// report builds a 14 byte Stream Deck+ input report of the given type, with
// data starting at byte 4.
func report(inputType byte, data ...byte) []byte {
	buf := make([]byte, touchReportSize)
	buf[0] = 0x01
	buf[1] = inputType
	copy(buf[4:], data)
	return buf
}

func TestParseInputPlus(t *testing.T) {
	d := plusDevice()

	steps := []struct {
		name string
		buf  []byte
		want []Event
	}{
		{"key down", report(inputTypeKeys, 0, 1, 0, 0, 0, 0, 0, 0), []Event{Key{Index: 1, Pressed: true}}},
		{"key unchanged", report(inputTypeKeys, 0, 1, 0, 0, 0, 0, 0, 0), nil},
		{"key up", report(inputTypeKeys), []Event{Key{Index: 1, Pressed: false}}},
		{"dial press", report(inputTypeDials, dialSubtypePress, 0, 0, 1, 0), []Event{DialPress{Index: 2, Pressed: true}}},
		{"dial turn while pressed", report(inputTypeDials, dialSubtypeTurn, 2, 0xff, 0, 0),
			[]Event{DialTurn{Index: 0, Steps: 2}, DialTurn{Index: 1, Steps: -1}}},
		{"dial release", report(inputTypeDials, dialSubtypePress), []Event{DialPress{Index: 2, Pressed: false}}},
		{"touch tap", report(inputTypeTouch, 1, 0, 0xc2, 0x01, 50, 0),
			[]Event{Touch{Type: TouchTap, X: 450, Y: 50}}},
		{"touch long press", report(inputTypeTouch, 2, 0, 10, 0, 20, 0),
			[]Event{Touch{Type: TouchLongPress, X: 10, Y: 20}}},
		{"touch swipe", report(inputTypeTouch, 3, 0, 0x58, 0x02, 40, 0, 100, 0, 45, 0),
			[]Event{Touch{Type: TouchSwipe, X: 600, Y: 40, EndX: 100, EndY: 45}}},
		{"unknown touch type", report(inputTypeTouch, 9), nil},
		{"unknown report type", report(0x7f, 1, 1, 1, 1), nil},
	}

	for _, s := range steps {
		got := d.parseInput(s.buf)
		if !reflect.DeepEqual(got, s.want) {
			t.Errorf("%s: got %#v, want %#v", s.name, got, s.want)
		}
	}
}

func TestParseInputShortReport(t *testing.T) {
	d := plusDevice()
	if got := d.parseInput([]byte{0x01, inputTypeTouch, 0, 0, 1}); got != nil {
		t.Errorf("got %#v for a truncated report, want no events", got)
	}
}

func TestParseInputWithoutReportType(t *testing.T) {
	// e.g. Stream Deck XL: every input report is a key report
	d := &Device{
		Columns:           8,
		Rows:              4,
		keyStateOffset:    4,
		translateKeyIndex: identity,
		keyState:          make([]byte, 32),
	}

	buf := make([]byte, 4+32)
	buf[1] = inputTypeDials // must not be interpreted as a report type
	buf[4+5] = 1

	got := d.parseInput(buf)
	want := []Event{Key{Index: 5, Pressed: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestScreenImagePageHeader(t *testing.T) {
	got := screenImagePageHeader(1, 200, 0, 200, 100, 1008, true)
	want := []byte{
		0x02, 0x0c,
		0xc8, 0x00, // x = 200
		0x00, 0x00, // y = 0
		0xc8, 0x00, // width = 200
		0x64, 0x00, // height = 100
		0x01,       // last page
		0x01, 0x00, // page 1
		0xf0, 0x03, // payload length 1008
		0x00,
	}
	if !bytes.Equal(got, want) {
		t.Errorf("got % x, want % x", got, want)
	}
}

// fakeHID records the reports written to it.
type fakeHID struct {
	writes [][]byte
}

func (f *fakeHID) Close() error                             { return nil }
func (f *fakeHID) Read(b []byte) (int, error)               { return 0, nil }
func (f *fakeHID) ReadTimeout(b []byte, _ int) (int, error) { return 0, nil }
func (f *fakeHID) GetFeatureReport(b []byte) (int, error)   { return len(b), nil }
func (f *fakeHID) SendFeatureReport(b []byte) (int, error)  { return len(b), nil }
func (f *fakeHID) Write(b []byte) (int, error) {
	f.writes = append(f.writes, append([]byte(nil), b...))
	return len(b), nil
}

func TestSetScreenSegmentImage(t *testing.T) {
	d := plusDevice()
	f := &fakeHID{}
	d.device = f

	if err := d.SetScreenSegmentImage(2, image.NewRGBA(image.Rect(0, 0, 200, 100))); err != nil {
		t.Fatal(err)
	}
	if len(f.writes) == 0 {
		t.Fatal("no reports written")
	}

	var payload []byte
	for i, w := range f.writes {
		if len(w) != 1024 {
			t.Fatalf("report %d has %d bytes, want 1024", i, len(w))
		}
		if w[0] != 0x02 || w[1] != 0x0c {
			t.Fatalf("report %d has command % x, want 02 0c", i, w[:2])
		}
		if x := int(w[2]) | int(w[3])<<8; x != 400 {
			t.Errorf("report %d has x = %d, want 400", i, x)
		}
		if page := int(w[11]) | int(w[12])<<8; page != i {
			t.Errorf("report %d has page %d", i, page)
		}
		last := i == len(f.writes)-1
		if (w[10] == 1) != last {
			t.Errorf("report %d has last page flag %d", i, w[10])
		}
		length := int(w[13]) | int(w[14])<<8
		payload = append(payload, w[16:16+length]...)
	}

	// the payload must be a complete JPEG image
	if !bytes.HasPrefix(payload, []byte{0xff, 0xd8}) || !bytes.HasSuffix(payload, []byte{0xff, 0xd9}) {
		t.Error("payload is not a complete JPEG image")
	}
}

func TestSetScreenImageErrors(t *testing.T) {
	d := plusDevice()
	d.device = &fakeHID{}

	if err := d.SetScreenSegmentImage(4, image.NewRGBA(image.Rect(0, 0, 200, 100))); err == nil {
		t.Error("expected error for invalid segment")
	}
	if err := d.SetScreenSegmentImage(0, image.NewRGBA(image.Rect(0, 0, 120, 120))); err == nil {
		t.Error("expected error for wrong segment image size")
	}
	if err := d.SetScreenImage(700, 0, image.NewRGBA(image.Rect(0, 0, 200, 100))); err == nil {
		t.Error("expected error for image exceeding the screen")
	}

	noScreen := &Device{}
	if err := noScreen.SetScreenImage(0, 0, image.NewRGBA(image.Rect(0, 0, 10, 10))); err == nil {
		t.Error("expected error for device without screen")
	}
}
