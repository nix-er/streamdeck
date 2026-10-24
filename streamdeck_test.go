package streamdeck

import (
	"bytes"
	"image"
	"image/color"
	"reflect"
	"sync"
	"testing"
)

// plusDevice returns a Stream Deck+ device without a hardware connection.
func plusDevice() *Device {
	return newDevice(PID_STREAMDECK_PLUS)
}

// newDevice returns a device without a hardware connection.
func newDevice(pid uint16) *Device {
	d, ok := deviceForProductID(pid)
	if !ok {
		panic("unknown product ID")
	}
	return &d
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
	writes   [][]byte
	features [][]byte
}

func (f *fakeHID) Close() error                             { return nil }
func (f *fakeHID) Read(b []byte) (int, error)               { return 0, nil }
func (f *fakeHID) ReadTimeout(b []byte, _ int) (int, error) { return 0, nil }
func (f *fakeHID) GetFeatureReport(b []byte) (int, error)   { return len(b), nil }
func (f *fakeHID) SendFeatureReport(b []byte) (int, error) {
	f.features = append(f.features, append([]byte(nil), b...))
	return len(b), nil
}
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

func TestDeviceForProductID(t *testing.T) {
	tests := []struct {
		pid                 uint16
		columns, rows, keys uint8
		width, height       uint
		touchKeys, dials    uint8
		screen              [2]uint
		info                [2]uint
		dialRings           bool
	}{
		{pid: PID_STREAMDECK, columns: 5, rows: 3, keys: 15, width: 72, height: 72},
		{pid: PID_STREAMDECK_V2, columns: 5, rows: 3, keys: 15, width: 72, height: 72},
		{pid: PID_STREAMDECK_MK2, columns: 5, rows: 3, keys: 15, width: 72, height: 72},
		{pid: PID_STREAMDECK_MK2_SCISSOR, columns: 5, rows: 3, keys: 15, width: 72, height: 72},
		{pid: PID_STREAMDECK_MK2_MODULE, columns: 5, rows: 3, keys: 15, width: 72, height: 72},
		{pid: PID_STREAMDECK_MK2_MODULE_SCISSOR, columns: 5, rows: 3, keys: 15, width: 72, height: 72},
		{pid: PID_STREAMDECK_MINI, columns: 3, rows: 2, keys: 6, width: 80, height: 80},
		{pid: PID_STREAMDECK_MINI_MK2, columns: 3, rows: 2, keys: 6, width: 80, height: 80},
		{pid: PID_STREAMDECK_MINI_DISCORD, columns: 3, rows: 2, keys: 6, width: 80, height: 80},
		{pid: PID_STREAMDECK_MINI_MK2_MODULE, columns: 3, rows: 2, keys: 6, width: 80, height: 80},
		{pid: PID_STREAMDECK_XL, columns: 8, rows: 4, keys: 32, width: 96, height: 96},
		{pid: PID_STREAMDECK_XL_V2, columns: 8, rows: 4, keys: 32, width: 96, height: 96},
		{pid: PID_STREAMDECK_XL_V2_MODULE, columns: 8, rows: 4, keys: 32, width: 96, height: 96},
		{
			pid: PID_STREAMDECK_NEO, columns: 4, rows: 2, keys: 8, width: 96, height: 96,
			touchKeys: 2, info: [2]uint{248, 58},
		},
		{pid: PID_STREAMDECK_PEDAL, columns: 3, rows: 1, keys: 3},
		{
			pid: PID_STREAMDECK_PLUS, columns: 4, rows: 2, keys: 8, width: 120, height: 120,
			dials: 4, screen: [2]uint{800, 100},
		},
		{
			pid: PID_STREAMDECK_PLUS_XL, columns: 9, rows: 4, keys: 36, width: 112, height: 112,
			dials: 6, screen: [2]uint{1200, 100},
		},
		{
			pid: PID_STREAMDECK_STUDIO, columns: 16, rows: 2, keys: 32, width: 144, height: 112,
			dials: 2, dialRings: true,
		},
	}

	for _, tt := range tests {
		d, ok := deviceForProductID(tt.pid)
		if !ok {
			t.Errorf("%#04x: unknown product ID", tt.pid)
			continue
		}
		if d.Columns != tt.columns || d.Rows != tt.rows || d.Keys != tt.keys {
			t.Errorf("%#04x: got %dx%d with %d keys, want %dx%d with %d keys",
				tt.pid, d.Columns, d.Rows, d.Keys, tt.columns, tt.rows, tt.keys)
		}
		if d.KeyWidth != tt.width || d.KeyHeight != tt.height || d.Pixels != min(tt.width, tt.height) {
			t.Errorf("%#04x: got %dx%d key pixels (%d), want %dx%d",
				tt.pid, d.KeyWidth, d.KeyHeight, d.Pixels, tt.width, tt.height)
		}
		if d.TouchKeys != tt.touchKeys || d.Dials != tt.dials || d.DialRings != tt.dialRings {
			t.Errorf("%#04x: got %d touch keys, %d dials, rings %v", tt.pid, d.TouchKeys, d.Dials, d.DialRings)
		}
		if d.ScreenWidth != tt.screen[0] || d.ScreenHeight != tt.screen[1] {
			t.Errorf("%#04x: got %dx%d touch screen", tt.pid, d.ScreenWidth, d.ScreenHeight)
		}
		if tt.dials > 0 && tt.screen[0] > 0 && d.ScreenSegments != tt.dials {
			t.Errorf("%#04x: got %d screen segments, want %d", tt.pid, d.ScreenSegments, tt.dials)
		}
		if d.InfoScreenWidth != tt.info[0] || d.InfoScreenHeight != tt.info[1] {
			t.Errorf("%#04x: got %dx%d info screen", tt.pid, d.InfoScreenWidth, d.InfoScreenHeight)
		}
		if len(d.keyState) != int(tt.keys+tt.touchKeys) || len(d.dialState) != int(tt.dials) {
			t.Errorf("%#04x: got %d key and %d dial states", tt.pid, len(d.keyState), len(d.dialState))
		}
	}

	if _, ok := deviceForProductID(0xffff); ok {
		t.Error("unknown product ID accepted")
	}
	if d := newDevice(PID_STREAMDECK_MINI_MK2_MODULE); d.getFirmwareCommand[0] != 0xa1 {
		t.Errorf("Mini MK.2 Module uses firmware command % x", d.getFirmwareCommand)
	}
}

// keysReport builds an input report without a report type, with key states
// starting at byte 4.
func keysReport(size int, pressed ...int) []byte {
	buf := make([]byte, size)
	buf[0] = 0x01
	for _, i := range pressed {
		buf[4+i] = 1
	}
	return buf
}

func TestParseInputNeoTouchKeys(t *testing.T) {
	d := newDevice(PID_STREAMDECK_NEO)

	got := d.parseInput(keysReport(14, 9))
	want := []Event{Key{Index: 9, Pressed: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseInputPedal(t *testing.T) {
	d := newDevice(PID_STREAMDECK_PEDAL)

	got := d.parseInput(keysReport(7, 1))
	want := []Event{Key{Index: 1, Pressed: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseInputStudio(t *testing.T) {
	d := newDevice(PID_STREAMDECK_STUDIO)

	buf := make([]byte, 43)
	buf[0], buf[1] = 0x01, inputTypeKeys
	buf[4+31] = 1
	got := d.parseInput(buf)
	want := []Event{Key{Index: 31, Pressed: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keys: got %v, want %v", got, want)
	}

	buf = make([]byte, 43)
	buf[0], buf[1], buf[4], buf[6] = 0x01, inputTypeDials, dialSubtypeTurn, 0xfe
	got = d.parseInput(buf)
	want = []Event{DialTurn{Index: 1, Steps: -2}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dials: got %v, want %v", got, want)
	}
}

func TestParseInputPlusXL(t *testing.T) {
	d := newDevice(PID_STREAMDECK_PLUS_XL)

	buf := make([]byte, 64)
	buf[0], buf[1] = 0x01, inputTypeKeys
	buf[4+35] = 1
	got := d.parseInput(buf)
	want := []Event{Key{Index: 35, Pressed: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keys: got %v, want %v", got, want)
	}

	buf = make([]byte, 64)
	buf[0], buf[1], buf[4], buf[5+5] = 0x01, inputTypeDials, dialSubtypePress, 1
	got = d.parseInput(buf)
	want = []Event{DialPress{Index: 5, Pressed: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dials: got %v, want %v", got, want)
	}
}

func TestRotateCounterclockwise(t *testing.T) {
	// a 3x2 image, where every pixel's red channel encodes its position
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for y := range 2 {
		for x := range 3 {
			src.SetRGBA(x, y, color.RGBA{R: uint8(y*3 + x), A: 255})
		}
	}

	got := rotateCounterclockwise(src)
	if b := got.Bounds(); b.Dx() != 2 || b.Dy() != 3 {
		t.Fatalf("got %dx%d image, want 2x3", b.Dx(), b.Dy())
	}

	// rotated counterclockwise, the top-right pixel is now the top-left one:
	//  0 1 2      2 5
	//  3 4 5  ->  1 4
	//             0 3
	want := [][]uint8{{2, 5}, {1, 4}, {0, 3}}
	for y, row := range want {
		for x, v := range row {
			if r := got.(*image.RGBA).RGBAAt(x, y).R; r != v {
				t.Errorf("pixel %d,%d is %d, want %d", x, y, r, v)
			}
		}
	}
}

func TestSetScreenImagePlusXL(t *testing.T) {
	d := newDevice(PID_STREAMDECK_PLUS_XL)
	f := &fakeHID{}
	d.device = f

	if err := d.SetScreenSegmentImage(2, image.NewRGBA(image.Rect(0, 0, 200, 100))); err != nil {
		t.Fatal(err)
	}

	// the region is sent with swapped axes: x = 0, y = 400, 100x200 pixels
	w := f.writes[0]
	want := []byte{0x02, 0x0c, 0x00, 0x00, 0x90, 0x01, 0x64, 0x00, 0xc8, 0x00}
	if !bytes.Equal(w[:10], want) {
		t.Errorf("got header % x, want % x", w[:10], want)
	}
}

func TestSetInfoScreenImage(t *testing.T) {
	d := newDevice(PID_STREAMDECK_NEO)
	f := &fakeHID{}
	d.device = f

	if err := d.SetInfoScreenImage(image.NewRGBA(image.Rect(0, 0, 248, 58))); err != nil {
		t.Fatal(err)
	}
	for i, w := range f.writes {
		last := byte(0)
		if i == len(f.writes)-1 {
			last = 1
		}
		if len(w) != 1024 || w[0] != 0x02 || w[1] != 0x0b || w[2] != 0 || w[3] != last ||
			int(w[6])|int(w[7])<<8 != i {
			t.Errorf("report %d has header % x", i, w[:8])
		}
	}

	if err := d.SetInfoScreenImage(image.NewRGBA(image.Rect(0, 0, 96, 96))); err == nil {
		t.Error("expected error for wrong image size")
	}
	if err := plusDevice().SetInfoScreenImage(image.NewRGBA(image.Rect(0, 0, 248, 58))); err == nil {
		t.Error("expected error for device without info screen")
	}
}

func TestSetImageNonSquareKeys(t *testing.T) {
	d := newDevice(PID_STREAMDECK_STUDIO)
	d.device = &fakeHID{}

	if err := d.SetImage(0, image.NewRGBA(image.Rect(0, 0, 144, 112))); err != nil {
		t.Error(err)
	}
	if err := d.SetImage(0, image.NewRGBA(image.Rect(0, 0, 112, 112))); err == nil {
		t.Error("expected error for wrong image size")
	}
	if err := d.SetImage(32, image.NewRGBA(image.Rect(0, 0, 144, 112))); err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestPedalHasNoDisplay(t *testing.T) {
	d := newDevice(PID_STREAMDECK_PEDAL)
	f := &fakeHID{}
	d.device = f

	if err := d.SetImage(0, image.NewRGBA(image.Rect(0, 0, 72, 72))); err != nil {
		t.Error(err)
	}
	if err := d.Clear(); err != nil {
		t.Error(err)
	}
	if err := d.Reset(); err != nil {
		t.Error(err)
	}
	if err := d.SetBrightness(50); err != nil {
		t.Error(err)
	}
	if len(f.writes) > 0 {
		t.Errorf("%d reports written to a device without displays", len(f.writes))
	}
}

func TestSetKeyColor(t *testing.T) {
	d := newDevice(PID_STREAMDECK_NEO)
	f := &fakeHID{}
	d.device = f

	if err := d.SetKeyColor(8, color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff}); err != nil {
		t.Error(err)
	}
	want := []byte{0x03, 0x06, 0x08, 0x11, 0x22, 0x33}
	if len(f.features) != 1 || !bytes.Equal(f.features[0][:6], want) {
		t.Errorf("got feature reports % x, want % x", f.features, want)
	}

	if err := d.SetKeyColor(7, color.Black); err == nil {
		t.Error("expected error for a key that is no touch key")
	}
	if err := plusDevice().SetKeyColor(8, color.Black); err == nil {
		t.Error("expected error for device without touch keys")
	}
}

func TestDialRings(t *testing.T) {
	d := newDevice(PID_STREAMDECK_STUDIO)
	f := &fakeHID{}
	d.device = f

	red := color.RGBA{R: 0xff, A: 0xff}
	if err := d.SetDialRingPercentage(1, red, 50); err != nil {
		t.Fatal(err)
	}
	w := f.writes[0]
	if len(w) != 1024 || !bytes.Equal(w[:3], []byte{0x02, 0x0f, 0x01}) {
		t.Fatalf("got report % x", w[:3])
	}
	// half of the ring is lit, starting at the right dial's offset of 12
	for led := range dialRingLEDs {
		lit := led >= 12
		if (w[3+led*3] == 0xff) != lit {
			t.Errorf("LED %d lit: %v, want %v", led, !lit, lit)
		}
	}

	if err := d.SetDialKnobColor(0, red); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f.writes[1][:6], []byte{0x02, 0x10, 0x00, 0xff, 0x00, 0x00}) {
		t.Errorf("got knob report % x", f.writes[1][:6])
	}

	if err := d.SetDialRingColor(2, red); err == nil {
		t.Error("expected error for invalid dial")
	}
	if err := plusDevice().SetDialRingColor(0, red); err == nil {
		t.Error("expected error for device without dial rings")
	}
}

func TestInputEventsWhileAsleep(t *testing.T) {
	d := plusDevice()

	// the key whose hold action puts the device to sleep is still down
	if _, wake := d.inputEvents(report(inputTypeKeys, 1)); wake {
		t.Fatal("awake device must not wake")
	}
	d.asleep = true

	steps := []struct {
		name string
		buf  []byte
		wake bool
	}{
		{"release of the held key", report(inputTypeKeys), false},
		{"dial release", report(inputTypeDials, dialSubtypePress), false},
		{"key press", report(inputTypeKeys, 0, 0, 1), true},
		{"dial press", report(inputTypeDials, dialSubtypePress, 1), true},
		{"dial turn", report(inputTypeDials, dialSubtypeTurn, 1), true},
		{"touch tap", report(inputTypeTouch, 1, 0, 0xc2, 0x01, 50, 0), true},
	}
	for _, s := range steps {
		events, wake := d.inputEvents(s.buf)
		if events != nil {
			t.Errorf("%s: got events %#v while asleep", s.name, events)
		}
		if wake != s.wake {
			t.Errorf("%s: wake = %v, want %v", s.name, wake, s.wake)
		}
	}

	// the key that woke the device is still down; its release is reported
	d.asleep = false
	d.inputEvents(report(inputTypeKeys, 0, 0, 1))
	events, _ := d.inputEvents(report(inputTypeKeys))
	want := []Event{Key{Index: 2, Pressed: false}}
	if !reflect.DeepEqual(events, want) {
		t.Errorf("release after wake: got %#v, want %#v", events, want)
	}
}

// Putting a sleeping device to sleep again must not lose the brightness to
// restore on wake-up.
func TestSleepWhileAsleep(t *testing.T) {
	d := plusDevice()
	d.sleepMutex = &sync.RWMutex{}
	d.asleep = true
	d.brightness = 0
	d.preSleepBrightness = 80

	if err := d.Sleep(); err != nil {
		t.Fatal(err)
	}
	if d.preSleepBrightness != 80 {
		t.Errorf("preSleepBrightness = %d, want 80", d.preSleepBrightness)
	}
}
