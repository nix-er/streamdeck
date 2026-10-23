// Package streamdeck controls Elgato Stream Deck devices.
package streamdeck

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/karalabe/hid"
	"golang.org/x/image/draw"
)

const (
	// 30 fps fade animation.
	fadeDelay = time.Second / 30
)

// Stream Deck Vendor & Product IDs.
//
//nolint:revive
const (
	VID_ELGATO              = 0x0fd9
	PID_STREAMDECK          = 0x0060
	PID_STREAMDECK_V2       = 0x006d
	PID_STREAMDECK_MK2      = 0x0080
	PID_STREAMDECK_MINI     = 0x0063
	PID_STREAMDECK_MINI_MK2 = 0x0090
	PID_STREAMDECK_XL       = 0x006c
	PID_STREAMDECK_XL_V2    = 0x008f
	PID_STREAMDECK_PLUS     = 0x0084
)

// Firmware command IDs.
//
//nolint:revive
var (
	c_REV1_FIRMWARE   = []byte{0x04}
	c_REV1_RESET      = []byte{0x0b, 0x63}
	c_REV1_BRIGHTNESS = []byte{0x05, 0x55, 0xaa, 0xd1, 0x01}

	c_REV2_FIRMWARE   = []byte{0x05}
	c_REV2_RESET      = []byte{0x03, 0x02}
	c_REV2_BRIGHTNESS = []byte{0x03, 0x08}
)

// Device represents a single Stream Deck device.
type Device struct {
	ID     string
	Serial string

	Columns uint8
	Rows    uint8
	Keys    uint8
	Pixels  uint
	DPI     uint
	Padding uint

	// Dials is the number of rotary dials (Stream Deck+).
	Dials uint8

	// ScreenWidth and ScreenHeight are the pixel dimensions of the touch
	// screen strip (Stream Deck+), which is split into ScreenSegments
	// equally sized segments, one above each dial. All zero if the device
	// has no touch screen.
	ScreenWidth    uint
	ScreenHeight   uint
	ScreenDPI      uint
	ScreenSegments uint8

	featureReportSize    int
	firmwareOffset       int
	keyStateOffset       int
	reportTypeOffset     int // offset of the input report type byte, 0 if the device has none
	translateKeyIndex    func(index, columns uint8) uint8
	imagePageSize        int
	imagePageHeaderSize  int
	flipImage            func(image.Image) image.Image
	toImageFormat        func(image.Image) ([]byte, error)
	imagePageHeader      func(pageIndex int, keyIndex uint8, payloadLength int, lastPage bool) []byte
	screenPageSize       int
	screenPageHeaderSize int

	getFirmwareCommand   []byte
	resetCommand         []byte
	setBrightnessCommand []byte

	keyState  []byte
	dialState []byte

	device hid.Device
	info   hid.DeviceInfo

	lastActionTime time.Time
	asleep         bool
	sleepCancel    context.CancelFunc
	sleepMutex     *sync.RWMutex
	fadeDuration   time.Duration

	brightness         uint8
	preSleepBrightness uint8
}

// Event is an input event read from the device: Key, DialPress, DialTurn or
// Touch.
type Event interface {
	isEvent()
}

// Key holds the current status of a key on the device.
type Key struct {
	Index   uint8
	Pressed bool
}

// DialPress is emitted when a dial gets pressed or released.
type DialPress struct {
	Index   uint8
	Pressed bool
}

// DialTurn is emitted when a dial gets turned. Steps is positive for
// clockwise and negative for counterclockwise turns.
type DialTurn struct {
	Index uint8
	Steps int8
}

// TouchType describes the kind of a Touch event.
type TouchType uint8

// Touch event types.
const (
	TouchTap TouchType = iota + 1
	TouchLongPress
	TouchSwipe
)

// Touch is emitted when the touch screen gets tapped, long-pressed or
// swiped. X and Y are the screen coordinates of the touch, EndX and EndY the
// end point of a swipe.
type Touch struct {
	Type       TouchType
	X, Y       uint16
	EndX, EndY uint16
}

func (Key) isEvent()       {}
func (DialPress) isEvent() {}
func (DialTurn) isEvent()  {}
func (Touch) isEvent()     {}

// Input report types and layout of devices with multiple input types
// (Stream Deck+).
const (
	inputTypeKeys  = 0x00
	inputTypeTouch = 0x02
	inputTypeDials = 0x03

	inputSubtypeOffset = 4
	dialStateOffset    = 5
	dialSubtypePress   = 0x00
	dialSubtypeTurn    = 0x01

	touchXOffset    = 6
	touchYOffset    = 8
	touchEndXOffset = 10
	touchEndYOffset = 12
	touchReportSize = 14
)

// Devices returns all attached Stream Decks.
func Devices() ([]Device, error) {
	dd := []Device{}

	devs, err := hid.Enumerate(VID_ELGATO, 0)
	if err != nil {
		return nil, err
	}
	for _, d := range devs {
		var dev Device

		switch {
		case d.VendorID == VID_ELGATO && d.ProductID == PID_STREAMDECK:
			dev = Device{
				ID:                   d.Path,
				Serial:               d.Serial,
				Columns:              5,
				Rows:                 3,
				Keys:                 15,
				Pixels:               72,
				DPI:                  124,
				Padding:              16,
				featureReportSize:    17,
				firmwareOffset:       5,
				keyStateOffset:       1,
				translateKeyIndex:    translateRightToLeft,
				imagePageSize:        7819,
				imagePageHeaderSize:  16,
				imagePageHeader:      rev1ImagePageHeader,
				flipImage:            flipHorizontally,
				toImageFormat:        toBMP,
				getFirmwareCommand:   c_REV1_FIRMWARE,
				resetCommand:         c_REV1_RESET,
				setBrightnessCommand: c_REV1_BRIGHTNESS,
			}
		case d.VendorID == VID_ELGATO && (d.ProductID == PID_STREAMDECK_MINI || d.ProductID == PID_STREAMDECK_MINI_MK2):
			dev = Device{
				ID:                   d.Path,
				Serial:               d.Serial,
				Columns:              3,
				Rows:                 2,
				Keys:                 6,
				Pixels:               80,
				DPI:                  138,
				Padding:              16,
				featureReportSize:    17,
				firmwareOffset:       5,
				keyStateOffset:       1,
				translateKeyIndex:    identity,
				imagePageSize:        1024,
				imagePageHeaderSize:  16,
				imagePageHeader:      miniImagePageHeader,
				flipImage:            rotateCounterclockwise,
				toImageFormat:        toBMP,
				getFirmwareCommand:   c_REV1_FIRMWARE,
				resetCommand:         c_REV1_RESET,
				setBrightnessCommand: c_REV1_BRIGHTNESS,
			}
		case d.VendorID == VID_ELGATO && (d.ProductID == PID_STREAMDECK_V2 || d.ProductID == PID_STREAMDECK_MK2):
			dev = Device{
				ID:                   d.Path,
				Serial:               d.Serial,
				Columns:              5,
				Rows:                 3,
				Keys:                 15,
				Pixels:               72,
				DPI:                  124,
				Padding:              16,
				featureReportSize:    32,
				firmwareOffset:       6,
				keyStateOffset:       4,
				translateKeyIndex:    identity,
				imagePageSize:        1024,
				imagePageHeaderSize:  8,
				imagePageHeader:      rev2ImagePageHeader,
				flipImage:            flipHorizontallyAndVertically,
				toImageFormat:        toJPEG,
				getFirmwareCommand:   c_REV2_FIRMWARE,
				resetCommand:         c_REV2_RESET,
				setBrightnessCommand: c_REV2_BRIGHTNESS,
			}
		case d.VendorID == VID_ELGATO && (d.ProductID == PID_STREAMDECK_XL || d.ProductID == PID_STREAMDECK_XL_V2):
			dev = Device{
				ID:                   d.Path,
				Serial:               d.Serial,
				Columns:              8,
				Rows:                 4,
				Keys:                 32,
				Pixels:               96,
				DPI:                  166,
				Padding:              16,
				featureReportSize:    32,
				firmwareOffset:       6,
				keyStateOffset:       4,
				translateKeyIndex:    identity,
				imagePageSize:        1024,
				imagePageHeaderSize:  8,
				imagePageHeader:      rev2ImagePageHeader,
				flipImage:            flipHorizontallyAndVertically,
				toImageFormat:        toJPEG,
				getFirmwareCommand:   c_REV2_FIRMWARE,
				resetCommand:         c_REV2_RESET,
				setBrightnessCommand: c_REV2_BRIGHTNESS,
			}
		case d.VendorID == VID_ELGATO && d.ProductID == PID_STREAMDECK_PLUS:
			dev = Device{
				ID:                   d.Path,
				Serial:               d.Serial,
				Columns:              4,
				Rows:                 2,
				Keys:                 8,
				Pixels:               120,
				DPI:                  166,
				Padding:              16,
				Dials:                4,
				ScreenWidth:          800,
				ScreenHeight:         100,
				ScreenDPI:            188, // 800px over 108mm
				ScreenSegments:       4,
				featureReportSize:    32,
				firmwareOffset:       6,
				keyStateOffset:       4,
				reportTypeOffset:     1,
				translateKeyIndex:    identity,
				imagePageSize:        1024,
				imagePageHeaderSize:  8,
				imagePageHeader:      rev2ImagePageHeader,
				flipImage:            noFlip,
				toImageFormat:        toJPEG,
				screenPageSize:       1024,
				screenPageHeaderSize: 16,
				getFirmwareCommand:   c_REV2_FIRMWARE,
				resetCommand:         c_REV2_RESET,
				setBrightnessCommand: c_REV2_BRIGHTNESS,
			}
		}

		if dev.ID != "" {
			dev.keyState = make([]byte, dev.Columns*dev.Rows)
			dev.dialState = make([]byte, dev.Dials)
			dev.info = d
			dd = append(dd, dev)
		}
	}

	return dd, nil
}

// Open the device for input/output. This must be called before trying to
// communicate with the device.
func (d *Device) Open() error {
	var err error
	d.device, err = d.info.Open()
	d.lastActionTime = time.Now()
	d.sleepMutex = &sync.RWMutex{}
	return err
}

// Close the connection with the device.
func (d *Device) Close() error {
	d.cancelSleepTimer()
	return d.device.Close()
}

// FirmwareVersion returns the firmware version of the device.
func (d Device) FirmwareVersion() (string, error) {
	result, err := d.getFeatureReport(d.getFirmwareCommand)
	if err != nil {
		return "", err
	}
	// the version string is padded with NUL bytes and spaces
	return strings.TrimRight(string(result[d.firmwareOffset:]), "\x00 "), nil
}

// Reset resets the Stream Deck, clears all button images and shows the standby
// image.
func (d Device) Reset() error {
	return d.sendFeatureReport(d.resetCommand)
}

// Clear clears the Stream Deck, setting a black image on all buttons and the
// touch screen.
func (d Device) Clear() error {
	img := image.NewRGBA(image.Rect(0, 0, int(d.Pixels), int(d.Pixels)))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{0, 0, 0, 255}), image.Point{}, draw.Src)
	for i := uint8(0); i < d.Columns*d.Rows; i++ {
		err := d.SetImage(i, img)
		if err != nil {
			fmt.Println(err)
			return err
		}
	}

	if d.ScreenWidth > 0 {
		screen := image.NewRGBA(image.Rect(0, 0, int(d.ScreenWidth), int(d.ScreenHeight)))
		draw.Draw(screen, screen.Bounds(), image.NewUniform(color.RGBA{0, 0, 0, 255}), image.Point{}, draw.Src)
		if err := d.SetScreenImage(0, 0, screen); err != nil {
			return err
		}
	}

	return nil
}

// ReadKeys returns a channel, which it will use to emit key presses/releases.
// Dial and touch screen input is ignored; use ReadInput to receive it.
func (d *Device) ReadKeys() (chan Key, error) {
	ich, err := d.ReadInput()
	if err != nil {
		return nil, err
	}

	kch := make(chan Key)
	go func() {
		for e := range ich {
			if k, ok := e.(Key); ok {
				kch <- k
			}
		}
		close(kch)
	}()

	return kch, nil
}

// ReadInput returns a channel, which it will use to emit all input events:
// Key, DialPress, DialTurn and Touch. The channel gets closed when reading
// from the device fails.
func (d *Device) ReadInput() (chan Event, error) {
	ch := make(chan Event)

	size := d.keyStateOffset + len(d.keyState)
	if d.reportTypeOffset > 0 && size < touchReportSize {
		size = touchReportSize
	}
	buf := make([]byte, size)

	go func() {
		for {
			n, err := d.device.Read(buf)
			if err != nil {
				close(ch)
				return
			}
			// don't parse leftovers of a previous report
			clear(buf[n:])

			// don't trigger an event if the device is asleep, but wake it
			if d.asleep {
				_ = d.Wake()

				// reset state so no spurious events get triggered
				for i := range d.keyState {
					d.keyState[i] = 0
				}
				for i := range d.dialState {
					d.dialState[i] = 0
				}
				continue
			}

			d.sleepMutex.Lock()
			d.lastActionTime = time.Now()
			d.sleepMutex.Unlock()

			for _, e := range d.parseInput(buf) {
				ch <- e
			}
		}
	}()

	return ch, nil
}

// parseInput turns an input report into events, updating the known key and
// dial states.
func (d *Device) parseInput(buf []byte) []Event {
	required := d.keyStateOffset + len(d.keyState)
	if d.reportTypeOffset > 0 {
		required = max(required, dialStateOffset+len(d.dialState), touchReportSize)
	}
	if len(buf) < required {
		return nil
	}

	inputType := byte(inputTypeKeys)
	if d.reportTypeOffset > 0 {
		inputType = buf[d.reportTypeOffset]
	}

	var events []Event
	switch inputType {
	case inputTypeKeys:
		for i := range d.keyState {
			state := buf[d.keyStateOffset+i]
			if state != d.keyState[i] {
				d.keyState[i] = state
				events = append(events, Key{
					Index:   d.translateKeyIndex(uint8(i), d.Columns),
					Pressed: state == 1,
				})
			}
		}

	case inputTypeDials:
		for i := range d.dialState {
			value := buf[dialStateOffset+i]
			switch buf[inputSubtypeOffset] {
			case dialSubtypePress:
				if value != d.dialState[i] {
					d.dialState[i] = value
					events = append(events, DialPress{Index: uint8(i), Pressed: value == 1})
				}
			case dialSubtypeTurn:
				if value != 0 {
					// the device reports steps as a signed byte
					events = append(events, DialTurn{Index: uint8(i), Steps: int8(value)}) //nolint:gosec
				}
			}
		}

	case inputTypeTouch:
		t := Touch{
			Type: TouchType(buf[inputSubtypeOffset]),
			X:    binary.LittleEndian.Uint16(buf[touchXOffset:]),
			Y:    binary.LittleEndian.Uint16(buf[touchYOffset:]),
		}
		switch t.Type {
		case TouchTap, TouchLongPress:
			events = append(events, t)
		case TouchSwipe:
			t.EndX = binary.LittleEndian.Uint16(buf[touchEndXOffset:])
			t.EndY = binary.LittleEndian.Uint16(buf[touchEndYOffset:])
			events = append(events, t)
		}
	}

	return events
}

// Sleep puts the device asleep, waiting for a key event to wake it up.
func (d *Device) Sleep() error {
	d.sleepMutex.Lock()
	defer d.sleepMutex.Unlock()

	d.preSleepBrightness = d.brightness

	if err := d.Fade(d.brightness, 0, d.fadeDuration); err != nil {
		return err
	}

	d.asleep = true
	return d.SetBrightness(0)
}

// Wake wakes the device from sleep.
func (d *Device) Wake() error {
	d.sleepMutex.Lock()
	defer d.sleepMutex.Unlock()

	d.asleep = false
	if err := d.Fade(0, d.preSleepBrightness, d.fadeDuration); err != nil {
		return err
	}

	d.lastActionTime = time.Now()
	return d.SetBrightness(d.preSleepBrightness)
}

// Asleep returns true if the device is asleep.
func (d Device) Asleep() bool {
	return d.asleep
}

func (d *Device) cancelSleepTimer() {
	if d.sleepCancel == nil {
		return
	}

	d.sleepCancel()
	d.sleepCancel = nil
}

// SetSleepFadeDuration sets the duration of the fading animation when the
// device is put to sleep or wakes up.
func (d *Device) SetSleepFadeDuration(t time.Duration) {
	d.fadeDuration = t
}

// SetSleepTimeout sets the time after which the device will sleep if no key
// events are received.
func (d *Device) SetSleepTimeout(t time.Duration) {
	d.cancelSleepTimer()
	if t == 0 {
		return
	}

	var ctx context.Context
	ctx, d.sleepCancel = context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-time.After(time.Second):
				d.sleepMutex.RLock()
				since := time.Since(d.lastActionTime)
				d.sleepMutex.RUnlock()

				if !d.asleep && since >= t {
					_ = d.Sleep()
				}

			case <-ctx.Done():
				return
			}
		}
	}()
}

// Fade fades the brightness in or out.
func (d *Device) Fade(start uint8, end uint8, duration time.Duration) error {
	step := (float64(end) - float64(start)) / float64(duration/fadeDelay)
	if step == math.Inf(1) || step == math.Inf(-1) {
		return nil
	}

	for current := float64(start); ; current += step {
		if !((start < end && int8(current) < int8(end)) ||
			(start > end && int8(current) > int8(end))) {
			break
		}
		if err := d.SetBrightness(uint8(current)); err != nil {
			return err
		}

		time.Sleep(fadeDelay)
	}
	return nil
}

// SetBrightness sets the background lighting brightness from 0 to 100 percent.
func (d *Device) SetBrightness(percent uint8) error {
	if percent > 100 {
		percent = 100
	}

	d.brightness = percent
	if d.asleep && percent > 0 {
		// if the device is asleep, remember the brightness, but don't set it
		d.sleepMutex.Lock()
		d.preSleepBrightness = percent
		d.sleepMutex.Unlock()
		return nil
	}

	report := make([]byte, len(d.setBrightnessCommand)+1)
	copy(report, d.setBrightnessCommand)
	report[len(report)-1] = percent

	return d.sendFeatureReport(report)
}

// SetImage sets the image of a button on the Stream Deck. The provided image
// needs to be in the correct resolution for the device. The index starts with
// 0 being the top-left button.
func (d Device) SetImage(index uint8, img image.Image) error {
	if img.Bounds().Dy() != int(d.Pixels) ||
		img.Bounds().Dx() != int(d.Pixels) {
		return fmt.Errorf("supplied image has wrong dimensions, expected %[1]dx%[1]d pixels", d.Pixels)
	}

	imageBytes, err := d.toImageFormat(d.flipImage(img))
	if err != nil {
		return fmt.Errorf("cannot convert image data: %v", err)
	}

	keyIndex := d.translateKeyIndex(index, d.Columns)
	return d.writeImagePages(imageBytes, d.imagePageSize, d.imagePageHeaderSize,
		func(page, payloadLength int, lastPage bool) []byte {
			return d.imagePageHeader(page, keyIndex, payloadLength, lastPage)
		})
}

// SetScreenImage sets the image of a region of the touch screen, with its
// top-left corner at x, y. The image must fit within the screen.
func (d Device) SetScreenImage(x, y int, img image.Image) error {
	if d.ScreenWidth == 0 {
		return fmt.Errorf("device has no touch screen")
	}

	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if x < 0 || y < 0 || x+w > int(d.ScreenWidth) || y+h > int(d.ScreenHeight) {
		return fmt.Errorf("supplied image of %dx%d pixels at %d,%d exceeds the %dx%d pixel screen",
			w, h, x, y, d.ScreenWidth, d.ScreenHeight)
	}

	imageBytes, err := toJPEG(img)
	if err != nil {
		return fmt.Errorf("cannot convert image data: %v", err)
	}

	return d.writeImagePages(imageBytes, d.screenPageSize, d.screenPageHeaderSize,
		func(page, payloadLength int, lastPage bool) []byte {
			return screenImagePageHeader(page, x, y, w, h, payloadLength, lastPage)
		})
}

// SetScreenSegmentImage sets the image of a touch screen segment. The image
// needs to be of the segment's size. The index starts with 0 being the
// leftmost segment.
func (d Device) SetScreenSegmentImage(index uint8, img image.Image) error {
	if index >= d.ScreenSegments {
		return fmt.Errorf("invalid screen segment %d, device has %d segments", index, d.ScreenSegments)
	}

	w, h := d.ScreenSegmentWidth(), d.ScreenHeight
	if img.Bounds().Dx() != int(w) || img.Bounds().Dy() != int(h) {
		return fmt.Errorf("supplied image has wrong dimensions, expected %dx%d pixels", w, h)
	}

	return d.SetScreenImage(int(uint(index)*w), 0, img)
}

// ScreenSegmentWidth returns the width of a touch screen segment in pixels,
// or 0 if the device has no touch screen.
func (d Device) ScreenSegmentWidth() uint {
	if d.ScreenSegments == 0 {
		return 0
	}
	return d.ScreenWidth / uint(d.ScreenSegments)
}

// writeImagePages sends image data to the device, split into pages of
// pageSize bytes (including a header of headerSize bytes).
func (d Device) writeImagePages(imageBytes []byte, pageSize, headerSize int,
	header func(page, payloadLength int, lastPage bool) []byte) error {
	imageData := imageData{
		image:    imageBytes,
		pageSize: pageSize - headerSize,
	}

	data := make([]byte, pageSize)

	var page int
	var lastPage bool
	for !lastPage {
		var payload []byte
		payload, lastPage = imageData.Page(page)
		h := header(page, len(payload), lastPage)

		// pad each page with zeros, like the reference implementations
		clear(data)
		copy(data, h)
		copy(data[len(h):], payload)

		n, err := d.device.Write(data)
		if err == nil && n < len(data) {
			err = fmt.Errorf("short write of %d bytes", n)
		}
		if err != nil {
			return fmt.Errorf("cannot write image page %d of %d (%d image bytes) %d bytes: %v",
				page, imageData.PageCount(), imageData.Length(), len(data), err)
		}

		page++
	}

	return nil
}

// getFeatureReport from the device without worries about the correct payload
// size.
func (d Device) getFeatureReport(payload []byte) ([]byte, error) {
	b := make([]byte, d.featureReportSize)
	copy(b, payload)
	_, err := d.device.GetFeatureReport(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// sendFeatureReport to the device without worries about the correct payload
// size.
func (d Device) sendFeatureReport(payload []byte) error {
	b := make([]byte, d.featureReportSize)
	copy(b, payload)
	_, err := d.device.SendFeatureReport(b)
	return err
}

// translateRightToLeft translates the given key index from right-to-left to
// left-to-right, based on the given number of columns.
func translateRightToLeft(index, columns uint8) uint8 {
	keyCol := index % columns
	return (index - keyCol) + (columns - 1) - keyCol
}

// identity returns the given key index as it is.
func identity(index, _ uint8) uint8 {
	return index
}

// toRGBA converts an image.Image to an image.RGBA.
func toRGBA(img image.Image) *image.RGBA {
	switch img := img.(type) {
	case *image.RGBA:
		return img
	}
	out := image.NewRGBA(img.Bounds())
	draw.Copy(out, image.Pt(0, 0), img, img.Bounds(), draw.Src, nil)
	return out
}

// noFlip returns the given image unchanged.
func noFlip(img image.Image) image.Image {
	return img
}

// flipHorizontally returns the given image horizontally flipped.
func flipHorizontally(img image.Image) image.Image {
	flipped := image.NewRGBA(img.Bounds())
	draw.Copy(flipped, image.Point{}, img, img.Bounds(), draw.Src, nil)
	for y := 0; y < flipped.Bounds().Dy(); y++ {
		for x := 0; x < flipped.Bounds().Dx()/2; x++ {
			xx := flipped.Bounds().Max.X - x - 1
			c := flipped.RGBAAt(x, y)
			flipped.SetRGBA(x, y, flipped.RGBAAt(xx, y))
			flipped.SetRGBA(xx, y, c)
		}
	}
	return flipped
}

// flipHorizontallyAndVertically returns the given image horizontally and
// vertically flipped.
func flipHorizontallyAndVertically(img image.Image) image.Image {
	flipped := image.NewRGBA(img.Bounds())
	draw.Copy(flipped, image.Point{}, img, img.Bounds(), draw.Src, nil)
	for y := 0; y < flipped.Bounds().Dy()/2; y++ {
		yy := flipped.Bounds().Max.Y - y - 1
		for x := 0; x < flipped.Bounds().Dx(); x++ {
			xx := flipped.Bounds().Max.X - x - 1
			c := flipped.RGBAAt(x, y)
			flipped.SetRGBA(x, y, flipped.RGBAAt(xx, yy))
			flipped.SetRGBA(xx, yy, c)
		}
	}
	return flipped
}

// rotateCounterclockwise returns the given image rotated counterclockwise.
func rotateCounterclockwise(img image.Image) image.Image {
	flipped := image.NewRGBA(img.Bounds())
	draw.Copy(flipped, image.Point{}, img, img.Bounds(), draw.Src, nil)
	for y := 0; y < flipped.Bounds().Dy(); y++ {
		for x := y + 1; x < flipped.Bounds().Dx(); x++ {
			c := flipped.RGBAAt(x, y)
			flipped.SetRGBA(x, y, flipped.RGBAAt(y, x))
			flipped.SetRGBA(y, x, c)
		}
	}
	for y := 0; y < flipped.Bounds().Dy()/2; y++ {
		yy := flipped.Bounds().Max.Y - y - 1
		for x := 0; x < flipped.Bounds().Dx(); x++ {
			c := flipped.RGBAAt(x, y)
			flipped.SetRGBA(x, y, flipped.RGBAAt(x, yy))
			flipped.SetRGBA(x, yy, c)
		}
	}
	return flipped
}

// toBMP returns the raw bytes of the given image in BMP format.
func toBMP(img image.Image) ([]byte, error) {
	rgba := toRGBA(img)

	// this is a BMP file header followed by a BPM bitmap info header
	// find more information here: https://en.wikipedia.org/wiki/BMP_file_format
	header := []byte{
		0x42, 0x4d, 0xf6, 0x3c, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x36, 0x00, 0x00, 0x00, 0x28, 0x00,
		0x00, 0x00, 0x48, 0x00, 0x00, 0x00, 0x48, 0x00,
		0x00, 0x00, 0x01, 0x00, 0x18, 0x00, 0x00, 0x00,
		0x00, 0x00, 0xc0, 0x3c, 0x00, 0x00, 0xc4, 0x0e,
		0x00, 0x00, 0xc4, 0x0e, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	buffer := make([]byte, len(header)+rgba.Bounds().Dx()*rgba.Bounds().Dy()*3)
	copy(buffer, header)

	i := len(header)
	for y := 0; y < rgba.Bounds().Dy(); y++ {
		for x := 0; x < rgba.Bounds().Dx(); x++ {
			c := rgba.RGBAAt(x, y)
			buffer[i] = c.B
			buffer[i+1] = c.G
			buffer[i+2] = c.R
			i += 3
		}
	}
	return buffer, nil
}

// toJPEG returns the raw bytes of the given image in JPEG format.
func toJPEG(img image.Image) ([]byte, error) {
	buffer := bytes.NewBuffer([]byte{})
	opts := jpeg.Options{
		Quality: 100,
	}
	err := jpeg.Encode(buffer, img, &opts)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), err
}

// rev1ImagePageHeader returns the image page header sequence used by the
// Stream Deck v1.
func rev1ImagePageHeader(pageIndex int, keyIndex uint8, _ int, lastPage bool) []byte {
	var lastPageByte byte
	if lastPage {
		lastPageByte = 1
	}
	return []byte{
		0x02, 0x01,
		byte(pageIndex + 1), 0x00,
		lastPageByte,
		keyIndex + 1,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
}

// miniImagePageHeader returns the image page header sequence used by the
// Stream Deck Mini.
func miniImagePageHeader(pageIndex int, keyIndex uint8, _ int, lastPage bool) []byte {
	var lastPageByte byte
	if lastPage {
		lastPageByte = 1
	}
	return []byte{
		0x02, 0x01,
		byte(pageIndex), 0x00,
		lastPageByte,
		keyIndex + 1,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
}

// rev2ImagePageHeader returns the image page header sequence used by Stream
// Deck XL and Stream Deck v2.
func rev2ImagePageHeader(pageIndex int, keyIndex uint8, payloadLength int, lastPage bool) []byte {
	var lastPageByte byte
	if lastPage {
		lastPageByte = 1
	}
	return []byte{
		0x02, 0x07, keyIndex, lastPageByte,
		byte(payloadLength), byte(payloadLength >> 8),
		byte(pageIndex), byte(pageIndex >> 8),
	}
}

// screenImagePageHeader returns the image page header sequence used by the
// Stream Deck+ touch screen. Values are encoded as little-endian 16 bit.
//
//nolint:gosec
func screenImagePageHeader(pageIndex, x, y, width, height, payloadLength int, lastPage bool) []byte {
	var lastPageByte byte
	if lastPage {
		lastPageByte = 1
	}
	return []byte{
		0x02, 0x0c,
		byte(x), byte(x >> 8),
		byte(y), byte(y >> 8),
		byte(width), byte(width >> 8),
		byte(height), byte(height >> 8),
		lastPageByte,
		byte(pageIndex), byte(pageIndex >> 8),
		byte(payloadLength), byte(payloadLength >> 8),
		0x00,
	}
}

// imageData allows to access raw image data in a byte array through pages of a
// given size.
type imageData struct {
	image    []byte
	pageSize int
}

// Page returns the page with the given index and an indication if this is the
// last page.
func (d imageData) Page(pageIndex int) ([]byte, bool) {
	offset := pageIndex * d.pageSize
	if offset >= len(d.image) {
		return []byte{}, true
	}

	length := d.pageLength(pageIndex)
	if offset+length > len(d.image) {
		length = len(d.image) - offset
	}

	return d.image[offset : offset+length], pageIndex == d.PageCount()-1
}

func (d imageData) pageLength(pageIndex int) int {
	remaining := len(d.image) - (pageIndex * d.pageSize)
	if remaining > d.pageSize {
		return d.pageSize
	}
	if remaining > 0 {
		return remaining
	}
	return 0
}

// PageCount returns the total number of pages.
func (d imageData) PageCount() int {
	count := len(d.image) / d.pageSize
	if len(d.image)%d.pageSize != 0 {
		return count + 1
	}
	return count
}

// Length of the raw image data in bytes.
func (d imageData) Length() int {
	return len(d.image)
}
