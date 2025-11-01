package io

import (
	"fmt"
	"time"

	"github.com/warthog618/go-gpiocdev"
)

type Button struct {
	offset        int
	status        bool
	start         time.Time
	Event         chan ButtonEvent
	hadFirstPress bool
}
type ButtonEvent struct {
	Status   bool
	Duration time.Duration
}

func (b *Button) eventHandler(evt gpiocdev.LineEvent) {
	var diff time.Duration

	rising := true
	if evt.Type == gpiocdev.LineEventFallingEdge {
		rising = false
	}
	if b.status == rising {
		if b.start.IsZero() {
			println("zero")
			b.start = time.Now()
		}
		return
	}
	if rising {
		diff = time.Since(b.start)
	}
	b.status = rising
	b.start = time.Now()
	if diff < 10*time.Millisecond {
		fmt.Printf("diff %v\n", diff)
		return
	}
	if !b.hadFirstPress {
		b.hadFirstPress = true
		return
	}
	b.Event <- ButtonEvent{
		Status:   rising,
		Duration: diff,
	}
}

// WatchButton initializes the periph.io host and returns a channel
// that delivers gpio.Edge events for a specified pin.
// The pin is configured as an input with a pull-down resistor.
func (io *IO) WatchButton(lineOffset int) (*Button, error) {
	// Open the GPIO line

	b := Button{
		Event: make(chan ButtonEvent),
	}
	line, err := io.chip.RequestLine(lineOffset,
		gpiocdev.WithPullUp,
		gpiocdev.WithBothEdges,
		gpiocdev.WithEventHandler(b.eventHandler),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to request GPIO line: %w", err)
	}
	io.lines[lineOffset] = line
	// Close line when context is done

	return &b, nil
}
