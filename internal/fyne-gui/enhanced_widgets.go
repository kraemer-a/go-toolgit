package fynegui

import (
	"fmt"
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

// RateLimitStatus displays GitHub API rate limit information
type RateLimitStatus struct {
	widget.BaseWidget

	CoreLabel      *widget.Label
	SearchLabel    *widget.Label
	CoreProgress   *widget.ProgressBar
	SearchProgress *widget.ProgressBar
	LastUpdate     *widget.Label
	container      *fyne.Container

	onRefresh func() // Callback to refresh rate limit data
}

// NewRateLimitStatus creates a new rate limit status widget
func NewRateLimitStatus(onRefresh func()) *RateLimitStatus {
	r := &RateLimitStatus{
		CoreLabel:      widget.NewLabel("Core: -/-"),
		SearchLabel:    widget.NewLabel("Search: -/-"),
		CoreProgress:   widget.NewProgressBar(),
		SearchProgress: widget.NewProgressBar(),
		LastUpdate:     widget.NewLabel(""),
		onRefresh:      onRefresh,
	}

	// Style the labels with center alignment
	r.CoreLabel.TextStyle = fyne.TextStyle{Bold: true}
	r.CoreLabel.Alignment = fyne.TextAlignCenter
	r.SearchLabel.TextStyle = fyne.TextStyle{Bold: true}
	r.SearchLabel.Alignment = fyne.TextAlignCenter
	r.LastUpdate.TextStyle = fyne.TextStyle{Italic: true}
	r.LastUpdate.Alignment = fyne.TextAlignCenter

	// Set initial progress values
	r.CoreProgress.SetValue(1.0)
	r.SearchProgress.SetValue(1.0)

	r.ExtendBaseWidget(r)
	return r
}

// CreateRenderer creates the renderer for the rate limit status
func (r *RateLimitStatus) CreateRenderer() fyne.WidgetRenderer {
	// Create centered core section
	coreSection := container.NewVBox(
		container.NewCenter(r.CoreLabel),
		r.CoreProgress,
	)

	// Create centered search section
	searchSection := container.NewVBox(
		container.NewCenter(r.SearchLabel),
		r.SearchProgress,
	)

	// Create compact layout for status bar
	r.container = container.NewHBox(
		widget.NewIcon(theme.InfoIcon()),
		coreSection,
		widget.NewSeparator(),
		searchSection,
		widget.NewSeparator(),
		container.NewCenter(r.LastUpdate),
	)

	return widget.NewSimpleRenderer(r.container)
}

// UpdateRateLimit updates the display with new rate limit information
func (r *RateLimitStatus) UpdateRateLimit(coreRemaining, coreLimit, searchRemaining, searchLimit int, coreReset, searchReset time.Time) {
	// Calculate values outside of fyne.Do to avoid heavy computation on UI thread
	corePercentage := float64(coreRemaining) / float64(coreLimit)
	searchPercentage := float64(searchRemaining) / float64(searchLimit)

	// Determine importance levels
	var coreImportance, searchImportance widget.Importance
	if corePercentage < 0.1 { // Less than 10%
		coreImportance = widget.DangerImportance
	} else if corePercentage < 0.3 { // Less than 30%
		coreImportance = widget.WarningImportance
	} else {
		coreImportance = widget.SuccessImportance
	}

	if searchPercentage < 0.2 { // Less than 20% (more aggressive for search API)
		searchImportance = widget.DangerImportance
	} else if searchPercentage < 0.5 { // Less than 50%
		searchImportance = widget.WarningImportance
	} else {
		searchImportance = widget.SuccessImportance
	}

	// All UI updates must be done on the main thread
	fyne.Do(func() {
		// Update labels
		r.CoreLabel.SetText(fmt.Sprintf("Core: %d/%d", coreRemaining, coreLimit))
		r.SearchLabel.SetText(fmt.Sprintf("Search: %d/%d", searchRemaining, searchLimit))

		// Update progress bars
		if coreLimit > 0 {
			r.CoreProgress.SetValue(float64(coreRemaining) / float64(coreLimit))
		}
		if searchLimit > 0 {
			r.SearchProgress.SetValue(float64(searchRemaining) / float64(searchLimit))
		}

		// Update timestamp
		r.LastUpdate.SetText(fmt.Sprintf("Updated: %s", time.Now().Format("15:04:05")))

		// Apply importance levels
		r.CoreLabel.Importance = coreImportance
		r.SearchLabel.Importance = searchImportance
	})
}

// ShowError displays an error state
func (r *RateLimitStatus) ShowError(err error) {
	// All UI updates must be done on the main thread
	fyne.Do(func() {
		r.CoreLabel.SetText("Core: Error")
		r.SearchLabel.SetText("Search: Error")
		r.LastUpdate.SetText(fmt.Sprintf("Error: %v", err))
		r.CoreProgress.SetValue(0)
		r.SearchProgress.SetValue(0)

		r.CoreLabel.Importance = widget.DangerImportance
		r.SearchLabel.Importance = widget.DangerImportance
	})
}

// ShowBitbucketMode displays a message indicating Bitbucket mode (no rate limits)
func (r *RateLimitStatus) ShowBitbucketMode() {
	// All UI updates must be done on the main thread
	fyne.Do(func() {
		r.CoreLabel.SetText("Bitbucket Mode")
		r.SearchLabel.SetText("No Rate Limits")
		r.LastUpdate.SetText("Bitbucket provider active")
		r.CoreProgress.SetValue(1.0) // Show as full/unlimited
		r.SearchProgress.SetValue(1.0)

		r.CoreLabel.Importance = widget.MediumImportance
		r.SearchLabel.Importance = widget.MediumImportance
	})
}

// Tapped handles tap events for manual refresh
func (r *RateLimitStatus) Tapped(*fyne.PointEvent) {
	if r.onRefresh != nil {
		r.onRefresh()
	}
}

// MouseIn handles mouse enter events
func (r *RateLimitStatus) MouseIn(*desktop.MouseEvent) {
	// TODO: Implement tooltip showing rate limit explanation
}

// MouseOut handles mouse leave events
func (r *RateLimitStatus) MouseOut() {
	// TODO: Hide tooltip
}

// MouseMoved handles mouse move events
func (r *RateLimitStatus) MouseMoved(*desktop.MouseEvent) {}

// OperationStatus displays current operation type and API usage information
type OperationStatus struct {
	widget.BaseWidget

	StatusLabel   *widget.Label
	APICallsLabel *widget.Label
	LastAPILabel  *widget.Label
	StatusIcon    *widget.Icon
	container     *fyne.Container

	apiCallsCount int
	lastAPITime   time.Time
}

// OperationType represents different types of operations
type OperationType int

const (
	OperationIdle OperationType = iota
	OperationGitClone
	OperationGitValidation
	OperationGitProcessing
	OperationAPIValidation
	OperationAPIProcessing
	OperationAPIRateLimit
)

// NewOperationStatus creates a new operation status widget
func NewOperationStatus() *OperationStatus {
	o := &OperationStatus{
		StatusLabel:   widget.NewLabel("Ready"),
		APICallsLabel: widget.NewLabel("API: 0"),
		LastAPILabel:  widget.NewLabel("Last: Never"),
		StatusIcon:    widget.NewIcon(theme.InfoIcon()),
		apiCallsCount: 0,
	}

	// Style the labels for compact display with center alignment
	o.StatusLabel.TextStyle = fyne.TextStyle{Bold: true}
	o.StatusLabel.Alignment = fyne.TextAlignCenter
	o.APICallsLabel.TextStyle = fyne.TextStyle{Italic: true}
	o.APICallsLabel.Alignment = fyne.TextAlignCenter
	o.LastAPILabel.TextStyle = fyne.TextStyle{Italic: true}
	o.LastAPILabel.Alignment = fyne.TextAlignCenter

	// Make API labels smaller
	o.APICallsLabel.Resize(fyne.NewSize(50, o.APICallsLabel.MinSize().Height))
	o.LastAPILabel.Resize(fyne.NewSize(80, o.LastAPILabel.MinSize().Height))

	o.ExtendBaseWidget(o)
	return o
}

// CreateRenderer creates the renderer for the operation status
func (o *OperationStatus) CreateRenderer() fyne.WidgetRenderer {
	// Create a more compact vertical layout for status bar with centered elements
	statusInfo := container.NewVBox(
		container.NewCenter(o.StatusLabel),
		container.NewCenter(container.NewHBox(o.APICallsLabel, o.LastAPILabel)),
	)

	o.container = container.NewHBox(
		o.StatusIcon,
		statusInfo,
	)

	return widget.NewSimpleRenderer(o.container)
}

// SetOperation updates the current operation status
func (o *OperationStatus) SetOperation(opType OperationType, message string) {
	fyne.Do(func() {
		switch opType {
		case OperationIdle:
			o.StatusIcon.SetResource(theme.InfoIcon())
			o.StatusLabel.SetText("Ready")
			o.StatusLabel.Importance = widget.MediumImportance
		case OperationGitClone:
			o.StatusIcon.SetResource(theme.DownloadIcon())
			o.StatusLabel.SetText("Git Cloning (No API limits)")
			o.StatusLabel.Importance = widget.SuccessImportance
		case OperationGitValidation:
			o.StatusIcon.SetResource(theme.ConfirmIcon())
			o.StatusLabel.SetText("Local Validation")
			o.StatusLabel.Importance = widget.SuccessImportance
		case OperationGitProcessing:
			o.StatusIcon.SetResource(theme.ComputerIcon())
			o.StatusLabel.SetText("Git Processing")
			o.StatusLabel.Importance = widget.SuccessImportance
		case OperationAPIValidation:
			o.StatusIcon.SetResource(theme.WarningIcon())
			o.StatusLabel.SetText("API Validation (Consuming limits)")
			o.StatusLabel.Importance = widget.WarningImportance
		case OperationAPIProcessing:
			o.StatusIcon.SetResource(theme.WarningIcon())
			o.StatusLabel.SetText("API Processing (Consuming limits)")
			o.StatusLabel.Importance = widget.WarningImportance
		case OperationAPIRateLimit:
			o.StatusIcon.SetResource(theme.WarningIcon())
			o.StatusLabel.SetText("Checking Rate Limits")
			o.StatusLabel.Importance = widget.WarningImportance
		}
	})
}

// IncrementAPICall increments the API call counter and updates display
func (o *OperationStatus) IncrementAPICall() {
	o.apiCallsCount++
	o.lastAPITime = time.Now()

	fyne.Do(func() {
		o.APICallsLabel.SetText(fmt.Sprintf("API: %d", o.apiCallsCount))
		o.LastAPILabel.SetText(fmt.Sprintf("Last: %s", o.lastAPITime.Format("15:04:05")))
	})
}

// ResetAPICounter resets the API call counter (typically when rate limits reset)
func (o *OperationStatus) ResetAPICounter() {
	o.apiCallsCount = 0

	fyne.Do(func() {
		o.APICallsLabel.SetText("API: 0")
		o.LastAPILabel.SetText("Last: Reset")
	})
}

// GetAPICallCount returns the current API call count
func (o *OperationStatus) GetAPICallCount() int {
	return o.apiCallsCount
}
