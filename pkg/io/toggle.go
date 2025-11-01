package io

import (
	"fmt"
	"os"

	"github.com/warthog618/go-gpiocdev"
)

// SetPinState is a utility function to set a GPIO pin to a high or low state.
// It takes the pin name as a string and the desired state (gpio.High or gpio.Low).
func (io *IO) SetPinState(pinName int, state int) error {
	// Look up the GPIO pin by its name (e.g., "GPIO21").
	var l *gpiocdev.Line
	var err error
	if v, ok := io.lines[pinName]; ok {
		l = v
	} else {
		l, err = io.chip.RequestLine(pinName, gpiocdev.AsOutput(0))
		if err != nil {
			fmt.Printf("Opening chip returned error: %s\n", err)
			os.Exit(1)
		}
		io.lines[pinName] = l

	}
	// Set the pin's output state.
	return l.SetValue(state)
}
