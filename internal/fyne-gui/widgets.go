package fynegui

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Icon cache to avoid repeated SVG processing
var (
	cachedCancelIcon fyne.Resource
)

func init() {
	// Cache the cancel icon at package initialization
	cachedCancelIcon = theme.CancelIcon()
}

// ToggleSwitch is a custom toggle switch widget
type ToggleSwitch struct {
	widget.BaseWidget

	OnChanged func(bool)
	Checked   bool
	Text      string

	background *canvas.Rectangle
	handle     *canvas.Circle
	label      *widget.Label
	animation  *fyne.Animation
}

// NewToggleSwitch creates a new toggle switch
func NewToggleSwitch(text string, changed func(bool)) *ToggleSwitch {
	t := &ToggleSwitch{
		Text:      text,
		OnChanged: changed,
	}
	t.ExtendBaseWidget(t)
	return t
}

// CreateRenderer creates the renderer for the toggle switch
func (t *ToggleSwitch) CreateRenderer() fyne.WidgetRenderer {
	t.background = canvas.NewRectangle(color.RGBA{200, 200, 200, 255})
	t.background.CornerRadius = 12
	t.background.StrokeWidth = 0

	t.handle = canvas.NewCircle(color.White)
	t.handle.StrokeWidth = 0

	t.label = widget.NewLabel(t.Text)

	objects := []fyne.CanvasObject{
		t.background,
		t.handle,
	}

	return &toggleSwitchRenderer{
		toggle:  t,
		objects: objects,
	}
}

// Tapped handles tap events
func (t *ToggleSwitch) Tapped(_ *fyne.PointEvent) {
	t.SetChecked(!t.Checked)
}

// SetChecked sets the checked state with animation
func (t *ToggleSwitch) SetChecked(checked bool) {
	if t.Checked == checked {
		return
	}

	t.Checked = checked

	// If the handle isn't created yet, just set the state
	if t.handle == nil || t.background == nil {
		if t.OnChanged != nil {
			t.OnChanged(checked)
		}
		return
	}

	// Animate the toggle
	startX := t.handle.Position().X
	var endX float32
	if checked {
		endX = 26
		t.background.FillColor = color.RGBA{R: 59, G: 130, B: 246, A: 255} // Softer blue
	} else {
		endX = 2
		t.background.FillColor = color.RGBA{156, 163, 175, 255} // Gray
	}

	if t.animation != nil {
		t.animation.Stop()
	}

	t.animation = fyne.NewAnimation(200*time.Millisecond, func(progress float32) {
		newX := startX + (endX-startX)*progress
		t.handle.Move(fyne.NewPos(newX, 2))
		t.background.Refresh()
		t.handle.Refresh()
	})

	t.animation.Curve = fyne.AnimationEaseInOut
	t.animation.Start()

	if t.OnChanged != nil {
		t.OnChanged(checked)
	}
}

// toggleSwitchRenderer is the renderer for ToggleSwitch
type toggleSwitchRenderer struct {
	toggle  *ToggleSwitch
	objects []fyne.CanvasObject
}

func (r *toggleSwitchRenderer) Layout(size fyne.Size) {
	r.toggle.background.Resize(fyne.NewSize(36, 18))
	r.toggle.handle.Resize(fyne.NewSize(14, 14))

	if r.toggle.Checked {
		r.toggle.handle.Move(fyne.NewPos(20, 2))
	} else {
		r.toggle.handle.Move(fyne.NewPos(2, 2))
	}
}

func (r *toggleSwitchRenderer) MinSize() fyne.Size {
	return fyne.NewSize(36, 18)
}

func (r *toggleSwitchRenderer) Refresh() {
	if r.toggle.Checked {
		r.toggle.background.FillColor = color.RGBA{R: 59, G: 130, B: 246, A: 255} // Softer blue
		r.toggle.handle.Move(fyne.NewPos(20, 2))
	} else {
		r.toggle.background.FillColor = color.RGBA{156, 163, 175, 255} // Gray
		r.toggle.handle.Move(fyne.NewPos(2, 2))
	}
	r.toggle.background.Refresh()
}

func (r *toggleSwitchRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *toggleSwitchRenderer) Destroy() {}

// ToastNotification represents a toast notification
type ToastNotification struct {
	widget.BaseWidget

	content   *fyne.Container
	message   string
	toastType string // "info", "success", "error", "warning"
	icon      *widget.Icon
	label     *widget.Label
	animation *fyne.Animation
}

// ShowToast shows a toast notification without blocking the main window
func ShowToast(window fyne.Window, message string, toastType string) {

	// Create toast content
	var iconResource fyne.Resource
	var bgColor color.Color

	switch toastType {
	case "success":
		iconResource = theme.ConfirmIcon()
		bgColor = color.RGBA{34, 197, 94, 240} // More opaque for visibility
	case "error":
		iconResource = theme.ErrorIcon()
		bgColor = color.RGBA{255, 85, 85, 240}
	case "warning":
		iconResource = theme.WarningIcon()
		bgColor = color.RGBA{255, 184, 108, 240}
	default:
		iconResource = theme.InfoIcon()
		bgColor = color.RGBA{98, 114, 164, 240}
	}

	icon := widget.NewIcon(iconResource)
	label := widget.NewLabel(message)
	label.TextStyle = fyne.TextStyle{Bold: true}

	// Create background with rounded corners
	bg := canvas.NewRectangle(bgColor)
	bg.CornerRadius = 8

	// Create content container
	content := container.New(
		layout.NewBorderLayout(nil, nil, icon, nil),
		icon,
		label,
	)

	// Create toast container with padding
	toastContainer := container.NewStack(
		bg,
		container.NewPadded(content),
	)

	// Calculate positioning - move to top-center for better visibility
	windowSize := window.Canvas().Size()
	toastSize := fyne.NewSize(300, 60) // Fixed size for consistency
	margin := float32(20)
	finalX := (windowSize.Width - toastSize.Width) / 2 // Center horizontally
	finalY := margin                                   // Top with margin
	startY := -toastSize.Height                        // Start above window

	// Create a tappable button for click-to-dismiss functionality
	dismissButton := widget.NewButton("", func() {
		// This will be set after overlay is created
	})
	dismissButton.Importance = widget.LowImportance

	// Update the toast container to include the dismiss button
	toastContainer = container.NewStack(
		bg,
		container.NewPadded(content),
		dismissButton, // Invisible button overlay for clicking the toast
	)

	// Create minimal overlay using NewWithoutLayout and only add the toast
	overlay := container.NewWithoutLayout(toastContainer)

	// Set up dismiss button callback now that overlay exists
	dismissButton.OnTapped = func() {
		window.Canvas().Overlays().Remove(overlay)
	}

	// Size and position the toast container
	toastContainer.Resize(toastSize)
	toastContainer.Move(fyne.NewPos(finalX, startY))

	// Add to window overlays
	window.Canvas().Overlays().Add(overlay)

	// Animate in from top (slide down)
	animation := fyne.NewAnimation(250*time.Millisecond, func(progress float32) {
		y := startY + (finalY-startY)*progress
		toastContainer.Move(fyne.NewPos(finalX, y))
	})
	animation.Curve = fyne.AnimationEaseOut
	animation.Start()

	// Auto-hide after delay (reduced to 1.5 seconds)
	go func() {
		time.Sleep(1500 * time.Millisecond)

		// Animate out to top (slide up)
		outAnimation := fyne.NewAnimation(250*time.Millisecond, func(progress float32) {
			y := finalY - (finalY-startY)*progress
			toastContainer.Move(fyne.NewPos(finalX, y))
			if progress >= 1.0 {
				window.Canvas().Overlays().Remove(overlay)
			}
		})
		outAnimation.Curve = fyne.AnimationEaseIn
		outAnimation.Start()
	}()
}

// TagChip represents a tag/chip widget
type TagChip struct {
	widget.BaseWidget

	Text      string
	OnDeleted func()

	background *canvas.Rectangle
	label      *widget.Label
	deleteBtn  *widget.Button
	container  *fyne.Container
}

// NewTagChip creates a new tag chip
func NewTagChip(text string, onDeleted func()) *TagChip {
	t := &TagChip{
		Text:      text,
		OnDeleted: onDeleted,
	}
	t.ExtendBaseWidget(t)
	return t
}

// CreateRenderer creates the renderer for the tag chip
func (t *TagChip) CreateRenderer() fyne.WidgetRenderer {
	t.background = canvas.NewRectangle(color.RGBA{107, 114, 128, 40})
	t.background.CornerRadius = 12

	t.label = widget.NewLabel(t.Text)
	t.label.TextStyle = fyne.TextStyle{Bold: true}

	if t.OnDeleted != nil {
		t.deleteBtn = widget.NewButtonWithIcon("", cachedCancelIcon, t.OnDeleted)
		t.deleteBtn.Importance = widget.LowImportance

		t.container = container.NewStack(
			t.background,
			container.NewPadded(
				container.New(
					layout.NewBorderLayout(nil, nil, nil, t.deleteBtn),
					t.label,
					t.deleteBtn,
				),
			),
		)
	} else {
		t.container = container.NewStack(
			t.background,
			container.NewPadded(t.label),
		)
	}

	return widget.NewSimpleRenderer(t.container)
}

// MouseIn handles mouse enter events
func (t *TagChip) MouseIn(*desktop.MouseEvent) {
	t.background.FillColor = color.RGBA{107, 114, 128, 60}
	t.background.Refresh()
}

// MouseOut handles mouse leave events
func (t *TagChip) MouseOut() {
	t.background.FillColor = color.RGBA{107, 114, 128, 40}
	t.background.Refresh()
}

// MouseMoved handles mouse move events
func (t *TagChip) MouseMoved(*desktop.MouseEvent) {}

// SetText updates the text of an existing TagChip (for reuse optimization)
func (t *TagChip) SetText(text string) {
	t.Text = text
	if t.label != nil {
		t.label.SetText(text)
	}
}

// SetOnDeleted updates the deletion callback (for reuse optimization)
func (t *TagChip) SetOnDeleted(onDeleted func()) {
	t.OnDeleted = onDeleted
	if t.deleteBtn != nil && onDeleted != nil {
		t.deleteBtn.OnTapped = onDeleted
	}
}

// Reset prepares the TagChip for reuse by clearing its state
func (t *TagChip) Reset() {
	t.Text = ""
	t.OnDeleted = nil
	if t.label != nil {
		t.label.SetText("")
	}
	if t.deleteBtn != nil {
		t.deleteBtn.OnTapped = nil
	}
}

// IsCreated checks if the TagChip renderer has been created
func (t *TagChip) IsCreated() bool {
	return t.container != nil
}

// EnhancedProgressBar is a progress bar with percentage display
type EnhancedProgressBar struct {
	widget.BaseWidget

	Value       float64
	ShowPercent bool

	bar       *widget.ProgressBar
	label     *widget.Label
	container *fyne.Container
}

// NewEnhancedProgressBar creates a new enhanced progress bar
func NewEnhancedProgressBar() *EnhancedProgressBar {
	p := &EnhancedProgressBar{
		ShowPercent: true,
	}
	p.ExtendBaseWidget(p)
	return p
}

// CreateRenderer creates the renderer for the enhanced progress bar
func (p *EnhancedProgressBar) CreateRenderer() fyne.WidgetRenderer {
	p.bar = widget.NewProgressBar()
	p.label = widget.NewLabel("0%")
	p.label.Alignment = fyne.TextAlignCenter

	p.container = container.NewStack(
		p.bar,
		container.NewCenter(p.label),
	)

	return widget.NewSimpleRenderer(p.container)
}

// SetValue sets the progress value
func (p *EnhancedProgressBar) SetValue(value float64) {
	p.Value = value
	p.bar.SetValue(value)

	if p.ShowPercent {
		percent := int(value * 100)
		p.label.SetText(fmt.Sprintf("%d%%", percent))
	}
}
