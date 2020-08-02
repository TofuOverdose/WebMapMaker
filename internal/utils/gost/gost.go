package gost

// Dedicated to Pauly, the love of my life ❤️

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"golang.org/x/crypto/ssh/terminal"
)

// StatusBar is a structure that acts as a proxy to Stdout to make your logs pretty
type StatusBar struct {
	tickRate time.Duration
	isTTY    bool
	widgets  []Widget
	disabled bool
}

const ticksChanCapacity = 10
const stateChanTotalCapacity = 100

// NewStatusBar makes a new status bar with desired UI and settings
func NewStatusBar(tickRate time.Duration, widgets ...Widget) *StatusBar {
	return &StatusBar{
		tickRate: tickRate,
		isTTY:    isTTY(),
		widgets:  widgets,
	}
}

// Run forces progress bar to tick
func (sb *StatusBar) Run() {
	go func() {
		for {
			if sb.disabled {
				sb.remove()
				return
			}
			err := sb.print()
			if err != nil {
				panic(err)
			}
			time.Sleep(time.Duration(sb.tickRate))
		}
	}()
}

func (sb *StatusBar) render() []byte {
	res := make([]byte, len(sb.widgets))
	for _, w := range sb.widgets {
		bs, _ := w.render()
		res = append(res, []byte(bs)...)
	}
	return res
}

func (sb *StatusBar) remove() {
	output := []byte(fmt.Sprintf("\r%s", overwriteBlank([]byte{})))
	os.Stdout.Write(output)
}

func (sb *StatusBar) print() error {
	if sb.disabled {
		return nil
	}
	output := fmt.Sprintf("\r%s", sb.render())
	_, err := os.Stdout.Write(overwriteBlank([]byte(output)))
	return err
}

func isTTY() bool {
	return terminal.IsTerminal(int(os.Stdout.Fd()))
}

func getTerminalWidth() int {
	w, _, _ := terminal.GetSize(int(os.Stdout.Fd()))
	return w
}

func overwriteBlank(data []byte) []byte {
	remainder := getTerminalWidth() - len(data)
	if remainder <= 0 {
		return data
	}
	whitespace := []byte(strings.Repeat(" ", remainder))
	return append(data, whitespace...)
}

// Write writes len(data) bytes to the File.
// It returns the number of bytes written and an error, if any.
// Write returns a non-nil error when n != len(b).
func (sb *StatusBar) Write(data []byte) (int, error) {
	output := []byte(fmt.Sprintf("\r%s\n", overwriteBlank(data)))
	n, err := os.Stdout.Write([]byte(output))
	if err != nil {
		return 0, err
	}

	err = sb.print()
	if err != nil {
		return 0, err
	}

	return n, nil
}

// WriteString is like Write, but writes the contents of string s rather than
// a slice of bytes.
func (sb StatusBar) WriteString(s string) (ret int, err error) {
	return sb.Write([]byte(s))
}

// Print formats using the default formats for its operands and writes to standard output.
// Spaces are added between operands when neither is a string.
// It returns the number of bytes written and any write error encountered.
func (sb StatusBar) Print(a ...interface{}) (n int, err error) {
	return sb.WriteString(fmt.Sprint(a...))
}

// Println formats using the default formats for its operands and writes to standard output.
// Spaces are always added between operands and a newline is appended.
// It returns the number of bytes written and any write error encountered.
func (sb StatusBar) Println(a ...interface{}) (n int, err error) {
	return sb.WriteString(fmt.Sprintln(a...))
}

// Printf formats according to a format specifier and writes to standard output.
// It returns the number of bytes written and any write error encountered.
func (sb StatusBar) Printf(format string, a ...interface{}) (n int, err error) {
	return sb.WriteString(fmt.Sprintf(format, a...))
}

func (sb *StatusBar) Close() {
	sb.disabled = true
}

// Widget is an interface that progress bar can consume to render it on itself
type Widget interface {
	// SetData updates some related data to be used for rendering
	// Not every Widget can consume data though, so behavior migth be undefined for this method
	SetData(data interface{})
	render() (string, error)
}

// Display is a kind of Widget that renders as a standard template.Template
type Display struct {
	tplt       *template.Template
	lastRender string
	curData    interface{}
}

// NewDisplay makes a new instance of Display
func NewDisplay(templateString string, initialValues interface{}) (*Display, error) {
	t, err := template.New("").Parse(templateString)
	if err != nil {
		return nil, err
	}

	w := &Display{
		tplt:       t,
		lastRender: "",
		curData:    initialValues,
	}

	_, err = w.render()
	if err != nil {
		return nil, err
	}
	return w, nil
}

// SetData updates values that will be passed to the underlying template
func (w *Display) SetData(data interface{}) {
	w.curData = data
}

func (w *Display) render() (string, error) {
	buf := bytes.NewBufferString("")
	err := w.tplt.Execute(buf, w.curData)
	// If something went wrong, just return cached render with error
	if err != nil {
		return w.lastRender, err
	}

	// Otherwise cache this render and return it
	w.lastRender = buf.String()
	return w.lastRender, nil
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

// SetData does nothing for Helicopter
func (heli *Helicopter) SetData(data interface{}) {
	return
}

func (heli *Helicopter) render() (string, error) {
	if heli.pos >= len(heli.charSeq) {
		heli.pos = 0
	}
	ch := heli.charSeq[heli.pos]
	heli.pos++
	return fmt.Sprintf("[%s]", ch), nil
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

// SetData does nothing for Bouncer
func (bouncer *Bouncer) SetData(data interface{}) {
	return
}

func (bouncer *Bouncer) render() (string, error) {
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

	return fmt.Sprintf(barStr), nil
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

type TimeFormatter func(time.Duration) string

const (
	unitFormatMs  = "ms"
	unitFormatSec = "sec"
	unitFormatMin = "min"
)

// TimeFormatterAdaptive chooses the most fitting unit of time
func TimeFormatterAdaptive(dur time.Duration) string {
	ms := dur.Milliseconds()
	str := TimeFormatterMilliseconds(dur)
	if ms > 999 {
		str = TimeFormatterSeconds(dur)
	}
	if ms > 59999 {
		str = TimeFormatterMinutes(dur)
	}
	return str
}

// TimeFormatterMilliseconds displays time in milliseconds
func TimeFormatterMilliseconds(dur time.Duration) string {
	return strconv.FormatInt(dur.Microseconds(), 10) + " " + unitFormatMs
}

// TimeFormatterSeconds displays time in seconds
func TimeFormatterSeconds(dur time.Duration) string {
	return strconv.FormatFloat(dur.Seconds(), 'f', 2, 64) + " " + unitFormatSec
}

// TimeFormatterMinutes displays time in minutes
func TimeFormatterMinutes(dur time.Duration) string {
	return strconv.FormatFloat(dur.Minutes(), 'f', 2, 64) + " " + unitFormatMin
}

type timerCongif struct {
	startImmediately bool
	showUnit         bool
	timeFormatter    TimeFormatter
	decoration       struct {
		left  string
		right string
	}
}

type TimerOption func() func(timer *Timer)

// TimerOptionsStartImmediately - set val to TRUE to start counting before the first render
// Default value is FALSE
func TimerOptionsStartImmediately(val bool) func(timer *Timer) {
	return func(timer *Timer) {
		timer.config.startImmediately = val
	}
}

// TimerOptionShowUnit - set val to TRUE to append unit text on render
// Default value is FALSE
func TimerOptionShowUnit(val bool) func(timer *Timer) {
	return func(timer *Timer) {
		timer.config.showUnit = val
	}
}

// TimerOptionTimeFormatter - set val to one of the functions of type TimeFormatter to define the desired display unit
// TimerOptionShowUnit should be set to TRUE
// Default value is TimeFormatterAdaptive
func TimerOptionTimeFormatter(val TimeFormatter) func(timer *Timer) {
	return func(timer *Timer) {
		timer.config.timeFormatter = val
	}
}

// TimerOptionSetDecoration - set val to TRUE to append unit text on render
// Default is TimerUnitAdaptive
func TimerOptionSetDecoration(left, right string) func(timer *Timer) {
	return func(timer *Timer) {
		timer.config.decoration.left = left
		timer.config.decoration.right = right
	}
}

// Timer is a type of widget that renders the time elapsed after its start
type Timer struct {
	startTime   time.Time
	config      timerCongif
	initialized bool
}

// NewTimer makes new Timer widget
func NewTimer(options ...func(*Timer)) *Timer {
	timer := &Timer{}
	// Apply default configs
	timer.config = timerCongif{
		startImmediately: false,
		showUnit:         false,
		timeFormatter:    TimeFormatterAdaptive,
	}
	// Reapply options set by user
	for _, opt := range options {
		opt(timer)
	}

	if timer.config.startImmediately {
		timer.initialized = true
		timer.startTime = time.Now()
	}
	return timer
}

// SetData does nothing for Timer
func (timer *Timer) SetData(data interface{}) {
	// todo allow setting time manually through this method
	return
}

func (timer *Timer) render() (string, error) {
	var elapsed time.Duration
	if timer.initialized {
		elapsed = time.Now().Sub(timer.startTime)
	} else {
		timer.initialized = true
		timer.startTime = time.Now()
		elapsed = 0
	}
	var str string
	if timer.config.showUnit {
		str = timer.config.timeFormatter(elapsed)
	} else {
		str = strconv.FormatInt(elapsed.Microseconds(), 10)
	}
	str = fmt.Sprintf("%s%s%s", timer.config.decoration.left, str, timer.config.decoration.right)
	return str, nil
}
