package fynegui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// EnhancedCard is a card widget with shadow and hover effects
type EnhancedCard struct {
	widget.BaseWidget

	Title    string
	Subtitle string
	Content  fyne.CanvasObject

	card       *widget.Card
	shadow     *canvas.Rectangle
	background *canvas.Rectangle
	container  *fyne.Container
	isHovered  bool
}

// NewEnhancedCard creates a new enhanced card
func NewEnhancedCard(title, subtitle string, content fyne.CanvasObject) *EnhancedCard {
	e := &EnhancedCard{
		Title:    title,
		Subtitle: subtitle,
		Content:  content,
	}
	e.ExtendBaseWidget(e)
	return e
}

// CreateRenderer creates the renderer for the enhanced card
func (e *EnhancedCard) CreateRenderer() fyne.WidgetRenderer {
	// Create shadow
	e.shadow = canvas.NewRectangle(color.RGBA{0, 0, 0, 30})
	e.shadow.CornerRadius = 8

	// Create background
	e.background = canvas.NewRectangle(theme.BackgroundColor())
	e.background.CornerRadius = 8
	e.background.StrokeColor = color.RGBA{200, 200, 200, 100}
	e.background.StrokeWidth = 1

	// Create card
	e.card = widget.NewCard(e.Title, e.Subtitle, e.Content)

	// Create container with shadow effect
	e.container = container.NewWithoutLayout(
		e.shadow,
		e.background,
		container.NewPadded(e.card),
	)

	return &enhancedCardRenderer{
		card:    e,
		objects: []fyne.CanvasObject{e.container},
	}
}

// MouseIn handles mouse enter events
func (e *EnhancedCard) MouseIn(*desktop.MouseEvent) {
	e.isHovered = true
	e.shadow.FillColor = color.RGBA{0, 0, 0, 50}
	e.shadow.Refresh()
	e.background.StrokeColor = theme.PrimaryColor()
	e.background.Refresh()
}

// MouseOut handles mouse leave events
func (e *EnhancedCard) MouseOut() {
	e.isHovered = false
	e.shadow.FillColor = color.RGBA{0, 0, 0, 30}
	e.shadow.Refresh()
	e.background.StrokeColor = color.RGBA{200, 200, 200, 100}
	e.background.Refresh()
}

// MouseMoved handles mouse move events
func (e *EnhancedCard) MouseMoved(*desktop.MouseEvent) {}

// enhancedCardRenderer is the renderer for EnhancedCard
type enhancedCardRenderer struct {
	card    *EnhancedCard
	objects []fyne.CanvasObject
}

func (r *enhancedCardRenderer) Layout(size fyne.Size) {
	// Position shadow slightly offset
	shadowOffset := float32(4)
	r.card.shadow.Resize(size)
	r.card.shadow.Move(fyne.NewPos(shadowOffset, shadowOffset))

	// Position background
	r.card.background.Resize(size)
	r.card.background.Move(fyne.NewPos(0, 0))

	// Position card content
	r.card.container.Resize(size)
}

func (r *enhancedCardRenderer) MinSize() fyne.Size {
	return r.card.card.MinSize().Add(fyne.NewSize(8, 8))
}

func (r *enhancedCardRenderer) Refresh() {
	r.card.card.Refresh()
}

func (r *enhancedCardRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *enhancedCardRenderer) Destroy() {}

// Tooltip is a custom tooltip widget
type Tooltip struct {
	widget.BaseWidget

	Text       string
	container  *fyne.Container
	background *canvas.Rectangle
	label      *widget.Label
}

// NewTooltip creates a new tooltip
func NewTooltip(text string) *Tooltip {
	t := &Tooltip{
		Text: text,
	}
	t.ExtendBaseWidget(t)
	return t
}

// CreateRenderer creates the renderer for the tooltip
func (t *Tooltip) CreateRenderer() fyne.WidgetRenderer {
	t.background = canvas.NewRectangle(color.RGBA{40, 40, 40, 230})
	t.background.CornerRadius = 4

	t.label = widget.NewLabel(t.Text)
	t.label.TextStyle = fyne.TextStyle{Bold: true}

	t.container = container.NewStack(
		t.background,
		container.NewPadded(t.label),
	)

	return widget.NewSimpleRenderer(t.container)
}

// ShowTooltip shows a tooltip near the given position
func ShowTooltip(window fyne.Window, text string, pos fyne.Position) {
	tooltip := NewTooltip(text)

	// Create popup
	popup := widget.NewPopUp(tooltip, window.Canvas())

	// Position tooltip
	tooltipSize := tooltip.MinSize()
	windowSize := window.Canvas().Size()

	// Adjust position to ensure tooltip stays within window
	if pos.X+tooltipSize.Width > windowSize.Width {
		pos.X = windowSize.Width - tooltipSize.Width - 10
	}
	if pos.Y+tooltipSize.Height > windowSize.Height {
		pos.Y = windowSize.Height - tooltipSize.Height - 10
	}

	popup.Move(pos)
	popup.Show()

	// Auto-hide after delay
	go func() {
		time.Sleep(2 * time.Second)
		popup.Hide()
	}()
}

// FloatingActionButton is a circular button that floats above content
type FloatingActionButton struct {
	widget.BaseWidget

	Icon     fyne.Resource
	OnTapped func()

	background *canvas.Circle
	icon       *widget.Icon
	container  *fyne.Container
}

// NewFloatingActionButton creates a new floating action button
func NewFloatingActionButton(icon fyne.Resource, tapped func()) *FloatingActionButton {
	f := &FloatingActionButton{
		Icon:     icon,
		OnTapped: tapped,
	}
	f.ExtendBaseWidget(f)
	return f
}

// CreateRenderer creates the renderer for the FAB
func (f *FloatingActionButton) CreateRenderer() fyne.WidgetRenderer {
	f.background = canvas.NewCircle(theme.PrimaryColor())

	f.icon = widget.NewIcon(f.Icon)

	f.container = container.NewStack(
		f.background,
		container.NewCenter(f.icon),
	)

	return widget.NewSimpleRenderer(f.container)
}

// Tapped handles tap events
func (f *FloatingActionButton) Tapped(*fyne.PointEvent) {
	if f.OnTapped != nil {
		f.OnTapped()
	}
}

// MouseIn handles mouse enter events
func (f *FloatingActionButton) MouseIn(*desktop.MouseEvent) {
	f.background.FillColor = theme.HoverColor()
	f.background.Refresh()
}

// MouseOut handles mouse leave events
func (f *FloatingActionButton) MouseOut() {
	f.background.FillColor = theme.PrimaryColor()
	f.background.Refresh()
}

// MouseMoved handles mouse move events
func (f *FloatingActionButton) MouseMoved(*desktop.MouseEvent) {}

// MinSize returns the minimum size of the FAB
func (f *FloatingActionButton) MinSize() fyne.Size {
	return fyne.NewSize(56, 56)
}
