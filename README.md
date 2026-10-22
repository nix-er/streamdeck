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
# Stream Deck Original, Mini, XL, Original v2, MK.2, Plus, XL v2, Mini MK.2
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
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="008f", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="008f", TAG+="uaccess"
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="0090", TAG+="uaccess"
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0fd9", ATTRS{idProduct}=="0090", TAG+="uaccess"
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

## Feedback

Got some feedback or suggestions? Please open an issue or drop me a note!

* [Twitter](https://twitter.com/mueslix)
* [The Fediverse](https://mastodon.social/@fribbledom)
