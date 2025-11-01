package io

import (
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/devices/v3/pca9685"
)

// ControlServo configures the PCA9685 board and sets the duty cycle for a given servo channel.
func ControlServo(servoChannel int, onCycle, offDuty gpio.Duty) error {
	// Find the I2C bus. Most Raspberry Pi boards use I2C1.
	bus, err := i2creg.Open("I2C1")
	if err != nil {
		return err
	}
	defer bus.Close()
	// Create a new PCA9685 device. The address is usually 0x40.
	dev, err := pca9685.NewI2C(bus, 0x40)
	if err != nil {
		return err
	}
	// Set the PWM frequency. 50Hz is standard for most servos.
	if err := dev.SetPwmFreq(50); err != nil {
		return err
	}

	// Set the duty cycle for the specified servo channel.
	// A standard servo range is from ~500µs to ~2500µs pulse width, which
	// corresponds to 0 and 180 degrees.
	return dev.SetPwm(servoChannel, gpio.Duty(onCycle), offDuty)
}
