# streamdeck

[![Latest Release](https://img.shields.io/github/release/muesli/streamdeck.svg?style=for-the-badge)](https://github.com/muesli/streamdeck/releases)
[![Software License](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=for-the-badge)](/LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/nix-er/streamdeck/build.yml?branch=master&style=for-the-badge)](https://github.com/nix-er/streamdeck/actions)
[![Go ReportCard](https://goreportcard.com/badge/github.com/nix-er/streamdeck?style=for-the-badge)](https://goreportcard.com/report/nix-er/streamdeck)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=for-the-badge)](https://pkg.go.dev/github.com/nix-er/streamdeck)

A CLI application and Go library to control your Elgato Stream Deck on Linux.

If you're looking for a complete Linux service to control your StreamDeck, check
out [Deckmaster](https://github.com/nix-er/deckmaster), which is based on this
library.

## Supported devices

| Device | Product ID | Keys | Other controls |
|---|---|---|---|
| Stream Deck Original, Original v2 | 0060, 006d | 5×3 | |
| Stream Deck MK.2, MK.2 Scissor Keys | 0080, 00a5 | 5×3 | |
| Stream Deck Mini, Mini MK.2, Mini Discord Edition | 0063, 0090, 00b3 | 3×2 | |
| Stream Deck XL, XL v2 | 006c, 008f | 8×4 | |
| Stream Deck Module 6, 15, 32 | 00b8, 00b9, 00e4, 00ba | 3×2, 5×3, 8×4 | |
| Stream Deck Neo* | 009a | 4×2 | 2 touch keys with RGB LEDs, 248×58 info screen |
| Stream Deck Pedal* | 0086 | 3 pedals | no displays |
| Stream Deck+ | 0084 | 4×2 | 4 dials, 800×100 touch screen |
| Stream Deck + XL* | 00c6 | 9×4 | 6 dials, 1200×100 touch screen |
| Stream Deck Studio* | 00aa | 16×2, 144×112 pixels | 2 dials with RGB LED rings |

\* Support for these models follows the reference implementations
([python-elgato-streamdeck](https://github.com/abcminiuser/python-elgato-streamdeck),
[node-elgato-stream-deck](https://github.com/Julusian/node-elgato-stream-deck),
[elgato-streamdeck](https://github.com/OpenActionAPI/rust-elgato-streamdeck))
and hasn't been tested on hardware yet. Reports are welcome!

## Installation

Make sure you have a working Go environment (Go 1.26 or higher is required).
See the [install instructions](http://golang.org/doc/install.html).

To install the `streamdeck-cli` tool, run:

    go install github.com/nix-er/streamdeck/cmd/streamdeck-cli@latest

To use streamdeck as a library in your Go module, run:

    go get github.com/nix-er/streamdeck

## Configuration

On Linux you need to set up some udev rules to be able to access the device as a
regular user. Create `/etc/udev/rules.d/70-streamdeck.rules` with these lines:

```
# Elgato Stream Deck (vendor 0fd9), all models
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="0060", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="0060", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="0063", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="0063", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="006c", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="006c", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="006d", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="006d", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="0080", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="0080", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="0084", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="0084", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="0086", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="0086", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="008f", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="008f", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="0090", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="0090", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="009a", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="009a", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00a5", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00a5", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00aa", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00aa", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00b3", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00b3", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00b8", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00b8", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00b9", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00b9", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00ba", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00ba", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00c6", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00c6", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00e4", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="00e4", TAG+="uaccess"
```

`TAG+="uaccess"` gives the user logged in on the local seat access to the
device, so no extra group membership is needed. The file name must sort before
`73-seat-late.rules`, or `uaccess` has no effect. Reload the rules with
`sudo udevadm control --reload-rules && sudo udevadm trigger`. Unplug and replug
the device and you should be good to go.

## Usage

Control the brightness, in percent between 0 and 100:

```
streamdeck-cli brightness 50
```

Set an image on the first key (from the top-left):

```
streamdeck-cli image 0 image.png
```

Clear all images:

```
streamdeck-cli clear
```

Reset the device:

```
streamdeck-cli reset
```

## Dials, touch screens and LEDs

`Device.ReadInput()` emits all input events of a device: `Key`, `DialPress`,
`DialTurn` (with a signed step count, negative for counterclockwise turns) and
`Touch` (tap, long press or swipe, with screen coordinates). `ReadKeys()` only
emits key events.

```go
events, err := dev.ReadInput()
if err != nil {
	return err
}
for e := range events {
	switch e := e.(type) {
	case streamdeck.Key:
		fmt.Println("key", e.Index, e.Pressed)
	case streamdeck.DialPress:
		fmt.Println("dial", e.Index, e.Pressed)
	case streamdeck.DialTurn:
		fmt.Println("dial", e.Index, "turned", e.Steps)
	case streamdeck.Touch:
		fmt.Println("touch", e.Type, e.X, e.Y)
	}
}
```

The touch screen (`dev.ScreenWidth` × `dev.ScreenHeight` pixels) is split into
`dev.ScreenSegments` segments, one above each dial. Set an image on a segment
with `dev.SetScreenSegmentImage(index, img)`, or on any region with
`dev.SetScreenImage(x, y, img)`.

Key images must be `dev.KeyWidth` × `dev.KeyHeight` pixels, which isn't square
on the Stream Deck Studio. The Stream Deck Pedal has no displays (`KeyWidth` is
0), so setting images and the brightness does nothing.

The Stream Deck Neo's touch keys report as keys `dev.Keys` to
`dev.Keys+dev.TouchKeys-1`. Set their LED color with `dev.SetKeyColor(index,
color)`, and the info screen (`dev.InfoScreenWidth` × `dev.InfoScreenHeight`
pixels) with `dev.SetInfoScreenImage(img)`.

On devices with `dev.DialRings` (Stream Deck Studio), light a dial's LED ring
with `dev.SetDialRingColor(index, color)` or partially with
`dev.SetDialRingPercentage(index, color, percent)`, and set its knob LED with
`dev.SetDialKnobColor(index, color)`.

## Feedback

Got some feedback or suggestions? Please open an issue or drop me a note!

* [Twitter](https://twitter.com/mueslix)
* [The Fediverse](https://mastodon.social/@fribbledom)
