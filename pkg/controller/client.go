package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/Seann-Moser/lazer/pkg/io"
	"github.com/warthog618/go-gpiocdev/device/rpi"
)

type State int

const (
	Off State = iota
	Configuring
	Slow
	Medium
	Fast
)

type MoveType int

const (
	Straight MoveType = iota
	Curve
	Bounce
	BackAndForth
	Jagged
	Ease
	ZigZag
	Spiral
	Random
	SmoothStep
	Wave
	ShortPause
	LongPause
)

type Controller struct {
	LeftButton  *io.Button
	RightButton *io.Button

	Servos        *io.IO
	motorX        int
	motorY        int
	State         State
	Configuration Configuration //todo load from file and save
	configuring   bool
	configChan    chan bool
	speed         float64
	maxActiveTime time.Duration
	active        time.Time
	pulsePercent  float64
}
type Configuration struct {
	MinXAngle float64
	MinYAngle float64
	MaxXAngle float64
	MaxYAngle float64
	Setting   GeneralSetting
}

const configFile = ".lazer.config.json"

func New(server bool) (*Controller, error) {
	var (
		err                     error
		leftButton, rightButton *io.Button
		client                  *io.IO
	)
	if !server {
		client = io.New("gpiochip0")
		leftButton, err = client.WatchButton(rpi.GPIO26)
		if err != nil {
			log.Printf("Error watching button: %v", err)
			return nil, err
		}

		rightButton, err = client.WatchButton(rpi.GPIO25)
		if err != nil {
			log.Printf("Error watching button: %v", err)
			return nil, err
		}
	}
	config := Configuration{
		MinXAngle: 0,
		MinYAngle: 0,
		MaxXAngle: 180,
		MaxYAngle: 180,
	}
	data, _ := os.ReadFile(configFile)
	if data != nil {
		err = json.Unmarshal(data, &config)
		if err != nil {
			log.Printf("failed loading config file")
		}
	}

	return &Controller{
		LeftButton:    leftButton,
		RightButton:   rightButton,
		Servos:        client,
		motorX:        1,
		motorY:        0,
		State:         0,
		Configuration: config,
		configChan:    make(chan bool, 1),
		speed:         0,
		maxActiveTime: 30 * time.Minute,
		pulsePercent:  .90,
	}, nil
}

func (c *Controller) Close() {
	if c.Servos != nil {
		defer c.Servos.Close()
		c.Servos.Reset()
	}
}

func (c *Controller) Run(ctx context.Context) {
	wg := sync.WaitGroup{}
	go func() {
		c.StartServer(ctx)
	}()
	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case b := <-c.LeftButton.Event:
				if c.configuring {
					select {
					case c.configChan <- true:
					default:
					}
				}
				if b.Duration > 2*time.Second {
					fmt.Printf("starting")
					go c.ChangeState(ctx, Configuring)
				} else {
					c.ChangeState(ctx, Off)
				}
			case b := <-c.RightButton.Event:
				select {
				case c.configChan <- true:
				default:
				}
				if b.Duration > 2*time.Second {
					c.State = Off
					continue
				}
				_ = c.Servos.SetPinState(23, 1)
				c.active = time.Now()
				if c.State <= Configuring {
					c.State = Slow
				} else if c.State < Fast {
					c.State = State(int(c.State) + 1)
				} else {
					c.State = Off
				}
				c.ChangeState(ctx, c.State)
			}
		}
	})
	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if c.State <= Configuring {
					_ = c.Servos.SetPinState(23, 0)
					time.Sleep(1 * time.Second)
					continue
				}
				if rand.Float64() > c.pulsePercent {
					_ = c.Servos.SetPinState(23, 0)
				} else {
					_ = c.Servos.SetPinState(23, 1)
				}
				x, y := c.getRandomXY()
				t := getRandomMoveType()
				fmt.Printf("x: %d y:%d MovementType:%d\n", x, y, t)
				err := c.moveTo(ctx, x, y, t)
				if err != nil {
					log.Printf("failed to  move to point")
					continue
				}

			}
		}
	})
	wg.Go(func() {
		ticker := time.NewTicker(time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if c.State <= Configuring {
					continue
				}
				if time.Since(c.active) > c.maxActiveTime {
					c.ChangeState(ctx, Off)
				}
			}
		}
	})
	wg.Go(func() {
		ticker := time.NewTicker(time.Minute)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				today := DaysOfWeek((int(now.Weekday()) + 6) % 7) // Map to your enum: Monday = 0

				scheduleList, ok := c.Configuration.Setting.Schedule[today]
				if !ok {
					continue
				}
				for _, schedule := range scheduleList {
					if isNowInSchedule(now, schedule) {
						c.active = time.Now()
						c.ChangeState(ctx, schedule.State)
						break // Only apply first matching schedule
					}
				}
			}
		}
	})
	wg.Wait()
}
func (s *State) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}

	switch str {
	case "Off":
		*s = Off
	case "Configuring":
		*s = Configuring
	case "Slow":
		*s = Slow
	case "Medium":
		*s = Medium
	case "Fast":
		*s = Fast
	default:
		return fmt.Errorf("unknown state: %s", str)
	}
	return nil
}

func isNowInSchedule(now time.Time, schedule GeneralSchedule) bool {
	st, err := time.Parse("15:04", schedule.StartTime)
	if err != nil {
		log.Printf("error:%s", err.Error())
		return false
	}
	start := time.Date(
		now.Year(), now.Month(), now.Day(),
		st.Hour(), st.Minute(), 0, 0,
		now.Location(),
	)
	end := start.Add(schedule.OnDuration)

	return now.After(start) && now.Before(end)
}
func (c *Controller) ChangeState(ctx context.Context, state State) {
	if c.configuring {
		return
	}
	switch state {
	case Off:
		c.Servos.Reset()
		_ = c.Servos.SetPinState(23, 0)
	case Configuring:
		c.configuring = true
		c.Configure(ctx)
	case Slow:
		c.speed = 0.25
	case Medium:
		c.speed = 0.5
	case Fast:
		c.speed = 1
	}

	c.State = state
}

func (c *Controller) getRandomXY() (uint8, uint8) {
	rand.Seed(time.Now().UnixNano())
	// Generate random X within configured range
	x := c.Configuration.MinXAngle + rand.Float64()*(c.Configuration.MaxXAngle-c.Configuration.MinXAngle)
	y := c.Configuration.MinYAngle + rand.Float64()*(c.Configuration.MaxYAngle-c.Configuration.MinYAngle)
	// Clamp to 0–180 just in case, then convert to uint8
	if x < 0 {
		x = 0
	} else if x > 180 {
		x = 180
	}

	if y < 0 {
		y = 0
	} else if y > 180 {
		y = 180
	}

	return uint8(x), uint8(y)
}

func (c *Controller) moveTo(ctx context.Context, x, y uint8, moveType MoveType) error {
	currentX, currentY := c.Servos.GetXY(c.motorX, c.motorY)

	dx := float64(int(x) - int(currentX))
	dy := float64(int(y) - int(currentY))

	// Calculate Euclidean distance
	distance := math.Sqrt(dx*dx + dy*dy)

	// Define step size (units per step), then calculate steps
	const stepSize = 2.0 // you can tune this
	steps := int(math.Ceil(distance / stepSize))
	if steps < 1 {
		steps = 1
	}

	for i := 1; i <= steps; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			t := float64(i) / float64(steps)

			var xi, yi int
			switch moveType {
			case Straight:
				xi = int(float64(currentX) + dx*t)
				yi = int(float64(currentY) + dy*t)
			case Curve:
				xi = int(float64(currentX) + dx*t)
				yi = int(float64(currentY) + dy*t + 10*math.Sin(t*math.Pi))
			case Bounce:
				bounceT := t + 0.1*math.Sin(3*math.Pi*t)
				xi = int(float64(currentX) + dx*bounceT)
				yi = int(float64(currentY) + dy*bounceT)
			case BackAndForth:
				bt := t
				if t < 0.5 {
					bt = t * 2
				} else {
					bt = 1 - ((t - 0.5) * 2)
				}
				xi = int(float64(currentX) + dx*bt)
				yi = int(float64(currentY) + dy*bt)
			case Jagged:
				jag := rand.Intn(5) - 2
				xi = int(float64(currentX)+dx*t) + jag
				yi = int(float64(currentY)+dy*t) - jag
			case Ease:
				easeT := -0.5 * (math.Cos(math.Pi*t) - 1)
				xi = int(float64(currentX) + dx*easeT)
				yi = int(float64(currentY) + dy*easeT)
			case ZigZag:
				offset := int(5 * math.Sin(10*t*math.Pi))
				xi = int(float64(currentX) + dx*t)
				yi = int(float64(currentY)+dy*t) + offset
			case Spiral:
				radius := 10.0 * (1 - t)
				angle := 4 * math.Pi * t
				xi = int(float64(currentX) + dx*t + radius*math.Cos(angle))
				yi = int(float64(currentY) + dy*t + radius*math.Sin(angle))
			case Random:
				xi = int(float64(currentX)+dx*t) + rand.Intn(7) - 3
				yi = int(float64(currentY)+dy*t) + rand.Intn(7) - 3
			case SmoothStep:
				smoothT := t * t * (3 - 2*t)
				xi = int(float64(currentX) + dx*smoothT)
				yi = int(float64(currentY) + dy*smoothT)
			case Wave:
				amp := 8.0
				freq := 4.0
				xi = int(float64(currentX) + dx*t)
				yi = int(float64(currentY) + dy*t + amp*math.Sin(freq*t*math.Pi))
			case ShortPause:
				time.Sleep(time.Second * time.Duration(1+rand.Intn(5)))
			case LongPause:
				time.Sleep(time.Second * time.Duration(5+rand.Intn(30)))
			default:
				xi = int(float64(currentX) + dx*t)
				yi = int(float64(currentY) + dy*t)
			}

			// Clamp to 0–180
			if xi < 0 {
				xi = 0
			} else if xi > 180 {
				xi = 180
			}
			if yi < 0 {
				yi = 0
			} else if yi > 180 {
				yi = 180
			}
			waitDelay, err := c.Servos.SetXY(c.motorX, c.motorY, uint8(xi), uint8(yi))
			if err != nil {
				return err
			}

			// Speed control
			speed := c.speed * 100
			if speed < 1 {
				speed = 1
			}
			if speed > 100 {
				speed = 100
			}
			speedMultiplier := 100.0 / float64(speed)
			delay := time.Duration(float64(waitDelay) * speedMultiplier)

			time.Sleep(time.Millisecond * delay)
		}
	}

	return nil
}

func (c *Controller) Configure(ctx context.Context) {
	//save file afster configuration
	defer func() {
		c.configuring = false
	}()
	fmt.Printf("Configuring....")

	c.Configuration.MinXAngle, c.Configuration.MaxXAngle = c.motorConfig(ctx, c.motorX)
	c.Configuration.MinYAngle, c.Configuration.MaxYAngle = c.motorConfig(ctx, c.motorY)

	data, err := json.Marshal(c.Configuration)
	if err != nil {
		log.Printf("failed marshalling config file")
	}
	err = os.WriteFile(configFile, data, 0777)
	if err != nil {
		log.Printf("failed saving config file")
	}
	fmt.Printf("Finishing Configuration")
	c.Servos.Reset()
}

func (c *Controller) motorConfig(ctx context.Context, pin int) (float64, float64) {
	start := false
	var startValue float64
	c.Servos.SetXY(pin, 0, uint8(0), 0)
	time.Sleep(time.Second)
	for i := 0; i < 180; i++ { //9-92
		d, _ := c.Servos.SetXY(pin, -1, uint8(i), 0)
		time.Sleep(time.Duration(d) * time.Millisecond * 4)
		select {
		case <-ctx.Done():
			return 0, 180
		case <-c.configChan:
			println("button press")
			if start {
				return startValue, float64(i)
			} else {
				start = true
				startValue = float64(i)
			}

		default:

		}
	}
	return startValue, 180
}
func getRandomMoveType() MoveType {
	// Total number of move types (update this if you add more types)
	const moveTypeCount = int(Wave + 1)

	return MoveType(rand.Intn(moveTypeCount))
}
