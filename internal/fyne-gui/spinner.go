package fynegui

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// SpinnerStyle represents different spinner animation styles
type SpinnerStyle int

const (
	SpinnerStyleCircles SpinnerStyle = iota
	SpinnerStyleDots
	SpinnerStyleBars
	SpinnerStylePulse
)

// AnimatedSpinner is a custom animated loading spinner widget
type AnimatedSpinner struct {
	widget.BaseWidget

	animation *fyne.Animation
	circles   []*canvas.Circle
	bars      []*canvas.Rectangle
	container *fyne.Container
	running   bool
	size      float32
	style     SpinnerStyle
}

// NewAnimatedSpinner creates a new animated spinner
func NewAnimatedSpinner() *AnimatedSpinner {
	s := &AnimatedSpinner{
		size:  50,
		style: SpinnerStyleCircles,
	}
	s.ExtendBaseWidget(s)
	return s
}

// NewAnimatedSpinnerWithStyle creates a new animated spinner with specific style
func NewAnimatedSpinnerWithStyle(style SpinnerStyle) *AnimatedSpinner {
	s := &AnimatedSpinner{
		size:  50,
		style: style,
	}
	s.ExtendBaseWidget(s)
	return s
}

// CreateRenderer creates the renderer for the spinner
func (s *AnimatedSpinner) CreateRenderer() fyne.WidgetRenderer {
	switch s.style {
	case SpinnerStyleDots:
		return s.createDotsRenderer()
	case SpinnerStyleBars:
		return s.createBarsRenderer()
	case SpinnerStylePulse:
		return s.createPulseRenderer()
	default:
		return s.createCirclesRenderer()
	}
}

func (s *AnimatedSpinner) createCirclesRenderer() fyne.WidgetRenderer {
	// Create 8 circles for the spinner
	numCircles := 8
	s.circles = make([]*canvas.Circle, numCircles)
	objects := make([]fyne.CanvasObject, numCircles)

	// Calculate positions for circles in a circular pattern
	radius := s.size / 3
	centerX := s.size / 2
	centerY := s.size / 2
	circleRadius := s.size / 12

	for i := 0; i < numCircles; i++ {
		angle := float64(i) * (2 * math.Pi / float64(numCircles))
		x := centerX + float32(math.Cos(angle))*radius
		y := centerY + float32(math.Sin(angle))*radius

		circle := canvas.NewCircle(color.RGBA{255, 255, 255, 255}) // Pure white for Adwaita theme visibility
		circle.StrokeWidth = 0
		circle.Resize(fyne.NewSize(circleRadius*2, circleRadius*2))
		circle.Move(fyne.NewPos(x-circleRadius, y-circleRadius))

		s.circles[i] = circle
		objects[i] = circle
	}

	s.container = container.NewWithoutLayout(objects...)

	return widget.NewSimpleRenderer(s.container)
}

func (s *AnimatedSpinner) createDotsRenderer() fyne.WidgetRenderer {
	// Create 3 dots for bouncing animation
	numDots := 3
	s.circles = make([]*canvas.Circle, numDots)
	objects := make([]fyne.CanvasObject, numDots)

	dotSize := s.size / 8
	spacing := s.size / 4
	startX := (s.size - (float32(numDots-1)*spacing + dotSize*2)) / 2
	y := s.size / 2

	for i := 0; i < numDots; i++ {
		x := startX + float32(i)*spacing

		dot := canvas.NewCircle(color.RGBA{255, 255, 255, 255}) // Pure white for Adwaita theme visibility
		dot.StrokeWidth = 0
		dot.Resize(fyne.NewSize(dotSize*2, dotSize*2))
		dot.Move(fyne.NewPos(x, y-dotSize))

		s.circles[i] = dot
		objects[i] = dot
	}

	s.container = container.NewWithoutLayout(objects...)

	return widget.NewSimpleRenderer(s.container)
}

func (s *AnimatedSpinner) createBarsRenderer() fyne.WidgetRenderer {
	// Create 5 bars for wave animation
	numBars := 5
	s.bars = make([]*canvas.Rectangle, numBars)
	objects := make([]fyne.CanvasObject, numBars)

	barWidth := s.size / 10
	barMaxHeight := s.size * 0.6
	spacing := s.size / 8
	startX := (s.size - (float32(numBars-1)*spacing + barWidth)) / 2

	for i := 0; i < numBars; i++ {
		x := startX + float32(i)*spacing

		bar := canvas.NewRectangle(color.RGBA{255, 255, 255, 255}) // Pure white for Adwaita theme visibility
		bar.CornerRadius = barWidth / 2
		bar.Resize(fyne.NewSize(barWidth, barMaxHeight/2))
		bar.Move(fyne.NewPos(x, (s.size-barMaxHeight/2)/2))

		s.bars[i] = bar
		objects[i] = bar
	}

	s.container = container.NewWithoutLayout(objects...)

	return widget.NewSimpleRenderer(s.container)
}

func (s *AnimatedSpinner) createPulseRenderer() fyne.WidgetRenderer {
	// Create 2 circles for pulse animation
	s.circles = make([]*canvas.Circle, 2)
	objects := make([]fyne.CanvasObject, 2)

	for i := 0; i < 2; i++ {
		circle := canvas.NewCircle(color.RGBA{255, 255, 255, 255}) // Pure white for Adwaita theme visibility
		circle.StrokeWidth = 0
		circle.Resize(fyne.NewSize(s.size*0.8, s.size*0.8))
		circle.Move(fyne.NewPos(s.size*0.1, s.size*0.1))

		s.circles[i] = circle
		objects[i] = circle
	}

	s.container = container.NewWithoutLayout(objects...)

	return widget.NewSimpleRenderer(s.container)
}

// Start starts the spinner animation
func (s *AnimatedSpinner) Start() {
	if s.running {
		return
	}
	s.running = true
	s.Show()

	switch s.style {
	case SpinnerStyleDots:
		s.startDotsAnimation()
	case SpinnerStyleBars:
		s.startBarsAnimation()
	case SpinnerStylePulse:
		s.startPulseAnimation()
	default:
		s.startCirclesAnimation()
	}
}

func (s *AnimatedSpinner) startCirclesAnimation() {
	// Create rotation animation
	startTime := time.Now()
	s.animation = fyne.NewAnimation(time.Second*2, func(progress float32) {
		elapsed := time.Since(startTime).Seconds()

		for i, circle := range s.circles {
			// Calculate opacity based on position in the rotation
			phase := math.Mod(elapsed*2+float64(i)*0.125, 1.0)
			opacity := uint8(180 + 75*math.Sin(phase*math.Pi)) // Higher minimum opacity for Adwaita visibility

			// Update circle color with new opacity
			circle.FillColor = color.RGBA{255, 255, 255, opacity}
			circle.Refresh()
		}
	})

	s.animation.RepeatCount = fyne.AnimationRepeatForever
	s.animation.Start()
}

func (s *AnimatedSpinner) startDotsAnimation() {
	// Create bouncing dots animation
	startTime := time.Now()
	s.animation = fyne.NewAnimation(1500*time.Millisecond, func(progress float32) {
		elapsed := time.Since(startTime).Seconds()

		for i, dot := range s.circles {
			// Calculate vertical position based on sine wave with phase offset
			phase := math.Mod(elapsed*2+float64(i)*0.3, 1.0)
			bounce := math.Abs(math.Sin(phase * math.Pi))

			dotSize := s.size / 8
			baseY := s.size / 2
			amplitude := s.size / 4

			y := baseY - dotSize - float32(bounce)*amplitude
			currentPos := dot.Position()
			dot.Move(fyne.NewPos(currentPos.X, y))

			// Also animate opacity - higher minimum for better visibility on dark backgrounds
			opacity := uint8(180 + 75*bounce) // Range: 180-255 instead of 100-255 for better Adwaita visibility
			dot.FillColor = color.RGBA{255, 255, 255, opacity}
			dot.Refresh()
		}
	})

	s.animation.RepeatCount = fyne.AnimationRepeatForever
	s.animation.Start()
}

func (s *AnimatedSpinner) startBarsAnimation() {
	// Create wave animation for bars
	startTime := time.Now()
	s.animation = fyne.NewAnimation(1200*time.Millisecond, func(progress float32) {
		elapsed := time.Since(startTime).Seconds()

		barMaxHeight := s.size * 0.6
		barMinHeight := s.size * 0.2

		for i, bar := range s.bars {
			// Calculate height based on sine wave with phase offset
			phase := math.Mod(elapsed*3+float64(i)*0.2, 1.0)
			heightFactor := (math.Sin(phase*math.Pi*2) + 1) / 2

			barWidth := s.size / 10
			height := barMinHeight + float32(heightFactor)*(barMaxHeight-barMinHeight)

			currentPos := bar.Position()
			bar.Resize(fyne.NewSize(barWidth, height))
			bar.Move(fyne.NewPos(currentPos.X, (s.size-height)/2))

			// Animate opacity for better Adwaita visibility
			opacity := uint8(180 + 75*heightFactor) // Higher minimum opacity
			bar.FillColor = color.RGBA{255, 255, 255, opacity}
			bar.Refresh()
		}
	})

	s.animation.RepeatCount = fyne.AnimationRepeatForever
	s.animation.Start()
}

func (s *AnimatedSpinner) startPulseAnimation() {
	// Create pulsing circles animation
	startTime := time.Now()
	s.animation = fyne.NewAnimation(1500*time.Millisecond, func(progress float32) {
		elapsed := time.Since(startTime).Seconds()

		for i, circle := range s.circles {
			// Each circle pulses with a slight delay
			phase := math.Mod(elapsed+float64(i)*0.5, 1.5)

			if phase < 1.0 {
				// Expand and fade - but maintain higher minimum opacity for Adwaita
				scale := 1.0 + phase*0.5
				opacity := uint8(255 * (1 - phase*0.7)) // Slower fade, higher minimum

				size := s.size * 0.8 * float32(scale)
				pos := (s.size - size) / 2

				circle.Resize(fyne.NewSize(size, size))
				circle.Move(fyne.NewPos(pos, pos))
				circle.FillColor = color.RGBA{255, 255, 255, opacity}
			} else {
				// Reset for next pulse
				circle.FillColor = color.RGBA{255, 255, 255, 0}
			}

			circle.Refresh()
		}
	})

	s.animation.RepeatCount = fyne.AnimationRepeatForever
	s.animation.Start()
}

// Stop stops the spinner animation
func (s *AnimatedSpinner) Stop() {
	s.running = false
	if s.animation != nil {
		s.animation.Stop()
	}
	s.Hide()
}

// MinSize returns the minimum size of the spinner
func (s *AnimatedSpinner) MinSize() fyne.Size {
	return fyne.NewSize(s.size, s.size)
}

// LoadingContainer creates a container with spinner and message
type LoadingContainer struct {
	widget.BaseWidget

	spinner      *AnimatedSpinner
	label        *widget.Label
	progressBar  *widget.ProgressBar
	background   *canvas.Rectangle
	container    *fyne.Container
	showProgress bool
}

// NewLoadingContainer creates a new loading container with spinner and message
func NewLoadingContainer(message string) *LoadingContainer {
	l := &LoadingContainer{}
	l.ExtendBaseWidget(l)

	// Create semi-transparent dark background for overlay effect
	l.background = canvas.NewRectangle(color.RGBA{0, 0, 0, 180}) // Semi-transparent black

	l.spinner = NewAnimatedSpinnerWithStyle(SpinnerStyleDots)
	l.label = widget.NewLabel(message)
	l.label.Alignment = fyne.TextAlignCenter
	l.progressBar = widget.NewProgressBar()
	l.progressBar.Hide() // Hidden by default
	l.showProgress = false

	// Content container with spinner and text
	contentContainer := container.NewVBox(
		container.NewCenter(l.spinner),
		container.NewCenter(l.progressBar),
		container.NewCenter(l.label),
	)

	// Layer the background behind the content using a Max layout
	// Max layout ensures both background and content fill the entire space
	l.container = container.NewMax(
		l.background,     // Background layer (fills entire space)
		contentContainer, // Content layer (centered on top)
	)

	return l
}

// CreateRenderer creates the renderer for the loading container
func (l *LoadingContainer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(l.container)
}

// Start starts the loading animation
func (l *LoadingContainer) Start() {
	fyne.Do(func() {
		l.spinner.Start()
		l.Show()
		l.Refresh()
	})
}

// Stop stops the loading animation
func (l *LoadingContainer) Stop() {
	fyne.Do(func() {
		l.spinner.Stop()
		l.Hide()
	})
}

// SetMessage updates the loading message
func (l *LoadingContainer) SetMessage(message string) {
	l.label.SetText(message)
}

// EnableProgress enables the progress bar display
func (l *LoadingContainer) EnableProgress() {
	fyne.Do(func() {
		l.showProgress = true
		l.progressBar.Show()
		l.progressBar.SetValue(0.0)
		l.Refresh()
	})
}

// DisableProgress disables the progress bar display
func (l *LoadingContainer) DisableProgress() {
	fyne.Do(func() {
		l.showProgress = false
		l.progressBar.Hide()
		l.Refresh()
	})
}

// SetProgress updates the progress bar and message
func (l *LoadingContainer) SetProgress(current, total int, message string) {
	if l.showProgress && total > 0 {
		fyne.Do(func() {
			progress := float64(current) / float64(total)
			l.progressBar.SetValue(progress)

			// Format message with progress
			progressMsg := fmt.Sprintf("%s (%d/%d)", message, current, total)
			l.label.SetText(progressMsg)
		})
	}
}
