package fynegui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// FileOperationRuleWidget represents a single file operation rule in the UI
type FileOperationRuleWidget struct {
	SourcePathEntry     *widget.Entry
	TargetPathEntry     *widget.Entry
	SearchModeSelect    *widget.Select
	OperationTypeSelect *widget.Select
	Container           *fyne.Container
}

// createFileOperationsTab creates the File Operations tab content
func (f *FyneApp) createFileOperationsTab() fyne.CanvasObject {
	// Initialize containers
	f.fileOperationsRulesContainer = container.New(layout.NewVBoxLayout())
	f.fileOpsRepoContainer = container.New(layout.NewVBoxLayout())

	// Left panel with file operation rules
	leftColumn := f.createFileOperationsLeftColumn()

	// Right panel with repository selection (based on String Replacement tab structure)
	rightColumn := f.createFileOperationsRightColumn()

	// Create horizontal split like in String Replacement tab
	mainContent := container.NewHSplit(leftColumn, rightColumn)
	mainContent.SetOffset(0.55) // Start with left column slightly larger

	// Process buttons at bottom
	dryRunBtn := widget.NewButtonWithIcon("🔍 Dry Run", theme.VisibilityIcon(), f.handleFileOperationsDryRun)
	dryRunBtn.Importance = widget.WarningImportance

	processBtn := widget.NewButtonWithIcon("▶ Process File Operations", theme.MediaPlayIcon(), f.handleFileOperationsProcess)
	processBtn.Importance = widget.HighImportance

	buttonsContainer := container.New(layout.NewHBoxLayout(),
		dryRunBtn,
		processBtn,
		layout.NewSpacer(),
	)

	return container.New(
		layout.NewBorderLayout(nil, buttonsContainer, nil, nil),
		mainContent,
		buttonsContainer,
	)
}

// createFileOperationsLeftColumn creates the left column with file operation rules
func (f *FyneApp) createFileOperationsLeftColumn() *container.Scroll {
	// Title
	titleLabel := widget.NewLabelWithStyle("File Operations Rules", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	// Add rule button
	addRuleBtn := widget.NewButtonWithIcon("Add File Rule", theme.ContentAddIcon(), func() {
		f.addFileOperationRule()
	})
	addRuleBtn.Importance = widget.HighImportance

	// Clear all button
	clearAllBtn := widget.NewButtonWithIcon("Clear All", theme.DeleteIcon(), func() {
		f.fileOperationsRulesContainer.RemoveAll()
		f.addFileOperationRule() // Add one empty rule
	})
	clearAllBtn.Importance = widget.DangerImportance

	// Header with title and buttons
	headerContainer := container.New(layout.NewVBoxLayout(),
		titleLabel,
		container.New(layout.NewHBoxLayout(),
			addRuleBtn,
			clearAllBtn,
			layout.NewSpacer(),
		),
		widget.NewSeparator(),
	)

	// Rules scroll container
	f.fileOperationsRulesScroll = container.NewVScroll(f.fileOperationsRulesContainer)
	f.fileOperationsRulesScroll.SetMinSize(fyne.NewSize(0, 200))

	// Add initial rule
	f.addFileOperationRule()

	// Instructions card
	instructions := widget.NewCard("Instructions", "", widget.NewLabel(
		"• Source Path: File or pattern to find (e.g., 'config.yml' or '*.yml')\n"+
			"• Target Path: New name or location (e.g., 'config.yaml' or '*.yaml')\n"+
			"• Search Mode:\n"+
			"  - Exact: Search only at the specified path\n"+
			"  - Filename: Search entire repository for filename\n"+
			"• Operation: Move to relocate, Rename to change name",
	))

	// PR/Push settings (moved from right column)
	f.fileOpsPRTitleEntry = widget.NewEntry()
	f.fileOpsPRTitleEntry.SetPlaceHolder("PR Title (e.g., 'chore: rename configuration files')")
	f.fileOpsPRTitleEntry.SetText("chore: file operations")

	f.fileOpsPRBodyEntry = widget.NewMultiLineEntry()
	f.fileOpsPRBodyEntry.SetPlaceHolder("PR Description...")
	f.fileOpsPRBodyEntry.SetText("Automated file operations performed by go-toolgit tool.")
	f.fileOpsPRBodyEntry.SetMinRowsVisible(3)

	f.fileOpsBranchPrefixEntry = widget.NewEntry()
	f.fileOpsBranchPrefixEntry.SetPlaceHolder("Branch prefix (e.g., 'file-ops')")
	f.fileOpsBranchPrefixEntry.SetText("file-ops")

	// Direct push toggle
	f.fileOpsPushDirectToggle = NewToggleSwitch("🔄 Push directly to default branch", func(checked bool) {
		f.updateFileOpsPushMethodStatus(checked)
	})

	pushMethodContainer := container.New(layout.NewVBoxLayout(),
		f.fileOpsPushDirectToggle,
		widget.NewLabel("⚠️ Direct push bypasses PR review process"),
	)

	// Create PR fields container that can be shown/hidden
	f.fileOpsPRSettingsContainer = container.New(layout.NewVBoxLayout(),
		widget.NewLabel("PR Title:"),
		f.fileOpsPRTitleEntry,
		widget.NewLabel("PR Body:"),
		f.fileOpsPRBodyEntry,
		widget.NewLabel("Branch Prefix:"),
		f.fileOpsBranchPrefixEntry,
		widget.NewSeparator(),
	)

	// PR Settings Card with dynamic content
	prSettingsCard := widget.NewCard("Pull Request Settings", "", container.New(layout.NewVBoxLayout(),
		f.fileOpsPRSettingsContainer,
		pushMethodContainer,
	))

	// Set initial visibility state (PR settings visible by default, Direct Push is off)
	f.fileOpsPRSettingsContainer.Show()

	// Create scrollable left column
	leftColumn := container.NewScroll(
		container.New(
			layout.NewVBoxLayout(),
			widget.NewCard("File Operation Rules", "",
				container.New(layout.NewVBoxLayout(),
					headerContainer,
					f.fileOperationsRulesScroll,
				),
			),
			instructions,
			prSettingsCard,
		),
	)
	leftColumn.SetMinSize(fyne.NewSize(600, 400))

	return leftColumn
}

// createFileOperationsRightColumn creates the right column with repository selection
func (f *FyneApp) createFileOperationsRightColumn() *fyne.Container {

	// Load repositories button
	loadReposBtn := widget.NewButtonWithIcon("Load Repositories", theme.DownloadIcon(), func() {
		f.handleFileOpsLoadRepositories()
	})
	loadReposBtn.Importance = widget.HighImportance

	// Filter entry
	f.fileOpsFilterEntry = widget.NewEntry()
	f.fileOpsFilterEntry.SetPlaceHolder("Filter repositories...")
	f.fileOpsFilterEntry.OnChanged = func(filter string) {
		f.filterFileOpsRepositories(filter)
	}

	// Selection buttons
	selectAllBtn := widget.NewButton("Select All", func() {
		f.selectAllFileOpsRepositories(true)
	})
	deselectAllBtn := widget.NewButton("Select None", func() {
		f.selectAllFileOpsRepositories(false)
	})

	// Repository list scroll
	repoScroll := container.NewVScroll(f.fileOpsRepoContainer)

	// Create prominent load button section
	loadSection := container.New(layout.NewHBoxLayout(),
		loadReposBtn,
		layout.NewSpacer(),
	)

	// Selection buttons in separate row
	selectionButtons := container.New(layout.NewHBoxLayout(),
		selectAllBtn,
		deselectAllBtn,
		layout.NewSpacer(),
	)

	// Create repository section (copy exact structure from String Replacement tab)
	repoHeaderLabel := widget.NewLabel("Repository Selection")
	repoHeaderLabel.TextStyle = fyne.TextStyle{Bold: true}
	repoSubLabel := widget.NewLabel("Load and select target repositories")

	repoHeader := container.New(layout.NewVBoxLayout(),
		repoHeaderLabel,
		repoSubLabel,
		widget.NewSeparator(),
	)

	// Combine buttons into one section like String Replacement tab
	repoButtons := container.New(layout.NewVBoxLayout(),
		loadSection,
		f.fileOpsFilterEntry,
		selectionButtons,
	)

	// Group fixed-height elements for the top section (exact copy from String Replacement)
	topSection := container.New(layout.NewVBoxLayout(),
		repoHeader,
		repoButtons,
		widget.NewSeparator(),
	)

	// Use BorderLayout so scroll area expands to fill available space
	rightColumn := container.New(layout.NewBorderLayout(topSection, nil, nil, nil),
		topSection, // Top border (fixed height)
		repoScroll, // Center (expands to fill)
	)

	return rightColumn
}

// addFileOperationRule adds a new file operation rule to the UI
func (f *FyneApp) addFileOperationRule() {
	rule := &FileOperationRuleWidget{
		SourcePathEntry:     widget.NewEntry(),
		TargetPathEntry:     widget.NewEntry(),
		SearchModeSelect:    widget.NewSelect([]string{"exact", "filename"}, nil),
		OperationTypeSelect: widget.NewSelect([]string{"move", "rename"}, nil),
	}

	rule.SourcePathEntry.SetPlaceHolder("Source path/pattern (e.g., 'config.yml')")
	rule.TargetPathEntry.SetPlaceHolder("Target path/pattern (e.g., 'config.yaml')")
	rule.SearchModeSelect.SetSelected("filename")
	rule.OperationTypeSelect.SetSelected("rename")

	// Remove button for this rule
	removeBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		f.fileOperationsRulesContainer.Remove(rule.Container)
	})
	removeBtn.Importance = widget.LowImportance

	// Create rule container
	rule.Container = container.New(layout.NewBorderLayout(nil, nil, nil, removeBtn),
		container.New(layout.NewVBoxLayout(),
			container.New(layout.NewGridLayout(2),
				widget.NewLabel("Source:"),
				rule.SourcePathEntry,
				widget.NewLabel("Target:"),
				rule.TargetPathEntry,
				widget.NewLabel("Search:"),
				rule.SearchModeSelect,
				widget.NewLabel("Operation:"),
				rule.OperationTypeSelect,
			),
			widget.NewSeparator(),
		),
		removeBtn,
	)

	f.fileOperationsRulesContainer.Add(rule.Container)
}

// handleFileOpsLoadRepositories loads repositories for file operations
func (f *FyneApp) handleFileOpsLoadRepositories() {
	f.setStatus("Loading repositories...")
	f.showLoading("Loading repositories...")

	go func() {
		repos, err := f.service.ListRepositories()
		f.hideLoading()

		if err != nil {
			f.setStatusError(fmt.Sprintf("Failed to load repositories: %v", err))
			return
		}

		f.repositories = repos
		f.fileOpsFilteredRepos = repos
		f.displayFileOpsRepositories()
		f.setStatusSuccess(fmt.Sprintf("Loaded %d repositories", len(repos)))
	}()
}

// displayFileOpsRepositories displays the repositories in the file operations tab
func (f *FyneApp) displayFileOpsRepositories() {
	f.fileOpsRepoContainer.RemoveAll()
	f.fileOpsRepoWidgets = nil

	for i := range f.fileOpsFilteredRepos {
		repo := &f.fileOpsFilteredRepos[i]

		toggle := NewToggleSwitch("", func(checked bool) {
			repo.Selected = checked
			f.updateFileOpsSelectionCount()
		})
		toggle.SetChecked(repo.Selected)

		label := widget.NewLabel(repo.Name)
		label.TextStyle = fyne.TextStyle{Bold: true}

		privateLabel := widget.NewLabel("")
		if repo.Private {
			privateLabel = widget.NewLabel("🔒")
		}

		repoContainer := container.New(layout.NewBorderLayout(nil, nil, toggle, privateLabel), toggle, label)

		f.fileOpsRepoWidgets = append(f.fileOpsRepoWidgets, repoContainer)
		f.fileOpsRepoContainer.Add(repoContainer)
	}

	f.updateFileOpsSelectionCount()
}

// filterFileOpsRepositories filters repositories based on search string
func (f *FyneApp) filterFileOpsRepositories(filter string) {
	if filter == "" {
		f.fileOpsFilteredRepos = f.repositories
	} else {
		f.fileOpsFilteredRepos = nil
		lowerFilter := strings.ToLower(filter)
		for _, repo := range f.repositories {
			if strings.Contains(strings.ToLower(repo.Name), lowerFilter) ||
				strings.Contains(strings.ToLower(repo.FullName), lowerFilter) {
				f.fileOpsFilteredRepos = append(f.fileOpsFilteredRepos, repo)
			}
		}
	}
	f.displayFileOpsRepositories()
}

// selectAllFileOpsRepositories selects or deselects all visible repositories
func (f *FyneApp) selectAllFileOpsRepositories(selected bool) {
	// Iterate through the actual UI widgets and call SetChecked to trigger animations and colors
	for _, obj := range f.fileOpsRepoContainer.Objects {
		if container, ok := obj.(*fyne.Container); ok {
			if toggle, ok := container.Objects[0].(*ToggleSwitch); ok {
				toggle.SetChecked(selected)
			}
		}
	}
}

// updateFileOpsSelectionCount updates the status with selection count
func (f *FyneApp) updateFileOpsSelectionCount() {
	count := 0
	for _, repo := range f.repositories {
		if repo.Selected {
			count++
		}
	}
	if count > 0 {
		f.setStatus(fmt.Sprintf("%d repositories selected for file operations", count))
	}
}

// updateFileOpsPushMethodStatus updates the push method status and PR settings visibility
func (f *FyneApp) updateFileOpsPushMethodStatus(directPush bool) {
	if directPush {
		// Hide PR settings when direct push is enabled
		f.fileOpsPRSettingsContainer.Hide()
		f.setStatus("⚠️ Direct push mode enabled - changes will be pushed to default branch")
	} else {
		// Show PR settings when PR mode is enabled
		f.fileOpsPRSettingsContainer.Show()
		f.setStatus("Pull request mode - changes will create PRs for review")
	}
}

// getFileOperationRules collects all file operation rules from the UI
func (f *FyneApp) getFileOperationRules() []FileOperationRule {
	var rules []FileOperationRule

	for _, obj := range f.fileOperationsRulesContainer.Objects {
		if container, ok := obj.(*fyne.Container); ok {
			// Extract the rule from the container
			if len(container.Objects) > 0 {
				if innerContainer, ok := container.Objects[0].(*fyne.Container); ok {
					if vbox, ok := innerContainer.Objects[0].(*fyne.Container); ok {
						// The grid layout contains our widgets
						gridObjects := vbox.Objects
						if len(gridObjects) >= 8 {
							// Extract entries and selects
							sourceEntry := gridObjects[1].(*widget.Entry)
							targetEntry := gridObjects[3].(*widget.Entry)
							searchSelect := gridObjects[5].(*widget.Select)
							operationSelect := gridObjects[7].(*widget.Select)

							if sourceEntry.Text != "" && targetEntry.Text != "" {
								rules = append(rules, FileOperationRule{
									SourcePath:    sourceEntry.Text,
									TargetPath:    targetEntry.Text,
									SearchMode:    searchSelect.Selected,
									OperationType: operationSelect.Selected,
								})
							}
						}
					}
				}
			}
		}
	}

	return rules
}

// handleFileOperationsPreview shows a preview of files that will be affected
func (f *FyneApp) handleFileOperationsPreview() {
	rules := f.getFileOperationRules()
	if len(rules) == 0 {
		f.setStatusError("Please add at least one file operation rule")
		return
	}

	selectedRepos := f.getSelectedFileOpsRepositories()
	if len(selectedRepos) == 0 {
		f.setStatusError("Please select at least one repository")
		return
	}

	f.setStatus("Previewing file operations...")
	// TODO: Implement preview logic using service.ProcessFileOperations with DryRun
	f.setStatus("Preview functionality not yet implemented")
}

// handleFileOperationsDryRun performs a dry run of file operations
func (f *FyneApp) handleFileOperationsDryRun() {
	rules := f.getFileOperationRules()
	if len(rules) == 0 {
		f.setStatusError("Please add at least one file operation rule")
		return
	}

	selectedRepos := f.getSelectedFileOpsRepositories()
	if len(selectedRepos) == 0 {
		f.setStatusError("Please select at least one repository")
		return
	}

	options := ProcessingOptions{
		DryRun:       true,
		DirectPush:   f.fileOpsPushDirectToggle.Checked,
		PRTitle:      f.fileOpsPRTitleEntry.Text,
		PRBody:       f.fileOpsPRBodyEntry.Text,
		BranchPrefix: f.fileOpsBranchPrefixEntry.Text,
	}

	f.showLoading("Running dry run for file operations...")

	go func() {
		result, err := f.service.ProcessFileOperations(rules, selectedRepos, options)
		f.hideLoading()

		if err != nil {
			f.setStatusError(fmt.Sprintf("Dry run failed: %v", err))
			return
		}

		f.showFileOperationsResults(result, true)
	}()
}

// handleFileOperationsProcess processes the actual file operations
func (f *FyneApp) handleFileOperationsProcess() {
	rules := f.getFileOperationRules()
	if len(rules) == 0 {
		f.setStatusError("Please add at least one file operation rule")
		return
	}

	selectedRepos := f.getSelectedFileOpsRepositories()
	if len(selectedRepos) == 0 {
		f.setStatusError("Please select at least one repository")
		return
	}

	// Simple confirmation - directly proceed
	if f.fileOpsPushDirectToggle.Checked {
		f.setStatus("⚠️ WARNING: Processing with direct push mode!")
	}

	options := ProcessingOptions{
		DryRun:       false,
		DirectPush:   f.fileOpsPushDirectToggle.Checked,
		PRTitle:      f.fileOpsPRTitleEntry.Text,
		PRBody:       f.fileOpsPRBodyEntry.Text,
		BranchPrefix: f.fileOpsBranchPrefixEntry.Text,
	}

	f.showLoading("Processing file operations...")

	go func() {
		result, err := f.service.ProcessFileOperations(rules, selectedRepos, options)
		f.hideLoading()

		if err != nil {
			f.setStatusError(fmt.Sprintf("Processing failed: %v", err))
			return
		}

		f.showFileOperationsResults(result, false)
	}()
}

// getSelectedFileOpsRepositories returns the selected repositories
func (f *FyneApp) getSelectedFileOpsRepositories() []Repository {
	var selected []Repository
	for _, repo := range f.repositories {
		if repo.Selected {
			selected = append(selected, repo)
		}
	}
	return selected
}

// showFileOperationsResults displays the results of file operations
func (f *FyneApp) showFileOperationsResults(result *FileOperationResult, isDryRun bool) {
	title := "File Operations Results"
	if isDryRun {
		title = "File Operations Dry Run Results"
	}

	// Create results window
	resultsWindow := f.app.NewWindow(title)
	resultsWindow.Resize(fyne.NewSize(800, 600))

	// Results content
	content := container.New(layout.NewVBoxLayout())

	// Summary
	summaryText := fmt.Sprintf("Overall: %s\n%s",
		map[bool]string{true: "✅ Success", false: "❌ Failed"}[result.Success],
		result.Message)
	summaryLabel := widget.NewLabelWithStyle(summaryText, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	content.Add(summaryLabel)
	content.Add(widget.NewSeparator())

	// Repository results
	for _, repoResult := range result.RepositoryResults {
		statusIcon := "✅"
		if !repoResult.Success {
			statusIcon = "❌"
		}

		resultText := fmt.Sprintf("%s %s: %s\nFiles changed: %d",
			statusIcon, repoResult.Repository, repoResult.Message,
			len(repoResult.FilesChanged))

		resultLabel := widget.NewLabel(resultText)
		resultLabel.Wrapping = fyne.TextWrapWord

		var resultCard *widget.Card
		if repoResult.PRUrl != "" {
			// Add PR link button
			prBtn := widget.NewButtonWithIcon("View Pull Request", theme.ComputerIcon(), func() {
				f.openURL(repoResult.PRUrl)
			})
			prBtn.Importance = widget.HighImportance

			cardContent := container.New(layout.NewVBoxLayout(), resultLabel, prBtn)
			resultCard = widget.NewCard("", "", cardContent)
		} else if repoResult.CommitURL != "" {
			// Add commit link button for direct push
			commitBtn := widget.NewButtonWithIcon("View Commit", theme.ComputerIcon(), func() {
				f.openURL(repoResult.CommitURL)
			})
			commitBtn.Importance = widget.HighImportance

			cardContent := container.New(layout.NewVBoxLayout(), resultLabel, commitBtn)
			resultCard = widget.NewCard("", "", cardContent)
		} else {
			resultCard = widget.NewCard("", "", resultLabel)
		}

		content.Add(resultCard)
	}

	// File matches for dry run
	if isDryRun && len(result.FileMatches) > 0 {
		content.Add(widget.NewSeparator())
		content.Add(widget.NewLabelWithStyle("Matched Files:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))

		for repo, files := range result.FileMatches {
			filesText := fmt.Sprintf("%s:\n  %s", repo, strings.Join(files, "\n  "))
			filesLabel := widget.NewLabel(filesText)
			filesLabel.Wrapping = fyne.TextWrapWord
			content.Add(widget.NewCard("", "", filesLabel))
		}
	}

	// Scroll container
	scroll := container.NewScroll(content)

	// Close button
	closeBtn := widget.NewButton("Close", func() {
		resultsWindow.Close()
	})

	// Window content
	windowContent := container.New(layout.NewBorderLayout(nil, closeBtn, nil, nil),
		scroll, closeBtn)

	resultsWindow.SetContent(windowContent)
	resultsWindow.Show()
}
