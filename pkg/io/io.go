package io

import (
	"fmt"
	"log"
	"math"
	"os"
	"sync"
	"time"

	"github.com/warthog618/go-gpiocdev"
	"gobot.io/x/gobot/drivers/i2c"
	"gobot.io/x/gobot/platforms/raspi"
)

type IO struct {
	chip       *gpiocdev.Chip
	buttons    []Button
	lines      map[int]*gpiocdev.Line
	servos     *i2c.PCA9685Driver // Add the PCA9685 driver field
	mu         sync.Mutex
	motorAngle map[int]*MotorInfo
}
type MotorInfo struct {
	LastDelay    float64
	CurrentAngle int
}

func New(chipset string) *IO {
	c, err := gpiocdev.NewChip(chipset)
	if err != nil {
		fmt.Printf("Opening chip returned error: %s\n", err)
		os.Exit(1)
	}
	info, err := c.LineInfo(0)
	if err != nil {
		fmt.Printf("Reading line info returned error: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("%v\n", info)

	// Initialize the PCA9685 driver
	r := raspi.NewAdaptor()
	servos := i2c.NewPCA9685Driver(r)

	// Start the driver
	if err := servos.Start(); err != nil {
		log.Fatalf("Failed to start PCA9685 driver: %v", err)
	}

	return &IO{
		chip:       c,
		buttons:    nil,
		lines:      make(map[int]*gpiocdev.Line),
		servos:     servos,
		motorAngle: make(map[int]*MotorInfo),
	}
}

func (io *IO) SetXY(channelX, channelY int, x, y uint8) (int, error) {
	wg := sync.WaitGroup{}
	var delay int
	var i int
	wg.Go(func() {
		delay, _ = io.SetServoAngle(channelX, x)

	})
	wg.Go(func() {
		i, _ = io.SetServoAngle(channelY, y)
	})
	wg.Wait()
	// get last angle+set new angle

	return i + delay, nil
}

// SetServoAngle sets the angle for a specific servo channel.
// The angle should be between 0 and 180 degrees.
func (io *IO) SetServoAngle(channel int, angle uint8) (int, error) {
	// Convert the 0-180 degree angle to a 12-bit PWM value (0-4095).
	// The typical pulse width for servos is between 0.5ms and 2.5ms.
	// With a 50Hz PWM frequency, the full cycle is 20ms (4096 ticks).
	// So, a 0.5ms pulse is about 102 ticks, and a 2.5ms pulse is about 512 ticks.
	if channel < 0 {
		return 0, nil
	}
	io.mu.Lock()
	defer io.mu.Unlock()
	if _, ok := io.motorAngle[channel]; !ok {
		io.motorAngle[channel] = &MotorInfo{CurrentAngle: 90, LastDelay: 1151.5}
	}
	// We can use these values for our scaling.
	minPulse := float64(255)  // Corresponds to ~0 degrees
	maxPulse := float64(2048) // Corresponds to ~180 degrees
	//
	//// Scale the angle from 0-180 to the minPulse-maxPulse range
	pulseWidth := minPulse + (maxPulse-minPulse)*float64(angle)/180.0
	//fmt.Printf("Angle:%v : %f\n", angle, pulseWidth)
	// Set the PWM for the specified channel using the 12-bit value
	diff := math.Abs(io.motorAngle[channel].LastDelay - pulseWidth)
	io.motorAngle[channel].LastDelay = pulseWidth
	io.motorAngle[channel].CurrentAngle = int(angle)
	return int(diff), io.servos.SetPWM(channel, 0, uint16(pulseWidth))
}
func (io *IO) GetXY(channelX, channelY int) (int, int) {
	if _, ok := io.motorAngle[channelX]; !ok {
		io.motorAngle[channelX] = &MotorInfo{CurrentAngle: 90, LastDelay: 1151.5}
	}
	if _, ok := io.motorAngle[channelY]; !ok {
		io.motorAngle[channelY] = &MotorInfo{CurrentAngle: 90, LastDelay: 1151.5}
	}
	return io.motorAngle[channelX].CurrentAngle, io.motorAngle[channelY].CurrentAngle
}
func (io *IO) Reset() {
	for k, _ := range io.motorAngle {
		_, _ = io.SetServoAngle(k, 90)
	}
	time.Sleep(1 * time.Second)
}
func (io *IO) Close() {
	io.Reset()
	for _, l := range io.lines {
		_ = l.SetValue(0)
		_ = l.Reconfigure(gpiocdev.AsInput)
		_ = l.Close()
	}
	// Stop and halt the servos, setting them to a neutral position
	if io.servos != nil {
		_ = io.servos.Halt()
		time.Sleep(100 * time.Millisecond) // Give it time to halt
	}
	_ = io.chip.Close()
}
