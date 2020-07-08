package gost

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// StatusBar is a structure that acts as a proxy to os.Stdout to output a pretty status info
type StatusBar struct {
	tr    time.Duration
	pb    ProgressBar
	tc    int
	isTTY bool
	ds    bool
	ticks chan uint8
}

const ticksChanCapacity = 10

// NewStatusBar makes a new status bar with desired UI and settings
func NewStatusBar(tickRate time.Duration, progressBar ProgressBar, dieSilently bool) *StatusBar {
	return &StatusBar{
		tr:    tickRate,
		ds:    dieSilently,
		isTTY: isTTY(),
		ticks: make(chan uint8, ticksChanCapacity),
		pb:    progressBar,
	}
}

// Run forces progress bar to tick
func (sb *StatusBar) Run() {
	go func() {
		for {
			sb.refresh()
			time.Sleep(time.Duration(sb.tr))
		}
	}()
}

func (sb *StatusBar) refresh() {
	var str string

	// todo implement different ui elements for status bar

	str = sb.pb.nextFrame()
	print(str)
}

func print(str string) {
	str = fmt.Sprintf("\r%s", str)
	os.Stdout.Write([]byte(str))
}

func isTTY() bool {
	return true
	// todo fix
	// fi, err := os.Stdout.Stat()
	// if err != nil {
	// 	return false
	// }

	// return fi.Mode()&os.ModeCharDevice != 0
}

func (sb *StatusBar) checkSelf() {
	// todo implement
}

func (sb *StatusBar) die(error) {
	// todo implement
}

// Tick forces the status bar to refresh
func (sb *StatusBar) Tick(num uint8) {
	defer func() {
		if err := recover(); err != nil {
			sb.die(errors.New("Ticker is closed"))
		}
	}()

	if len(sb.ticks) == ticksChanCapacity {
		sb.die(errors.New("Ticker buffer overflow"))
	}
	sb.ticks <- num
}

// todo implement
// func (sb *sb) Done() {

// }

// ProgressBar is a common interface for all types of progress bars in this package
type ProgressBar interface {
	nextFrame() string
}

/* All kinds of progress bars */

// Helicopter is an infinite progress bar which displays a "rotating" bar
type Helicopter struct {
	charSeq []string
	pos     int
}

// NewHelicopter makes a new Helicopter progress bar
func NewHelicopter() *Helicopter {
	return &Helicopter{
		charSeq: []string{"/", "-", "\\", "|"},
	}
}

func (heli *Helicopter) nextFrame() string {
	if heli.pos >= len(heli.charSeq) {
		heli.pos = 0
	}
	ch := heli.charSeq[heli.pos]
	heli.pos++
	return fmt.Sprintf("[%s]", ch)
}

// Bouncer is an infinite progress bar with one active character moving back and forth from the sides
type Bouncer struct {
	lineLen   int
	direction int8
	pos       int
	charSet   BouncerCharSet
}

// BouncerCharSet defines how the Bouncer progress bar looks by setting individual parts of it
type BouncerCharSet struct {
	Inactive    rune
	Active      rune
	Separator   string
	BorderLeft  string
	BorderRight string
}

var defaultBouncerCharSet = BouncerCharSet{
	Inactive:    '.',
	Active:      '*',
	Separator:   "",
	BorderLeft:  "[",
	BorderRight: "]",
}

func applyDefaults(charSet *BouncerCharSet) {
	if charSet.Active == 0 {
		charSet.Active = defaultBouncerCharSet.Active
	}
	if charSet.Inactive == 0 {
		charSet.Inactive = defaultBouncerCharSet.Inactive
	}
	if charSet.Separator == "" {
		charSet.Separator = defaultBouncerCharSet.Separator
	}
	if charSet.BorderLeft == "" {
		charSet.BorderLeft = defaultBouncerCharSet.BorderLeft
	}
	if charSet.BorderRight == "" {
		charSet.BorderRight = defaultBouncerCharSet.BorderRight
	}
}

func (bouncer *Bouncer) nextFrame() string {
	if bouncer.pos == 0 {
		bouncer.direction = 1
	}
	if bouncer.pos == bouncer.lineLen-1 {
		bouncer.direction = -1
	}
	bar := make([]string, bouncer.lineLen)
	for i := 0; i < bouncer.lineLen; i++ {
		if i == bouncer.pos {
			bar[i] = string(bouncer.charSet.Active)
		} else {
			bar[i] = string(bouncer.charSet.Inactive)
		}
	}
	bouncer.pos += int(bouncer.direction)

	barStr := fmt.Sprintf("%s%s%s",
		bouncer.charSet.BorderLeft,
		strings.Join(bar, bouncer.charSet.Separator),
		bouncer.charSet.BorderRight)

	return fmt.Sprintf(barStr)
}

// NewBouncer makes a new Bouncer progress bar
func NewBouncer(lineLen int, charSet BouncerCharSet) *Bouncer {
	applyDefaults(&charSet)
	return &Bouncer{
		lineLen:   lineLen,
		direction: 1,
		charSet:   charSet,
	}
}
