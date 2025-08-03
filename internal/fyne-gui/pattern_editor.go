package fynegui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// PatternEditor is a widget for editing file patterns with tag chips
type PatternEditor struct {
	widget.BaseWidget

	patterns  []string
	onChanged func([]string)

	entry           *widget.Entry
	addButton       *widget.Button
	removeAllButton *widget.Button
	tagsContainer   *fyne.Container
	scrollContainer *container.Scroll

	// Performance optimization fields
	chipPool    []*TagChip // Pool of reusable TagChip widgets
	activeChips []*TagChip // Currently displayed chips
	updatesSuspended bool  // Flag to batch updates
}

// NewPatternEditor creates a new pattern editor
func NewPatternEditor(placeholder string, patterns []string, onChanged func([]string)) *PatternEditor {
	p := &PatternEditor{
		patterns:  patterns,
		onChanged: onChanged,
	}
	p.ExtendBaseWidget(p)

	// Create entry field
	p.entry = widget.NewEntry()
	p.entry.SetPlaceHolder(placeholder)
	p.entry.OnSubmitted = func(text string) {
		p.addPattern(text)
	}

	// Create add button
	p.addButton = widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), func() {
		p.addPattern(p.entry.Text)
	})
	p.addButton.Importance = widget.HighImportance
	
	// Create remove all button
	p.removeAllButton = widget.NewButtonWithIcon("Remove All", theme.DeleteIcon(), func() {
		p.removeAllPatterns()
	})
	p.removeAllButton.Importance = widget.DangerImportance

	// Create tags container
	p.tagsContainer = container.New(layout.NewGridWrapLayout(fyne.NewSize(150, 35)))
	p.scrollContainer = container.NewHScroll(p.tagsContainer)
	p.scrollContainer.SetMinSize(fyne.NewSize(0, 40))

	// Initialize pools
	p.chipPool = make([]*TagChip, 0, 10) // Pre-allocate for 10 chips
	p.activeChips = make([]*TagChip, 0, len(patterns))

	// Initialize with existing patterns
	p.updateTagsOptimized()

	return p
}

// CreateRenderer creates the renderer for the pattern editor
func (p *PatternEditor) CreateRenderer() fyne.WidgetRenderer {
	// Entry container with buttons
	buttonsContainer := container.New(
		layout.NewHBoxLayout(),
		p.addButton,
		p.removeAllButton,
	)
	
	entryContainer := container.New(
		layout.NewBorderLayout(nil, nil, nil, buttonsContainer),
		p.entry,
		buttonsContainer,
	)

	// Main container
	mainContainer := container.New(
		layout.NewVBoxLayout(),
		entryContainer,
		p.scrollContainer,
	)

	return widget.NewSimpleRenderer(mainContainer)
}

// addPattern adds a new pattern
func (p *PatternEditor) addPattern(pattern string) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return
	}

	// Check for duplicates
	for _, existing := range p.patterns {
		if existing == pattern {
			ShowToast(fyne.CurrentApp().Driver().AllWindows()[0], "Pattern already exists", "warning")
			p.entry.SetText("")
			return
		}
	}

	// Add pattern
	p.patterns = append(p.patterns, pattern)
	if !p.updatesSuspended {
		p.updateTagsOptimized()
	}
	p.entry.SetText("")

	if p.onChanged != nil {
		p.onChanged(p.patterns)
	}
}

// removePattern removes a pattern
func (p *PatternEditor) removePattern(pattern string) {
	newPatterns := make([]string, 0, len(p.patterns))
	for _, p := range p.patterns {
		if p != pattern {
			newPatterns = append(newPatterns, p)
		}
	}

	p.patterns = newPatterns
	if !p.updatesSuspended {
		p.updateTagsOptimized()
	}

	if p.onChanged != nil {
		p.onChanged(p.patterns)
	}
}

// removeAllPatterns removes all patterns
func (p *PatternEditor) removeAllPatterns() {
	if len(p.patterns) == 0 {
		return
	}
	
	p.patterns = []string{}
	if !p.updatesSuspended {
		p.updateTagsOptimized()
	}

	if p.onChanged != nil {
		p.onChanged(p.patterns)
	}
}

// updateTags updates the tag chips display
func (p *PatternEditor) updateTags() {
	p.tagsContainer.RemoveAll()

	for _, pattern := range p.patterns {
		patternCopy := pattern // Capture for closure
		chip := NewTagChip(pattern, func() {
			p.removePattern(patternCopy)
		})
		p.tagsContainer.Add(chip)
	}

	p.tagsContainer.Refresh()
}

// SetPatterns sets the patterns
func (p *PatternEditor) SetPatterns(patterns []string) {
	p.patterns = patterns
	if !p.updatesSuspended {
		p.updateTagsOptimized()
	}
}

// GetPatterns returns the current patterns
func (p *PatternEditor) GetPatterns() []string {
	return p.patterns
}

// GetPatternsAsString returns patterns as a comma-separated string
func (p *PatternEditor) GetPatternsAsString() string {
	return strings.Join(p.patterns, ",")
}

// Performance optimization methods

// getPooledChip gets a TagChip from the pool or creates a new one
func (p *PatternEditor) getPooledChip(text string, onDeleted func()) *TagChip {
	var chip *TagChip
	
	// Try to get from pool
	if len(p.chipPool) > 0 {
		chip = p.chipPool[len(p.chipPool)-1]
		p.chipPool = p.chipPool[:len(p.chipPool)-1]
		
		// Reuse the existing chip
		chip.SetText(text)
		chip.SetOnDeleted(onDeleted)
	} else {
		// Create new chip if pool is empty
		chip = NewTagChip(text, onDeleted)
	}
	
	return chip
}

// returnChipToPool returns a TagChip to the pool for reuse
func (p *PatternEditor) returnChipToPool(chip *TagChip) {
	// Reset the chip state
	chip.Reset()
	
	// Add back to pool if we haven't hit capacity
	if len(p.chipPool) < cap(p.chipPool) {
		p.chipPool = append(p.chipPool, chip)
	}
}

// updateTagsOptimized performs differential updates using widget pooling
func (p *PatternEditor) updateTagsOptimized() {
	if p.updatesSuspended {
		return
	}

	// Return excess chips to pool
	for i := len(p.patterns); i < len(p.activeChips); i++ {
		p.tagsContainer.Remove(p.activeChips[i])
		p.returnChipToPool(p.activeChips[i])
	}

	// Resize active chips slice
	if len(p.patterns) < len(p.activeChips) {
		p.activeChips = p.activeChips[:len(p.patterns)]
	}

	// Update or create chips for current patterns
	for i, pattern := range p.patterns {
		patternCopy := pattern // Capture for closure outside the conditional
		if i < len(p.activeChips) {
			// Update existing chip
			chip := p.activeChips[i]
			chip.SetText(pattern)
			chip.SetOnDeleted(func() {
				p.removePattern(patternCopy)
			})
		} else {
			// Create new chip (from pool or new)
			chip := p.getPooledChip(pattern, func() {
				p.removePattern(patternCopy)
			})
			p.activeChips = append(p.activeChips, chip)
			p.tagsContainer.Add(chip)
		}
	}

	p.tagsContainer.Refresh()
}

// SuspendUpdates temporarily disables UI updates for batch operations
func (p *PatternEditor) SuspendUpdates() {
	p.updatesSuspended = true
}

// ResumeUpdates re-enables UI updates and refreshes the display
func (p *PatternEditor) ResumeUpdates() {
	p.updatesSuspended = false
	p.updateTagsOptimized()
}

// GetPoolStats returns statistics about the widget pool (for debugging)
func (p *PatternEditor) GetPoolStats() (poolSize, activeChips int) {
	return len(p.chipPool), len(p.activeChips)
}
