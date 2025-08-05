package fynegui

import (
	"fmt"
	"image/color"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	xtheme "fyne.io/x/fyne/theme"

	"go-toolgit/internal/core/config"
	"go-toolgit/internal/core/utils"
)

// AdwaitaVariantTheme wraps the Adwaita theme to force a specific variant (light/dark)
type AdwaitaVariantTheme struct {
	baseTheme fyne.Theme
	variant   fyne.ThemeVariant
}

// Color forces the specific variant instead of using the system default
func (a *AdwaitaVariantTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return a.baseTheme.Color(name, a.variant)
}

// Font delegates to the base Adwaita theme
func (a *AdwaitaVariantTheme) Font(style fyne.TextStyle) fyne.Resource {
	return a.baseTheme.Font(style)
}

// Icon delegates to the base Adwaita theme
func (a *AdwaitaVariantTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return a.baseTheme.Icon(name)
}

// Size delegates to the base Adwaita theme
func (a *AdwaitaVariantTheme) Size(name fyne.ThemeSizeName) float32 {
	return a.baseTheme.Size(name)
}

type FyneApp struct {
	app         fyne.App
	window      fyne.Window
	service     *Service
	logger      *utils.Logger
	modernTheme *ModernTheme
	currentThemeType string // "Modern" or "Adwaita"
	isDarkMode  bool

	// Current tab
	currentTab *container.AppTabs

	// Config widgets
	providerSelect *widget.Select
	
	// GitHub-specific widget instances
	githubURLEntry    *widget.Entry
	githubTokenEntry  *widget.Entry
	githubOrgEntry    *widget.Entry
	githubTeamEntry   *widget.Entry
	
	// Bitbucket-specific widget instances  
	bitbucketURLEntry      *widget.Entry
	bitbucketUsernameEntry *widget.Entry
	bitbucketPasswordEntry *widget.Entry
	bitbucketProjectEntry  *widget.Entry
	
	// Dynamic form management
	configForm        *widget.Form
	providerContainer *fyne.Container
	helpText          *widget.RichText

	// String replacement widgets
	replacementRulesContainer *fyne.Container
	replacementRulesScroll    *container.Scroll
	repoSelectionContainer    *fyne.Container
	includePatternEditor      *PatternEditor
	excludePatternEditor      *PatternEditor
	prTitleEntry              *widget.Entry
	prBodyEntry               *widget.Entry
	branchPrefixEntry         *widget.Entry

	// Repository filtering
	repoFilterEntry           *widget.Entry
	filteredRepositories      []Repository
	repoWidgets              []*fyne.Container

	// Migration widgets
	sourceURLEntry    *widget.Entry
	targetOrgEntry    *widget.Entry
	targetRepoEntry   *widget.Entry
	webhookURLEntry   *widget.Entry
	teamsContainer    *fyne.Container
	progressContainer *fyne.Container

	// Status
	statusLabel *widget.Label
	statusIcon  *widget.Icon
	rateLimitStatus *RateLimitStatus
	operationStatus *OperationStatus

	// Repository data storage
	repositories []Repository

	// Loading indicators
	loadingOverlay *LoadingContainer
	mainContent    *fyne.Container
}

func NewFyneApp() *FyneApp {
	cfg, _ := config.Load()
	
	// Use configured log level, default to info if config loading failed
	logLevel := "info"
	logFormat := "text"
	if cfg != nil {
		logLevel = cfg.Logging.Level
		logFormat = cfg.Logging.Format
	}
	
	logger := utils.NewLogger(logLevel, logFormat)

	fyneApp := app.NewWithID("com.github.go-toolgit")

	// Initialize with Adwaita theme and dark mode
	modernTheme := NewModernTheme(true) // Keep for fallback
	adwaitaTheme := &AdwaitaVariantTheme{
		baseTheme: xtheme.AdwaitaTheme(),
		variant:   theme.VariantDark,
	}
	fyneApp.Settings().SetTheme(adwaitaTheme)

	window := fyneApp.NewWindow("GitHub & Bitbucket DevOps Tool")
	window.Resize(fyne.NewSize(1200, 1000))
	window.CenterOnScreen()

	service := NewService(cfg, logger)

	return &FyneApp{
		app:              fyneApp,
		window:           window,
		service:          service,
		logger:           logger,
		modernTheme:      modernTheme.(*ModernTheme),
		currentThemeType: "Adwaita",
		isDarkMode:       true,
	}
}

func (f *FyneApp) Run() {
	f.setupUI()
	f.loadConfigurationFromFile() // Load config and prefill GUI
	f.startRateLimitRefreshTimer()
	f.window.ShowAndRun()
}

// getCurrentTheme returns the appropriate theme based on current settings
func (f *FyneApp) getCurrentTheme() fyne.Theme {
	switch f.currentThemeType {
	case "Adwaita":
		variant := theme.VariantLight
		if f.isDarkMode {
			variant = theme.VariantDark
		}
		return &AdwaitaVariantTheme{
			baseTheme: xtheme.AdwaitaTheme(),
			variant:   variant,
		}
	default: // "Modern"
		f.modernTheme.isDark = f.isDarkMode
		return f.modernTheme
	}
}

// applyTheme applies the current theme to the app
func (f *FyneApp) applyTheme() {
	currentTheme := f.getCurrentTheme()
	f.app.Settings().SetTheme(currentTheme)
	
	// Force refresh of the entire UI
	f.window.Content().Refresh()
	
	// Show feedback to user
	themeMode := "light"
	if f.isDarkMode {
		themeMode = "dark"
	}
	ShowToast(f.window, fmt.Sprintf("Switched to %s %s theme", f.currentThemeType, themeMode), "info")
}

func (f *FyneApp) setupUI() {
	// Create theme selector dropdown
	themeSelector := widget.NewSelect([]string{"Modern", "Adwaita"}, func(selected string) {
		f.currentThemeType = selected
		f.applyTheme()
	})
	themeSelector.SetSelected("Adwaita")

	// Create theme toggle for dark/light mode
	themeToggle := NewToggleSwitch("Dark Mode", func(dark bool) {
		f.isDarkMode = dark
		f.applyTheme()
	})
	themeToggle.SetChecked(true) // Start with dark mode

	themeContainer := container.NewHBox(
		layout.NewSpacer(),
		widget.NewLabel("Theme:"),
		themeSelector,
		widget.NewLabel("Mode:"),
		themeToggle,
	)

	// Create main tabs with icons
	f.currentTab = container.NewAppTabs(
		container.NewTabItemWithIcon("Configuration", theme.SettingsIcon(), f.createConfigTab()),
		container.NewTabItemWithIcon("String Replacement", theme.DocumentCreateIcon(), f.createReplacementTab()),
		container.NewTabItemWithIcon("Repository Migration", theme.UploadIcon(), f.createMigrationTab()),
	)

	// Enhanced status bar with icon and rate limit status
	f.statusLabel = widget.NewLabel("Ready")
	f.statusLabel.TextStyle = fyne.TextStyle{Bold: true}
	f.statusLabel.Alignment = fyne.TextAlignCenter
	f.statusIcon = widget.NewIcon(theme.InfoIcon())
	
	// Create rate limit status widget with refresh callback
	f.rateLimitStatus = NewRateLimitStatus(f.refreshRateLimit)
	
	// Create operation status widget
	f.operationStatus = NewOperationStatus()

	// Create a more compact layout with main status on left, API info on right
	leftStatus := container.NewHBox(f.statusIcon, container.NewCenter(f.statusLabel))
	rightStatus := container.NewHBox(f.operationStatus, widget.NewSeparator(), f.rateLimitStatus)
	
	statusContent := container.New(
		layout.NewBorderLayout(nil, nil, leftStatus, rightStatus),
		leftStatus,
		rightStatus,
	)

	statusBar := widget.NewCard("", "", statusContent)

	// Main layout with padding and theme toggle
	f.mainContent = container.NewPadded(
		container.New(
			layout.NewBorderLayout(themeContainer, statusBar, nil, nil),
			themeContainer,
			f.currentTab,
			statusBar,
		),
	)

	// Create loading overlay with enhanced spinner
	f.loadingOverlay = NewLoadingContainer("Loading...")
	f.loadingOverlay.Hide()

	// Stack the main content and loading overlay
	content := container.NewStack(
		f.mainContent,
		container.NewCenter(f.loadingOverlay),
	)

	f.window.SetContent(content)
}

func (f *FyneApp) createConfigTab() *fyne.Container {
	// Provider selection with callback
	f.providerSelect = widget.NewSelect([]string{"github", "bitbucket"}, func(selected string) {
		f.logger.Info("Provider selected", "provider", selected)
		// Clear values when user manually switches providers to prevent cross-contamination
		f.updateProviderFieldsWithClear(selected, true)
	})
	f.providerSelect.Selected = "github"

	// Initialize all field entries
	f.initializeConfigFields()

	// Create dynamic form
	f.configForm = &widget.Form{}
	f.providerContainer = container.New(layout.NewVBoxLayout())

	// Buttons
	validateBtn := widget.NewButtonWithIcon("Validate Configuration", theme.ConfirmIcon(), f.handleValidateConfig)
	validateBtn.Importance = widget.HighImportance

	saveBtn := widget.NewButtonWithIcon("Save Configuration", theme.DocumentSaveIcon(), f.handleSaveConfig)
	saveBtn.Importance = widget.MediumImportance

	buttonsContainer := container.New(
		layout.NewHBoxLayout(),
		saveBtn,
		validateBtn,
	)

	// Initialize with GitHub fields (default)
	f.updateProviderFields("github")

	configCard := widget.NewCard("Configuration", "Setup your Git provider access", f.providerContainer)

	// Create dynamic help text
	f.helpText = widget.NewRichTextFromMarkdown("")
	f.updateHelpText("github")
	helpCard := widget.NewCard("", "Getting Started", f.helpText)

	return container.New(
		layout.NewVBoxLayout(),
		configCard,
		helpCard,
		container.NewPadded(buttonsContainer),
	)
}

// Initialize all configuration field entries with separate instances for each provider
func (f *FyneApp) initializeConfigFields() {
	// GitHub widget instances - all separate
	f.githubURLEntry = widget.NewEntry()
	f.githubURLEntry.SetPlaceHolder("https://api.github.com")

	f.githubTokenEntry = widget.NewPasswordEntry()
	f.githubTokenEntry.SetPlaceHolder("ghp_your_github_token")

	f.githubOrgEntry = widget.NewEntry()
	f.githubOrgEntry.SetPlaceHolder("your-organization")

	f.githubTeamEntry = widget.NewEntry()
	f.githubTeamEntry.SetPlaceHolder("your-team")

	// Bitbucket widget instances - all separate
	f.bitbucketURLEntry = widget.NewEntry()
	f.bitbucketURLEntry.SetPlaceHolder("https://api.bitbucket.org")

	// Create username field as regular Entry (plain text)
	f.bitbucketUsernameEntry = widget.NewEntry()
	f.bitbucketUsernameEntry.SetPlaceHolder("your-username")
	// Explicitly ensure this is NOT a password field
	f.bitbucketUsernameEntry.Password = false
	f.logger.Info("Created Bitbucket username widget", "type", fmt.Sprintf("%T", f.bitbucketUsernameEntry), "widget_ptr", fmt.Sprintf("%p", f.bitbucketUsernameEntry), "password_property", f.bitbucketUsernameEntry.Password)

	// Create password field as PasswordEntry (obfuscated)
	f.bitbucketPasswordEntry = widget.NewPasswordEntry()
	f.bitbucketPasswordEntry.SetPlaceHolder("your-app-password")
	// Explicitly ensure password property is set
	f.bitbucketPasswordEntry.Password = true
	f.logger.Info("Created Bitbucket password widget", "type", fmt.Sprintf("%T", f.bitbucketPasswordEntry), "widget_ptr", fmt.Sprintf("%p", f.bitbucketPasswordEntry), "password_property", f.bitbucketPasswordEntry.Password)

	f.bitbucketProjectEntry = widget.NewEntry()
	f.bitbucketProjectEntry.SetPlaceHolder("PROJECT_KEY")
}

// clearAllWidgetValues clears all configuration widget values to prevent cross-contamination
func (f *FyneApp) clearAllWidgetValues() {
	// Clear GitHub widget instances
	if f.githubURLEntry != nil {
		f.githubURLEntry.SetText("")
	}
	if f.githubTokenEntry != nil {
		f.githubTokenEntry.SetText("")
	}
	if f.githubOrgEntry != nil {
		f.githubOrgEntry.SetText("")
	}
	if f.githubTeamEntry != nil {
		f.githubTeamEntry.SetText("")
	}
	
	// Clear Bitbucket widget instances
	if f.bitbucketURLEntry != nil {
		f.bitbucketURLEntry.SetText("")
	}
	if f.bitbucketUsernameEntry != nil {
		f.bitbucketUsernameEntry.SetText("")
	}
	if f.bitbucketPasswordEntry != nil {
		f.bitbucketPasswordEntry.SetText("")
	}
	if f.bitbucketProjectEntry != nil {
		f.bitbucketProjectEntry.SetText("")
	}
}

// Update form fields based on selected provider
func (f *FyneApp) updateProviderFields(provider string) {
	f.updateProviderFieldsWithClear(provider, false)
}

// updateProviderFieldsWithClear updates form fields and optionally clears values
func (f *FyneApp) updateProviderFieldsWithClear(provider string, clearValues bool) {
	f.logger.Info("Updating provider fields", "provider", provider, "clear_values", clearValues)
	
	// Clear existing form items
	f.configForm.Items = []*widget.FormItem{}

	// Clear widget values if requested (to prevent cross-contamination)
	if clearValues {
		f.clearAllWidgetValues()
	}

	// Always add provider selection first
	f.configForm.Items = append(f.configForm.Items, &widget.FormItem{
		Text:     "Provider",
		Widget:   f.providerSelect,
		HintText: "Choose your Git provider",
	})

	if provider == "github" {
		f.addGitHubFields()
	} else if provider == "bitbucket" {
		f.addBitbucketFields()
	}

	// Update the provider container
	f.providerContainer.Objects = []fyne.CanvasObject{f.configForm}
	f.providerContainer.Refresh()
	f.logger.Info("Provider container refreshed", "provider", provider)

	// Update help text
	f.updateHelpText(provider)
}

// Add GitHub-specific form fields using dedicated GitHub widget instances
func (f *FyneApp) addGitHubFields() {
	f.configForm.Items = append(f.configForm.Items, []*widget.FormItem{
		{
			Text:     "GitHub URL",
			Widget:   f.githubURLEntry,
			HintText: "API endpoint (e.g., https://api.github.com or https://github.company.com/api/v3)",
		},
		{
			Text:     "Personal Access Token",
			Widget:   f.githubTokenEntry,
			HintText: "GitHub PAT with repo and org:read permissions",
		},
		{
			Text:     "Organization",
			Widget:   f.githubOrgEntry,
			HintText: "Your GitHub organization name",
		},
		{
			Text:     "Team",
			Widget:   f.githubTeamEntry,
			HintText: "Your team slug within the organization",
		},
	}...)
}

// Add Bitbucket-specific form fields using dedicated Bitbucket widget instances
func (f *FyneApp) addBitbucketFields() {
	f.logger.Info("Adding Bitbucket form fields", 
		"username_widget_type", fmt.Sprintf("%T", f.bitbucketUsernameEntry),
		"username_widget_ptr", fmt.Sprintf("%p", f.bitbucketUsernameEntry),
		"username_password_prop", f.bitbucketUsernameEntry.Password,
		"password_widget_type", fmt.Sprintf("%T", f.bitbucketPasswordEntry),
		"password_widget_ptr", fmt.Sprintf("%p", f.bitbucketPasswordEntry),
		"password_password_prop", f.bitbucketPasswordEntry.Password)
	
	f.configForm.Items = append(f.configForm.Items, []*widget.FormItem{
		{
			Text:     "Bitbucket URL",
			Widget:   f.bitbucketURLEntry,
			HintText: "API endpoint (e.g., https://api.bitbucket.org or https://bitbucket.company.com)",
		},
		{
			Text:     "Username",
			Widget:   f.bitbucketUsernameEntry,
			HintText: "Your Bitbucket username",
		},
		{
			Text:     "App Password",
			Widget:   f.bitbucketPasswordEntry,
			HintText: "Bitbucket app password with repository permissions",
		},
		{
			Text:     "Project Key",
			Widget:   f.bitbucketProjectEntry,
			HintText: "Your Bitbucket project key (e.g., MYPROJ)",
		},
	}...)
}

// Update help text based on provider
func (f *FyneApp) updateHelpText(provider string) {
	if f.helpText == nil {
		return
	}

	var helpContent string
	if provider == "github" {
		helpContent = `
### GitHub Setup Guide
1. **Generate Personal Access Token**:
   - Go to GitHub Settings → Developer settings → Personal access tokens
   - Create token with 'repo' and 'read:org' scopes
2. **Find Organization & Team**:
   - Use your GitHub organization name
   - Use team slug (lowercase, hyphenated)
3. **Enterprise GitHub**: Use your company's API URL
4. Click "Validate Configuration" to test connection
`
	} else {
		helpContent = `
### Bitbucket Setup Guide
1. **Create App Password**:
   - Go to Bitbucket Settings → App passwords
   - Create password with 'Repositories: Read/Write' permissions
2. **Find Project Key**:
   - Visit your project page
   - Project key is shown in uppercase (e.g., MYPROJ)
3. **Bitbucket Server**: Use your company's Bitbucket URL
4. Click "Validate Configuration" to test connection
`
	}

	f.helpText.ParseMarkdown(helpContent)
	f.helpText.Refresh()
}

func (f *FyneApp) createReplacementTab() *fyne.Container {
	// Replacement rules container with scroll
	f.replacementRulesContainer = container.New(layout.NewVBoxLayout())
	f.replacementRulesScroll = container.NewScroll(f.replacementRulesContainer)
	f.replacementRulesScroll.SetMinSize(fyne.NewSize(0, 200))

	addRuleBtn := widget.NewButtonWithIcon("Add Replacement Rule", theme.ContentAddIcon(), f.handleAddReplacementRule)
	addRuleBtn.Importance = widget.HighImportance

	// File patterns with tag chips
	defaultIncludePatterns := []string{"*.go", "*.java", "*.js", "*.py", "*.ts", "*.jsx", "*.tsx", "*.md", "*.txt", "*.yaml", "*.yml"}
	f.includePatternEditor = NewPatternEditor("Add include pattern (e.g., *.go)", defaultIncludePatterns, nil)

	defaultExcludePatterns := []string{"vendor/*", "node_modules/*", "*.min.js", ".git/*"}
	f.excludePatternEditor = NewPatternEditor("Add exclude pattern (e.g., vendor/*)", defaultExcludePatterns, nil)

	// PR settings
	f.prTitleEntry = widget.NewEntry()
	f.prTitleEntry.SetPlaceHolder("chore: automated string replacement")
	f.prTitleEntry.SetText("chore: automated string replacement")

	f.prBodyEntry = widget.NewEntry()
	f.prBodyEntry.SetPlaceHolder("Automated replacement performed by go-toolgit tool.")
	f.prBodyEntry.SetText("Automated replacement performed by go-toolgit tool.")

	f.branchPrefixEntry = widget.NewEntry()
	f.branchPrefixEntry.SetPlaceHolder("auto-replace")
	f.branchPrefixEntry.SetText("auto-replace")

	// Repository selection with scroll
	f.repoSelectionContainer = container.New(layout.NewVBoxLayout())
	repoScroll := container.NewScroll(f.repoSelectionContainer)

	loadReposBtn := widget.NewButtonWithIcon("🔄 Load Repositories", theme.DownloadIcon(), f.handleLoadRepositories)
	loadReposBtn.Importance = widget.HighImportance

	// Repository filter entry
	f.repoFilterEntry = widget.NewEntry()
	f.repoFilterEntry.SetPlaceHolder("🔍 Filter repositories...")
	f.repoFilterEntry.OnChanged = f.handleRepositoryFilter

	// Clear filter button
	clearFilterBtn := widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
		f.repoFilterEntry.SetText("")
	})
	clearFilterBtn.Importance = widget.MediumImportance

	// Filter container
	filterContainer := container.New(layout.NewBorderLayout(nil, nil, nil, clearFilterBtn),
		f.repoFilterEntry,
		clearFilterBtn,
	)

	// Select/Deselect all buttons
	selectAllBtn := widget.NewButton("Select All", f.handleSelectAllRepos)
	deselectAllBtn := widget.NewButton("Deselect All", f.handleDeselectAllRepos)
	
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
	
	repoButtons := container.New(layout.NewVBoxLayout(),
		loadSection,
		selectionButtons,
		widget.NewSeparator(),
		filterContainer,
	)

	// Processing buttons
	validateReplacementBtn := widget.NewButtonWithIcon("Validate", theme.ConfirmIcon(), f.handleValidateReplacement)
	validateReplacementBtn.Importance = widget.MediumImportance

	dryRunReplacementBtn := widget.NewButtonWithIcon("Dry Run", theme.VisibilityIcon(), f.handleReplacementDryRun)
	dryRunReplacementBtn.Importance = widget.WarningImportance

	processBtn := widget.NewButtonWithIcon("Process Replacements", theme.MediaPlayIcon(), f.handleProcessReplacements)
	processBtn.Importance = widget.HighImportance

	// Forms with better styling
	rulesForm := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Include Patterns", Widget: f.includePatternEditor, HintText: "File patterns to include"},
			{Text: "Exclude Patterns", Widget: f.excludePatternEditor, HintText: "File patterns to exclude"},
			{Text: "PR Title", Widget: f.prTitleEntry, HintText: "Title for the pull request"},
			{Text: "PR Body", Widget: f.prBodyEntry, HintText: "Body text for the pull request"},
			{Text: "Branch Prefix", Widget: f.branchPrefixEntry, HintText: "Prefix for created branch names"},
		},
	}

	rulesCard := widget.NewCard("Replacement Rules", "Define string replacement patterns",
		container.New(layout.NewVBoxLayout(),
			f.replacementRulesScroll,
			container.NewPadded(addRuleBtn),
		))

	settingsCard := widget.NewCard("Processing Settings", "File patterns and PR configuration", rulesForm)

	// Create prominent repository section with better visual hierarchy
	repoHeaderLabel := widget.NewLabel("Repository Selection")
	repoHeaderLabel.TextStyle = fyne.TextStyle{Bold: true}
	repoSubLabel := widget.NewLabel("Load and select target repositories")
	
	repoHeader := container.New(layout.NewVBoxLayout(),
		repoHeaderLabel,
		repoSubLabel,
		widget.NewSeparator(),
	)
	
	// Group fixed-height elements for the top section
	topSection := container.New(layout.NewVBoxLayout(),
		repoHeader,
		repoButtons,
		widget.NewSeparator(),
	)
	
	// Use BorderLayout so scroll area expands to fill available space
	repoContainer := container.New(layout.NewBorderLayout(topSection, nil, nil, nil),
		topSection,  // Top border (fixed height)
		repoScroll,  // Center (expands to fill)
	)

	buttonsContainer := container.NewPadded(
		container.New(
			layout.NewHBoxLayout(),
			validateReplacementBtn,
			dryRunReplacementBtn,
			layout.NewSpacer(),
			processBtn,
		),
	)

	// Create two-column layout: scrollable left side, fixed right side
	leftColumn := container.NewScroll(
		container.New(
			layout.NewVBoxLayout(),
			rulesCard,
			settingsCard,
		),
	)
	leftColumn.SetMinSize(fyne.NewSize(600, 400))

	rightColumn := repoContainer
	rightColumn.Resize(fyne.NewSize(500, 400))

	// Create resizable horizontal split layout
	mainContent := container.NewHSplit(leftColumn, rightColumn)
	mainContent.SetOffset(0.55) // Start with left column slightly larger

	return container.New(
		layout.NewBorderLayout(nil, buttonsContainer, nil, nil),
		mainContent,
		buttonsContainer,
	)
}

func (f *FyneApp) createMigrationTab() *fyne.Container {
	// Migration form
	f.sourceURLEntry = widget.NewEntry()
	f.sourceURLEntry.SetPlaceHolder("ssh://git@bitbucket.company.com:2222/PROJ/repo.git")

	f.targetOrgEntry = widget.NewEntry()
	f.targetOrgEntry.SetPlaceHolder("target-github-org")

	f.targetRepoEntry = widget.NewEntry()
	f.targetRepoEntry.SetPlaceHolder("target-repo-name")

	f.webhookURLEntry = widget.NewEntry()
	f.webhookURLEntry.SetPlaceHolder("https://ci.company.com/webhook")

	// Teams management
	f.teamsContainer = container.New(layout.NewVBoxLayout())
	addTeamBtn := widget.NewButton("Add Team", f.handleAddTeam)

	// Migration buttons
	saveMigrationBtn := widget.NewButtonWithIcon("Save Configuration", theme.DocumentSaveIcon(), f.handleSaveMigrationConfig)
	saveMigrationBtn.Importance = widget.MediumImportance

	validateMigrationBtn := widget.NewButton("Validate Migration", f.handleValidateMigration)
	validateMigrationBtn.Importance = widget.MediumImportance

	dryRunBtn := widget.NewButton("Dry Run", f.handleMigrationDryRun)
	dryRunBtn.Importance = widget.WarningImportance

	migrateBtn := widget.NewButton("Start Migration", f.handleStartMigration)
	migrateBtn.Importance = widget.HighImportance

	// Progress area
	f.progressContainer = container.New(layout.NewVBoxLayout())

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Source Bitbucket URL", Widget: f.sourceURLEntry},
			{Text: "Target GitHub Organization", Widget: f.targetOrgEntry},
			{Text: "Target Repository Name", Widget: f.targetRepoEntry},
			{Text: "Webhook URL (optional)", Widget: f.webhookURLEntry},
		},
	}

	teamsCard := widget.NewCard("Team Permissions", "Assign GitHub teams to the repository",
		container.New(layout.NewVBoxLayout(), f.teamsContainer, addTeamBtn))

	buttonsContainer := container.New(
		layout.NewHBoxLayout(),
		saveMigrationBtn,
		validateMigrationBtn,
		dryRunBtn,
		migrateBtn,
	)

	progressCard := widget.NewCard("Migration Progress", "Real-time migration status", f.progressContainer)

	return container.New(
		layout.NewVBoxLayout(),
		widget.NewCard("Repository Migration", "Migrate from Bitbucket Server to GitHub", form),
		teamsCard,
		buttonsContainer,
		progressCard,
	)
}

func (f *FyneApp) handleValidateConfig() {
	f.setStatus("Validating configuration...")
	f.operationStatus.SetOperation(OperationAPIValidation, "Testing GitHub API connection")
	f.showLoading("Validating configuration...")

	configData := ConfigData{
		Provider:        f.providerSelect.Selected,
		GitHubURL:       f.githubURLEntry.Text,
		Token:           f.githubTokenEntry.Text,
		Organization:    f.githubOrgEntry.Text,
		Team:            f.githubTeamEntry.Text,
		IncludePatterns: f.includePatternEditor.GetPatterns(),
		ExcludePatterns: f.excludePatternEditor.GetPatterns(),
		PRTitleTemplate: f.prTitleEntry.Text,
		PRBodyTemplate:  f.prBodyEntry.Text,
		BranchPrefix:    f.branchPrefixEntry.Text,
	}

	go func() {
		err := f.service.UpdateConfig(configData)
		if err != nil {
			f.hideLoading()
			f.setStatusError(fmt.Sprintf("Configuration error: %v", err))
			return
		}

		err = f.service.ValidateAccess()
		f.hideLoading()

		if err != nil {
			f.setStatusError(fmt.Sprintf("Validation failed: %v", err))
			f.operationStatus.SetOperation(OperationIdle, "")
			return
		}

		f.setStatusSuccess("Configuration validated successfully!")
		f.operationStatus.SetOperation(OperationIdle, "")
		
		// Increment API call counter and refresh rate limit after GitHub API call
		f.operationStatus.IncrementAPICall()
		f.refreshRateLimit()
	}()
}

func (f *FyneApp) handleSaveConfig() {
	f.setStatus("Saving configuration...")
	f.showLoading("Saving configuration...")

	configData := ConfigData{
		Provider:        f.providerSelect.Selected,
		IncludePatterns: f.includePatternEditor.GetPatterns(),
		ExcludePatterns: f.excludePatternEditor.GetPatterns(),
		PRTitleTemplate: f.prTitleEntry.Text,
		PRBodyTemplate:  f.prBodyEntry.Text,
		BranchPrefix:    f.branchPrefixEntry.Text,
	}

	f.logger.Info("GUI collecting config data", "provider", f.providerSelect.Selected)

	// Collect provider-specific fields from dedicated widget instances
	if f.providerSelect.Selected == "github" {
		configData.GitHubURL = f.githubURLEntry.Text
		configData.Token = f.githubTokenEntry.Text
		configData.Organization = f.githubOrgEntry.Text
		configData.Team = f.githubTeamEntry.Text
		
		f.logger.Debug("Collected GitHub fields from dedicated widgets", 
			"base_url", configData.GitHubURL,
			"org", configData.Organization,
			"team", configData.Team,
			"token_length", len(configData.Token))
		
		// Clear Bitbucket fields
		configData.BitbucketURL = ""
		configData.Username = ""
		configData.Password = ""
		configData.Project = ""
	} else if f.providerSelect.Selected == "bitbucket" {
		configData.BitbucketURL = f.bitbucketURLEntry.Text
		configData.Username = f.bitbucketUsernameEntry.Text
		configData.Password = f.bitbucketPasswordEntry.Text
		configData.Project = f.bitbucketProjectEntry.Text
		
		f.logger.Info("Collected Bitbucket fields from dedicated widgets", 
			"base_url", configData.BitbucketURL,
			"username", configData.Username,
			"project", configData.Project,
			"password_length", len(configData.Password))
		
		// Clear GitHub fields
		configData.GitHubURL = ""
		configData.Token = ""
		configData.Organization = ""
		configData.Team = ""
	}

	go func() {
		err := f.service.UpdateConfig(configData)
		f.hideLoading()

		if err != nil {
			f.setStatusError(fmt.Sprintf("Failed to save configuration: %v", err))
			return
		}

		f.setStatusSuccess("Configuration saved successfully!")
	}()
}

func (f *FyneApp) handleSaveMigrationConfig() {
	f.setStatus("Saving migration configuration...")
	f.showLoading("Saving migration configuration...")
	
	// Collect current base configuration data from dedicated widget instances
	configData := ConfigData{
		Provider:        f.providerSelect.Selected,
		GitHubURL:       f.githubURLEntry.Text,
		Token:           f.githubTokenEntry.Text,
		Organization:    f.githubOrgEntry.Text,
		Team:            f.githubTeamEntry.Text,
		IncludePatterns: f.includePatternEditor.GetPatterns(),
		ExcludePatterns: f.excludePatternEditor.GetPatterns(),
		PRTitleTemplate: f.prTitleEntry.Text,
		PRBodyTemplate:  f.prBodyEntry.Text,
		BranchPrefix:    f.branchPrefixEntry.Text,
	}
	
	// Add migration data
	migrationConfig := f.collectMigrationConfig()
	configData.MigrationSourceURL = migrationConfig.SourceBitbucketURL
	configData.MigrationTargetOrg = migrationConfig.TargetGitHubOrg
	configData.MigrationTargetRepo = migrationConfig.TargetRepositoryName
	configData.MigrationWebhookURL = migrationConfig.WebhookURL
	configData.MigrationTeams = migrationConfig.Teams
	
	// Debug: Log what we're about to save
	f.logger.Info("Saving migration config", 
		"source_url", configData.MigrationSourceURL,
		"target_org", configData.MigrationTargetOrg,
		"target_repo", configData.MigrationTargetRepo,
		"webhook_url", configData.MigrationWebhookURL,
		"teams", configData.MigrationTeams)
	
	go func() {
		err := f.service.SaveConfig(configData)
		f.hideLoading()
		if err != nil {
			f.setStatusError(fmt.Sprintf("Failed to save migration configuration: %v", err))
			return
		}
		f.setStatusSuccess("Migration configuration saved successfully!")
	}()
}

func (f *FyneApp) handleAddTeam() {
	teamNameEntry := widget.NewEntry()
	teamNameEntry.SetPlaceHolder("team-name")

	permissionSelect := widget.NewSelect([]string{"pull", "push", "maintain", "admin"}, nil)
	permissionSelect.Selected = "pull"

	removeBtn := widget.NewButton("Remove", func() {
		// This will be set when the container is created
	})
	removeBtn.Importance = widget.DangerImportance

	// Create a container with more control over sizing
	rightControls := container.New(
		layout.NewHBoxLayout(),
		permissionSelect,
		removeBtn,
	)
	
	// Use BorderLayout to give the team entry more space
	teamContainer := container.New(
		layout.NewBorderLayout(nil, nil, nil, rightControls),
		rightControls,
		teamNameEntry, // This will take up remaining space
	)

	// Set the remove function to remove this specific container
	removeBtn.OnTapped = func() {
		f.teamsContainer.Remove(teamContainer)
	}

	f.teamsContainer.Add(teamContainer)
}

func (f *FyneApp) addTeamFromConfig(teamName, permission string) {
	teamNameEntry := widget.NewEntry()
	teamNameEntry.SetPlaceHolder("team-name")
	teamNameEntry.SetText(teamName)

	permissionSelect := widget.NewSelect([]string{"pull", "push", "maintain", "admin"}, nil)
	permissionSelect.Selected = permission

	removeBtn := widget.NewButton("Remove", func() {
		// This will be set when the container is created
	})
	removeBtn.Importance = widget.DangerImportance

	// Create a container with more control over sizing
	rightControls := container.New(
		layout.NewHBoxLayout(),
		permissionSelect,
		removeBtn,
	)
	
	// Use BorderLayout to give the team entry more space
	teamContainer := container.New(
		layout.NewBorderLayout(nil, nil, nil, rightControls),
		rightControls,
		teamNameEntry, // This will take up remaining space
	)

	// Set the remove function to remove this specific container
	removeBtn.OnTapped = func() {
		f.teamsContainer.Remove(teamContainer)
	}

	f.teamsContainer.Add(teamContainer)
}

func (f *FyneApp) handleValidateMigration() {
	f.setStatus("Validating migration configuration...")

	config := f.collectMigrationConfig()

	go func() {
		err := f.service.ValidateMigrationConfig(config)
		if err != nil {
			f.setStatus(fmt.Sprintf("Migration validation failed: %v", err))
			return
		}

		f.setStatus("Migration configuration is valid!")
	}()
}

func (f *FyneApp) handleMigrationDryRun() {
	f.setStatus("Running migration dry run...")

	config := f.collectMigrationConfig()
	config.DryRun = true

	go func() {
		result, err := f.service.MigrateRepository(config)
		if err != nil {
			f.setStatus(fmt.Sprintf("Dry run failed: %v", err))
			return
		}

		fyne.Do(func() {
			f.displayMigrationSteps(result.Steps)
		})
		f.setStatus("Dry run completed successfully!")
		
		// Refresh rate limit after GitHub API calls during migration dry run
		f.refreshRateLimit()
	}()
}

func (f *FyneApp) handleStartMigration() {
	f.setStatus("Starting repository migration...")

	config := f.collectMigrationConfig()
	config.DryRun = false

	go func() {
		result, err := f.service.MigrateRepository(config)
		if err != nil {
			f.setStatus(fmt.Sprintf("Migration failed: %v", err))
			return
		}

		fyne.Do(func() {
			f.displayMigrationSteps(result.Steps)
		})
		if result.Success {
			f.setStatus(fmt.Sprintf("Migration completed! Repository: %s", result.GitHubRepoURL))
		} else {
			f.setStatus(fmt.Sprintf("Migration failed: %s", result.Message))
		}
		
		// Refresh rate limit after GitHub API calls during migration
		f.refreshRateLimit()
	}()
}

func (f *FyneApp) collectMigrationConfig() MigrationConfig {
	teams := make(map[string]string)

	// Collect teams from UI - updated for new BorderLayout structure
	for _, obj := range f.teamsContainer.Objects {
		if teamContainer, ok := obj.(*fyne.Container); ok && len(teamContainer.Objects) >= 2 {
			// New structure: teamContainer has rightControls and teamNameEntry
			var teamEntry *widget.Entry
			var permissionSelect *widget.Select
			
			// Find the team entry (it's the standalone object, not in rightControls)
			for _, containerObj := range teamContainer.Objects {
				if entry, ok := containerObj.(*widget.Entry); ok {
					teamEntry = entry
					break
				}
			}
			
			// Find the permission select (it's inside rightControls container)
			for _, containerObj := range teamContainer.Objects {
				if rightControls, ok := containerObj.(*fyne.Container); ok {
					// Look inside rightControls for the select widget
					for _, rightObj := range rightControls.Objects {
						if selectWidget, ok := rightObj.(*widget.Select); ok {
							permissionSelect = selectWidget
							break
						}
					}
				}
			}
			
			// If we found both widgets, add the team
			if teamEntry != nil && permissionSelect != nil {
				if teamEntry.Text != "" && permissionSelect.Selected != "" {
					teams[teamEntry.Text] = permissionSelect.Selected
				}
			}
		}
	}

	return MigrationConfig{
		SourceBitbucketURL:   f.sourceURLEntry.Text,
		TargetGitHubOrg:      f.targetOrgEntry.Text,
		TargetRepositoryName: f.targetRepoEntry.Text,
		WebhookURL:           f.webhookURLEntry.Text,
		Teams:                teams,
	}
}

func (f *FyneApp) displayMigrationSteps(steps []MigrationStep) {
	f.progressContainer.RemoveAll()

	for _, step := range steps {
		statusIcon := "⏳"
		switch step.Status {
		case "completed":
			statusIcon = "✅"
		case "failed":
			statusIcon = "❌"
		case "running":
			statusIcon = "🔄"
		}

		stepLabel := widget.NewLabel(fmt.Sprintf("%s %s", statusIcon, step.Description))
		if step.Message != "" {
			stepLabel.Text += fmt.Sprintf(" - %s", step.Message)
		}

		progressBar := widget.NewProgressBar()
		progressBar.SetValue(float64(step.Progress) / 100.0)

		stepContainer := container.New(
			layout.NewVBoxLayout(),
			stepLabel,
			progressBar,
		)

		f.progressContainer.Add(stepContainer)
	}
}

func (f *FyneApp) setStatus(status string) {
	f.logger.Info("Status update", "status", status)
	fyne.Do(func() {
		f.statusLabel.SetText(status)
		// Reset to default styling
		f.statusLabel.Importance = widget.MediumImportance
		f.statusIcon.SetResource(theme.InfoIcon())
	})
}

// showLoading shows the loading spinner with a message
func (f *FyneApp) showLoading(message string) {
	fyne.Do(func() {
		f.loadingOverlay.SetMessage(message)
		f.loadingOverlay.Start()
	})
}

// hideLoading hides the loading spinner
func (f *FyneApp) hideLoading() {
	fyne.Do(func() {
		f.loadingOverlay.Stop()
	})
}

// setStatusSuccess sets status with green styling for successful operations
func (f *FyneApp) setStatusSuccess(status string) {
	f.logger.Info("Status update (success)", "status", status)
	fyne.Do(func() {
		f.statusLabel.SetText(status)
		f.statusIcon.SetResource(theme.ConfirmIcon())
		f.statusLabel.Importance = widget.SuccessImportance
		// Show toast notification
		ShowToast(f.window, status, "success")
	})
}

// setStatusError sets status with red styling for error operations
func (f *FyneApp) setStatusError(status string) {
	f.logger.Info("Status update (error)", "status", status)
	fyne.Do(func() {
		f.statusLabel.SetText(status)
		f.statusIcon.SetResource(theme.ErrorIcon())
		f.statusLabel.Importance = widget.DangerImportance
		// Show toast notification
		ShowToast(f.window, status, "error")
	})
}

// String Replacement handlers
func (f *FyneApp) handleAddReplacementRule() {
	originalEntry := widget.NewMultiLineEntry()
	originalEntry.SetPlaceHolder("Enter original text or pattern...")
	originalEntry.Resize(fyne.NewSize(400, 80))

	replacementEntry := widget.NewMultiLineEntry()
	replacementEntry.SetPlaceHolder("Enter replacement text...")
	replacementEntry.Resize(fyne.NewSize(400, 80))

	regexCheck := widget.NewCheck("Regex", nil)
	caseSensitiveCheck := widget.NewCheck("Case Sensitive", nil)
	caseSensitiveCheck.Checked = true
	wholeWordCheck := widget.NewCheck("Whole Word", nil)
	wholeWordCheck.Checked = true

	removeBtn := widget.NewButton("Remove", func() {
		// This will be set when the container is created
	})
	removeBtn.Importance = widget.DangerImportance

	// Create cards for better visual organization
	originalCard := widget.NewCard("", "Original Text", originalEntry)
	replacementCard := widget.NewCard("", "Replacement Text", replacementEntry)

	optionsContainer := container.New(
		layout.NewHBoxLayout(),
		regexCheck,
		caseSensitiveCheck,
		wholeWordCheck,
		layout.NewSpacer(),
		removeBtn,
	)

	ruleContainer := widget.NewCard(
		"",
		"",
		container.New(
			layout.NewVBoxLayout(),
			container.New(
				layout.NewGridLayout(2),
				originalCard,
				replacementCard,
			),
			widget.NewSeparator(),
			optionsContainer,
		),
	)

	// Set the remove function to remove this specific container
	removeBtn.OnTapped = func() {
		f.replacementRulesContainer.Remove(ruleContainer)
	}

	f.replacementRulesContainer.Add(ruleContainer)
	f.replacementRulesContainer.Refresh()

	// Auto-scroll to the newly added rule with a small delay to ensure UI is updated
	go func() {
		time.Sleep(100 * time.Millisecond)
		fyne.Do(func() {
			f.replacementRulesScroll.ScrollToBottom()
		})
	}()
}

func (f *FyneApp) handleLoadRepositories() {
	f.showLoading("Loading repositories...")

	go func() {
		// Simulate some loading time for demo
		time.Sleep(500 * time.Millisecond)

		repos, err := f.service.ListRepositories()

		if err != nil {
			f.hideLoading()
			f.setStatusError(fmt.Sprintf("Failed to load repositories: %v", err))
			return
		}

		// Update UI with repository data
		fyne.Do(func() {
			f.repoSelectionContainer.RemoveAll()
			f.repositories = repos // Store complete repository information
			f.filteredRepositories = repos // Initially all repositories are visible
			f.repoWidgets = make([]*fyne.Container, 0, len(repos)) // Clear existing widgets

			for _, repo := range repos {
				// Create a container with toggle and label
				toggle := NewToggleSwitch("", nil)
				toggle.SetChecked(false) // Deselect all by default

				label := widget.NewLabel(repo.Name)
				label.TextStyle = fyne.TextStyle{Bold: true}

				repoContainer := container.New(
					layout.NewBorderLayout(nil, nil, toggle, nil),
					toggle,
					label,
				)

				f.repoSelectionContainer.Add(repoContainer)
				f.repoWidgets = append(f.repoWidgets, repoContainer) // Store widget reference
			}
			
			// Clear any existing filter
			f.repoFilterEntry.SetText("")
		})

		f.hideLoading()
		f.setStatusSuccess(fmt.Sprintf("Loaded %d repositories", len(repos)))
		
		// Refresh rate limit after GitHub API call
		f.refreshRateLimit()
	}()
}

// handleRepositoryFilter filters repositories based on search text
func (f *FyneApp) handleRepositoryFilter(filterText string) {
	if len(f.repositories) == 0 {
		return // No repositories loaded yet
	}
	
	// Convert filter text to lowercase for case-insensitive search
	filterLower := strings.ToLower(strings.TrimSpace(filterText))
	
	// Filter repositories
	var filteredRepos []Repository
	var visibleIndices []int
	
	for i, repo := range f.repositories {
		if filterLower == "" || strings.Contains(strings.ToLower(repo.Name), filterLower) {
			filteredRepos = append(filteredRepos, repo)
			visibleIndices = append(visibleIndices, i)
		}
	}
	
	f.filteredRepositories = filteredRepos
	
	// Update UI visibility
	f.updateRepositoryVisibility(visibleIndices)
	
	// Update status with filter results
	if filterLower == "" {
		f.setStatus(fmt.Sprintf("Showing all %d repositories", len(f.repositories)))
	} else {
		f.setStatus(fmt.Sprintf("Filter: %d of %d repositories match '%s'", len(filteredRepos), len(f.repositories), filterText))
	}
}

// updateRepositoryVisibility shows/hides repository widgets based on filter
func (f *FyneApp) updateRepositoryVisibility(visibleIndices []int) {
	if len(f.repoWidgets) == 0 {
		return
	}
	
	// Create a set of visible indices for O(1) lookup
	visibleSet := make(map[int]bool)
	for _, idx := range visibleIndices {
		visibleSet[idx] = true
	}
	
	// Remove all widgets from container
	f.repoSelectionContainer.RemoveAll()
	
	// Add only visible widgets back
	for i, widget := range f.repoWidgets {
		if visibleSet[i] {
			f.repoSelectionContainer.Add(widget)
		}
	}
	
	// Add "no results" message if no repositories match
	if len(visibleIndices) == 0 && len(f.repositories) > 0 {
		noResultsLabel := widget.NewLabel("No repositories match the current filter")
		noResultsLabel.Alignment = fyne.TextAlignCenter
		noResultsLabel.TextStyle = fyne.TextStyle{Italic: true}
		f.repoSelectionContainer.Add(container.NewCenter(noResultsLabel))
	}
	
	// Refresh the container to update the UI
	f.repoSelectionContainer.Refresh()
}

func (f *FyneApp) handleValidateReplacement() {
	f.setStatus("Validating replacement configuration...")
	f.operationStatus.SetOperation(OperationGitValidation, "Local validation (no API calls)")

	rules := f.collectReplacementRules()
	if len(rules) == 0 {
		f.setStatusError("Please add at least one replacement rule")
		f.operationStatus.SetOperation(OperationIdle, "")
		return
	}

	f.setStatus("Replacement configuration is valid!")
	f.operationStatus.SetOperation(OperationIdle, "")
}

func (f *FyneApp) handleReplacementDryRun() {
	f.setStatus("Running replacement dry run...")
	f.operationStatus.SetOperation(OperationGitClone, "Analyzing repositories using Git (no API limits consumed)")

	rules := f.collectReplacementRules()
	repos := f.collectSelectedRepositories()

	if len(rules) == 0 {
		f.setStatusError("Please add at least one replacement rule")
		f.operationStatus.SetOperation(OperationIdle, "")
		return
	}

	if len(repos) == 0 {
		f.setStatusError("Please select at least one repository")
		f.operationStatus.SetOperation(OperationIdle, "")
		return
	}

	f.showLoading("Analyzing repositories using Git cloning (no API limits consumed)...")

	options := ProcessingOptions{
		DryRun:          true,
		IncludePatterns: f.includePatternEditor.GetPatterns(),
		ExcludePatterns: f.excludePatternEditor.GetPatterns(),
		PRTitle:         f.prTitleEntry.Text,
	}

	go func() {
		result, err := f.service.ProcessReplacements(rules, repos, options)
		f.hideLoading()

		if err != nil {
			f.setStatus(fmt.Sprintf("Dry run failed: %v", err))
			return
		}

		// Increment API call counter for repository processing (each repo requires multiple API calls)
		for range repos {
			f.operationStatus.IncrementAPICall() // Repository access/clone
		}

		// Count actual repositories with changes
		reposWithChanges := 0
		totalFiles := 0

		if result.Diffs != nil {
			for _, repoDiffs := range result.Diffs {
				hasChanges := false
				fileCount := 0

				for _, fileDiff := range repoDiffs {
					if strings.TrimSpace(fileDiff) != "" {
						hasChanges = true
						fileCount++
					}
				}

				if hasChanges {
					reposWithChanges++
					totalFiles += fileCount
				}
			}
		}

		if reposWithChanges > 0 {
			f.setStatus(fmt.Sprintf("Dry run completed! Found changes in %d repository(ies), %d file(s)", reposWithChanges, totalFiles))
			fyne.Do(func() {
				f.showDiffPreview(result.Diffs, rules, repos, options)
			})
		} else {
			f.setStatus("Dry run completed! No changes found")
		}
		
		// Reset operation status to idle
		f.operationStatus.SetOperation(OperationIdle, "")
		
		// Refresh rate limit after GitHub API calls during dry run
		f.refreshRateLimit()
	}()
}

func (f *FyneApp) handleProcessReplacements() {
	f.setStatus("Processing replacements...")
	f.operationStatus.SetOperation(OperationAPIProcessing, "Creating pull requests (consuming API limits)")

	rules := f.collectReplacementRules()
	repos := f.collectSelectedRepositories()

	if len(rules) == 0 {
		f.setStatusError("Please add at least one replacement rule")
		return
	}

	if len(repos) == 0 {
		f.setStatusError("Please select at least one repository")
		return
	}

	f.showLoading("Processing replacements and creating pull requests...")

	options := ProcessingOptions{
		DryRun:          false,
		IncludePatterns: f.includePatternEditor.GetPatterns(),
		ExcludePatterns: f.excludePatternEditor.GetPatterns(),
		PRTitle:         f.prTitleEntry.Text,
	}

	go func() {
		// First, run a dry run to determine which repositories have changes
		dryRunOptions := options
		dryRunOptions.DryRun = true

		dryResult, err := f.service.ProcessReplacements(rules, repos, dryRunOptions)
		if err != nil {
			f.hideLoading()
			f.setStatusError(fmt.Sprintf("Pre-processing check failed: %v", err))
			return
		}

		// Increment API call counter for dry run processing (pre-processing check)
		for range repos {
			f.operationStatus.IncrementAPICall() // Repository access/clone for dry run
		}

		// Filter repositories to only include those with actual changes
		var reposWithChanges []Repository
		repoMap := make(map[string]Repository)

		// Create a map for quick repository lookup
		for _, repo := range repos {
			repoMap[repo.FullName] = repo
		}

		// Check which repositories have actual changes
		if dryResult.Diffs != nil {
			for repoName, repoDiffs := range dryResult.Diffs {
				hasChanges := false
				for _, fileDiff := range repoDiffs {
					if strings.TrimSpace(fileDiff) != "" {
						hasChanges = true
						break
					}
				}

				if hasChanges {
					if repo, exists := repoMap[repoName]; exists {
						reposWithChanges = append(reposWithChanges, repo)
					}
				}
			}
		}

		if len(reposWithChanges) == 0 {
			f.hideLoading()
			f.setStatus("No changes found to apply")
			return
		}

		f.setStatus(fmt.Sprintf("Applying changes to %d repository(ies) with actual changes...", len(reposWithChanges)))

		// Now process only repositories with changes
		options.DryRun = false
		result, err := f.service.ProcessReplacements(rules, reposWithChanges, options)
		if err != nil {
			f.hideLoading()
			f.setStatusError(fmt.Sprintf("Processing failed: %v", err))
			return
		}

		// Increment API call counter for actual processing (PR creation)
		for range reposWithChanges {
			f.operationStatus.IncrementAPICall() // Pull request creation
		}

		f.hideLoading()
		if result.Success {
			f.setStatusSuccess(fmt.Sprintf("Processing completed! %d repositories processed", len(result.RepositoryResults)))
			fyne.Do(func() {
				f.showResultsDialog(result)
			})
		} else {
			f.setStatusError(fmt.Sprintf("Processing failed: %s", result.Message))
		}
		
		// Refresh rate limit after GitHub API calls during processing
		f.refreshRateLimit()
	}()
}

func (f *FyneApp) collectReplacementRules() []ReplacementRule {
	var rules []ReplacementRule

	for _, obj := range f.replacementRulesContainer.Objects {
		if card, ok := obj.(*widget.Card); ok {
			// Get the content container from the card
			if content, ok := card.Content.(*fyne.Container); ok && len(content.Objects) >= 3 {
				// Get the grid container with the two cards
				if gridContainer, ok := content.Objects[0].(*fyne.Container); ok && len(gridContainer.Objects) >= 2 {
					// Get the original and replacement cards
					if originalCard, ok := gridContainer.Objects[0].(*widget.Card); ok {
						if replacementCard, ok := gridContainer.Objects[1].(*widget.Card); ok {
							// Extract the entries from the cards
							if originalEntry, ok := originalCard.Content.(*widget.Entry); ok {
								if replacementEntry, ok := replacementCard.Content.(*widget.Entry); ok {
									if originalEntry.Text != "" && replacementEntry.Text != "" {
										// Get the options from the options container
										var regex, caseSensitive, wholeWord bool
										if len(content.Objects) >= 3 {
											if optionsContainer, ok := content.Objects[2].(*fyne.Container); ok && len(optionsContainer.Objects) >= 3 {
												if regexCheck, ok := optionsContainer.Objects[0].(*widget.Check); ok {
													regex = regexCheck.Checked
												}
												if caseCheck, ok := optionsContainer.Objects[1].(*widget.Check); ok {
													caseSensitive = caseCheck.Checked
												}
												if wholeCheck, ok := optionsContainer.Objects[2].(*widget.Check); ok {
													wholeWord = wholeCheck.Checked
												}
											}
										}

										// Show whole word status in status bar for debugging
										if wholeWord {
											f.setStatus(fmt.Sprintf("Rule collected with WHOLE WORD enabled: %s → %s", originalEntry.Text, replacementEntry.Text))
										}

										rules = append(rules, ReplacementRule{
											Original:      originalEntry.Text,
											Replacement:   replacementEntry.Text,
											Regex:         regex,
											CaseSensitive: caseSensitive,
											WholeWord:     wholeWord,
										})
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return rules
}

func (f *FyneApp) collectSelectedRepositories() []Repository {
	var selectedRepos []Repository

	// Since widgets might be filtered, we need to map visible widgets back to repository indices
	if len(f.repoWidgets) == 0 {
		return selectedRepos
	}

	// Check each repository widget for selection state
	for i, repoWidget := range f.repoWidgets {
		if toggle, ok := repoWidget.Objects[0].(*ToggleSwitch); ok {
			if toggle.Checked && i < len(f.repositories) {
				repo := f.repositories[i]
				repo.Selected = true
				selectedRepos = append(selectedRepos, repo)
			}
		}
	}

	return selectedRepos
}

func (f *FyneApp) parsePatterns(patterns string) []string {
	if patterns == "" {
		return nil
	}

	var result []string
	for _, pattern := range strings.Split(patterns, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern != "" {
			result = append(result, pattern)
		}
	}

	return result
}

// joinPatterns converts a slice of patterns back to comma-separated string for GUI display
func (f *FyneApp) joinPatterns(patterns []string) string {
	return strings.Join(patterns, ",")
}

func (f *FyneApp) handleSelectAllRepos() {
	// Select all currently visible repositories (filtered)
	for _, obj := range f.repoSelectionContainer.Objects {
		if container, ok := obj.(*fyne.Container); ok {
			if toggle, ok := container.Objects[0].(*ToggleSwitch); ok {
				toggle.SetChecked(true)
			}
		}
	}
	
	// Update status to show how many were selected
	visibleCount := len(f.repoSelectionContainer.Objects)
	if visibleCount > 0 {
		f.setStatus(fmt.Sprintf("Selected all %d visible repositories", visibleCount))
	}
}

func (f *FyneApp) handleDeselectAllRepos() {
	// Deselect all currently visible repositories (filtered)  
	for _, obj := range f.repoSelectionContainer.Objects {
		if container, ok := obj.(*fyne.Container); ok {
			if toggle, ok := container.Objects[0].(*ToggleSwitch); ok {
				toggle.SetChecked(false)
			}
		}
	}
	
	// Update status to show how many were deselected
	visibleCount := len(f.repoSelectionContainer.Objects)
	if visibleCount > 0 {
		f.setStatus(fmt.Sprintf("Deselected all %d visible repositories", visibleCount))
	}
}

func (f *FyneApp) showDiffPreview(diffs map[string]map[string]string, rules []ReplacementRule, repos []Repository, options ProcessingOptions) {
	// Create new window for diff preview
	diffWindow := f.app.NewWindow("Diff Preview - Dry Run Results")
	diffWindow.Resize(fyne.NewSize(1000, 700))
	diffWindow.CenterOnScreen()

	// Calculate statistics
	totalFiles := 0
	totalAdditions := 0
	totalDeletions := 0

	for _, repoDiffs := range diffs {
		for _, fileDiff := range repoDiffs {
			totalFiles++
			lines := strings.Split(fileDiff, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
					totalAdditions++
				}
				if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
					totalDeletions++
				}
			}
		}
	}

	// Create stats header
	statsText := fmt.Sprintf("📊 %d file(s), +%d additions, -%d deletions", totalFiles, totalAdditions, totalDeletions)
	statsLabel := widget.NewLabel(statsText)
	statsLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Create diff content
	diffContent := f.createDiffContent(diffs)

	// Create scrollable container for diffs
	diffScroll := container.NewScroll(diffContent)
	diffScroll.SetMinSize(fyne.NewSize(950, 400))

	// Create action buttons
	applyBtn := widget.NewButtonWithIcon("Apply Changes", theme.ConfirmIcon(), func() {
		diffWindow.Close()
		f.applyChanges(rules, repos, options)
	})
	applyBtn.Importance = widget.HighImportance

	cancelBtn := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		diffWindow.Close()
		f.setStatus("Changes cancelled")
	})
	cancelBtn.Importance = widget.MediumImportance

	buttonsContainer := container.New(
		layout.NewHBoxLayout(),
		layout.NewSpacer(),
		cancelBtn,
		applyBtn,
	)

	// Create main content
	content := container.New(
		layout.NewVBoxLayout(),
		widget.NewCard("", "Diff Preview", container.New(
			layout.NewVBoxLayout(),
			statsLabel,
			widget.NewSeparator(),
			diffScroll,
		)),
		container.NewPadded(buttonsContainer),
	)

	diffWindow.SetContent(content)
	diffWindow.Show()
}

func (f *FyneApp) createDiffContent(diffs map[string]map[string]string) *fyne.Container {
	content := container.New(layout.NewVBoxLayout())

	for repoName, repoDiffs := range diffs {
		// Skip repositories with no diffs
		if len(repoDiffs) == 0 {
			continue
		}

		// Check if there are actual changes (not just empty diffs)
		hasChanges := false
		for _, fileDiff := range repoDiffs {
			if strings.TrimSpace(fileDiff) != "" {
				hasChanges = true
				break
			}
		}

		if !hasChanges {
			continue
		}

		// Create repository accordion only if there are actual changes
		repoAccordion := f.createRepositoryAccordion(repoName, repoDiffs)
		content.Add(repoAccordion)
		content.Add(widget.NewSeparator())
	}

	return content
}

func (f *FyneApp) createRepositoryAccordion(repoName string, repoDiffs map[string]string) *widget.Accordion {
	// Create container for all files in this repository
	filesContainer := container.New(layout.NewVBoxLayout())

	for fileName, fileDiff := range repoDiffs {
		// Skip files with no actual diff content
		if strings.TrimSpace(fileDiff) == "" {
			continue
		}

		// Create expandable file entry
		fileEntry := f.createExpandableFileEntry(fileName, fileDiff)
		filesContainer.Add(fileEntry)
	}

	// Calculate total stats for repository (only files with actual changes)
	totalFiles := 0
	totalAdditions := 0
	totalDeletions := 0

	for _, fileDiff := range repoDiffs {
		if strings.TrimSpace(fileDiff) == "" {
			continue
		}

		totalFiles++
		lines := strings.Split(fileDiff, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				totalAdditions++
			}
			if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				totalDeletions++
			}
		}
	}

	repoHeaderText := fmt.Sprintf("📁 %s (%d files, +%d -%d)", repoName, totalFiles, totalAdditions, totalDeletions)

	// Create repository accordion
	accordion := widget.NewAccordion(
		widget.NewAccordionItem(repoHeaderText, filesContainer),
	)

	return accordion
}

func (f *FyneApp) createExpandableFileEntry(fileName string, diffContent string) *widget.Card {
	// Calculate diff statistics for this file
	lines := strings.Split(diffContent, "\n")
	additions := 0
	deletions := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			additions++
		}
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deletions++
		}
	}

	// Create header with file name and statistics
	headerText := fmt.Sprintf("📄 %s (+%d -%d)", fileName, additions, deletions)
	headerLabel := widget.NewLabel(headerText)
	headerLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Create show/hide button
	// Button to open diff in new window
	toggleBtn := widget.NewButton("🔍 View Diff", func() {
		f.openDiffWindow(fileName, diffContent)
	})

	// Create header container with label and toggle button
	headerContainer := container.New(
		layout.NewBorderLayout(nil, nil, nil, toggleBtn),
		headerLabel,
		toggleBtn,
	)

	// Create main content container
	cardContent := container.New(
		layout.NewVBoxLayout(),
		headerContainer,
	)

	return widget.NewCard("", "", cardContent)
}

// openDiffWindow opens a file's diff content in a dedicated window
func (f *FyneApp) openDiffWindow(fileName, diffContent string) {
	// Create new window for the file diff
	diffWindow := f.app.NewWindow(fmt.Sprintf("Diff: %s", fileName))
	diffWindow.Resize(fyne.NewSize(1400, 800))
	diffWindow.CenterOnScreen()

	// Parse the diff and extract only the changed hunks
	formattedDiff := f.formatDiffHunks(diffContent)

	// Use colored RichText for better syntax highlighting
	diffText := f.createColoredDiffText(formattedDiff)
	diffText.Wrapping = fyne.TextWrapWord

	diffScroll := container.NewScroll(diffText)

	// Add some padding around the content
	content := container.NewPadded(diffScroll)

	// Close button at the bottom
	closeBtn := widget.NewButton("Close", func() {
		diffWindow.Close()
	})
	closeBtn.Importance = widget.MediumImportance

	// Create layout with close button at bottom
	layout := container.New(
		layout.NewBorderLayout(nil, closeBtn, nil, nil),
		content,
		closeBtn,
	)

	diffWindow.SetContent(layout)

	// Show window and bring to front
	diffWindow.Show()
	diffWindow.RequestFocus()

	// Additional attempt to bring window to front
	go func() {
		time.Sleep(50 * time.Millisecond)
		fyne.Do(func() {
			diffWindow.RequestFocus()
		})
	}()
}

// formatDiffHunks parses a diff and formats only the changed hunks with line numbers
func (f *FyneApp) formatDiffHunks(diffContent string) string {
	// Force aggressive filtering - always use it regardless of size reduction
	return f.aggressiveFilterDiff(diffContent)
}

// aggressiveFilterDiff filters diff content while preserving structure
func (f *FyneApp) aggressiveFilterDiff(diffContent string) string {
	// Simply return the original diff content to preserve structure
	// The generateDiffFromFileChange already creates a proper, concise diff
	lines := strings.Split(diffContent, "\n")
	const maxLines = 100 // Allow more lines to show meaningful diffs
	
	// If the diff is too long, truncate but preserve structure
	if len(lines) > maxLines {
		var result []string
		result = append(result, lines[:maxLines]...)
		result = append(result, "")
		result = append(result, fmt.Sprintf("... [Truncated - showing first %d lines of %d total] ...", maxLines, len(lines)))
		return strings.Join(result, "\n")
	}
	
	// Return original diff content to preserve proper structure
	return diffContent
}

// extractEssentialDiff extracts only the essential diff parts when parsing fails
func (f *FyneApp) extractEssentialDiff(diffContent string) string {
	lines := strings.Split(diffContent, "\n")
	var result strings.Builder

	var currentHunk []string
	var hunkLineNum int
	inHunk := false

	for _, line := range lines {
		// Start of a hunk
		if strings.HasPrefix(line, "@@") {
			// If we have a previous hunk, add it to result
			if len(currentHunk) > 0 {
				result.WriteString(f.formatHunkLines(currentHunk, hunkLineNum))
				result.WriteString("\n" + strings.Repeat("─", 40) + "\n\n")
			}

			// Start new hunk
			result.WriteString(line + "\n")
			currentHunk = []string{}
			hunkLineNum = f.extractLineNumber(line)
			inHunk = true
			continue
		}

		// If we're in a hunk, collect lines
		if inHunk && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-")) {
			currentHunk = append(currentHunk, line)
		} else if inHunk && !strings.HasPrefix(line, "\\") {
			// End of hunk (reached a line that's not part of the hunk)
			inHunk = false
			if len(currentHunk) > 0 {
				result.WriteString(f.formatHunkLines(currentHunk, hunkLineNum))
				result.WriteString("\n" + strings.Repeat("─", 40) + "\n\n")
				currentHunk = []string{}
			}
		}
	}

	// Add the last hunk if exists
	if len(currentHunk) > 0 {
		result.WriteString(f.formatHunkLines(currentHunk, hunkLineNum))
	}

	return result.String()
}

// formatHunkLines formats the lines within a hunk with line numbers
func (f *FyneApp) formatHunkLines(hunkLines []string, startLine int) string {
	var result strings.Builder
	lineNum := startLine

	for _, line := range hunkLines {
		if len(line) == 0 {
			continue
		}

		switch line[0] {
		case ' ': // Context line
			result.WriteString(fmt.Sprintf("%4d   %s\n", lineNum, line[1:]))
			lineNum++
		case '-': // Deleted line
			result.WriteString(fmt.Sprintf("%4d - %s\n", lineNum, line[1:]))
			lineNum++
		case '+': // Added line
			result.WriteString(fmt.Sprintf("%4d + %s\n", lineNum, line[1:]))
			// Don't increment lineNum for added lines in this simple case
		}
	}

	return result.String()
}

// extractLineNumber extracts the starting line number from a hunk header
func (f *FyneApp) extractLineNumber(hunkHeader string) int {
	// Parse "@@-oldStart,oldLines +newStart,newLines @@"
	parts := strings.Split(hunkHeader, " ")
	for _, part := range parts {
		if strings.HasPrefix(part, "+") {
			numPart := strings.TrimPrefix(part, "+")
			if commaIdx := strings.Index(numPart, ","); commaIdx > 0 {
				numPart = numPart[:commaIdx]
			}
			var lineNum int
			if n, _ := fmt.Sscanf(numPart, "%d", &lineNum); n == 1 {
				return lineNum
			}
		}
	}
	return 1
}

// addLineNumbersToRawDiff adds line numbers to raw diff content as fallback
func (f *FyneApp) addLineNumbersToRawDiff(diffContent string) string {
	const maxLines = 500
	lines := strings.Split(diffContent, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, fmt.Sprintf("... [Truncated - showing first %d lines] ...", maxLines))
	}

	var numberedLines []string
	for i, line := range lines {
		lineNumber := fmt.Sprintf("%4d: %s", i+1, line)
		numberedLines = append(numberedLines, lineNumber)
	}

	return strings.Join(numberedLines, "\n")
}

func (f *FyneApp) createColoredDiffText(diffContent string) *widget.RichText {
	richText := widget.NewRichText()
	richText.Wrapping = fyne.TextWrapWord

	lines := strings.Split(diffContent, "\n")

	for _, line := range lines {
		var segment *widget.TextSegment

		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			// Addition line - green
			segment = &widget.TextSegment{
				Text: line + "\n",
				Style: widget.RichTextStyle{
					ColorName: theme.ColorNameSuccess,
				},
			}
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			// Deletion line - red
			segment = &widget.TextSegment{
				Text: line + "\n",
				Style: widget.RichTextStyle{
					ColorName: theme.ColorNameError,
				},
			}
		} else if strings.HasPrefix(line, "@@") {
			// Hunk header - blue
			segment = &widget.TextSegment{
				Text: line + "\n",
				Style: widget.RichTextStyle{
					ColorName: theme.ColorNamePrimary,
					TextStyle: fyne.TextStyle{Bold: true},
				},
			}
		} else if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			// File headers - gray
			segment = &widget.TextSegment{
				Text: line + "\n",
				Style: widget.RichTextStyle{
					ColorName: theme.ColorNameDisabled,
					TextStyle: fyne.TextStyle{Italic: true},
				},
			}
		} else {
			// Context line - normal
			segment = &widget.TextSegment{
				Text: line + "\n",
				Style: widget.RichTextStyle{
					ColorName: theme.ColorNameForeground,
				},
			}
		}

		richText.Segments = append(richText.Segments, segment)
	}

	return richText
}

func (f *FyneApp) applyChanges(rules []ReplacementRule, repos []Repository, options ProcessingOptions) {
	f.setStatus("Applying changes and creating pull requests...")
	f.showLoading("Applying changes and creating pull requests...")

	go func() {
		// First, run a dry run to determine which repositories have changes
		dryRunOptions := options
		dryRunOptions.DryRun = true

		dryResult, err := f.service.ProcessReplacements(rules, repos, dryRunOptions)
		if err != nil {
			f.hideLoading()
			f.setStatusError(fmt.Sprintf("Pre-processing check failed: %v", err))
			return
		}

		// Increment API call counter for dry run processing (pre-processing check)
		for range repos {
			f.operationStatus.IncrementAPICall() // Repository access/clone for dry run
		}

		// Filter repositories to only include those with actual changes
		var reposWithChanges []Repository
		repoMap := make(map[string]Repository)

		// Create a map for quick repository lookup
		for _, repo := range repos {
			repoMap[repo.FullName] = repo
		}

		// Check which repositories have actual changes
		if dryResult.Diffs != nil {
			for repoName, repoDiffs := range dryResult.Diffs {
				hasChanges := false
				for _, fileDiff := range repoDiffs {
					if strings.TrimSpace(fileDiff) != "" {
						hasChanges = true
						break
					}
				}

				if hasChanges {
					if repo, exists := repoMap[repoName]; exists {
						reposWithChanges = append(reposWithChanges, repo)
					}
				}
			}
		}

		if len(reposWithChanges) == 0 {
			f.hideLoading()
			f.setStatus("No changes found to apply")
			return
		}

		f.setStatus(fmt.Sprintf("Applying changes to %d repository(ies) with actual changes...", len(reposWithChanges)))

		// Now process only repositories with changes
		options.DryRun = false
		result, err := f.service.ProcessReplacements(rules, reposWithChanges, options)
		if err != nil {
			f.hideLoading()
			f.setStatusError(fmt.Sprintf("Processing failed: %v", err))
			return
		}

		// Increment API call counter for actual processing (PR creation)
		for range reposWithChanges {
			f.operationStatus.IncrementAPICall() // Pull request creation
		}

		f.hideLoading()
		if result.Success {
			f.setStatusSuccess(fmt.Sprintf("Processing completed! %d repositories processed", len(result.RepositoryResults)))
			fyne.Do(func() {
				f.showResultsDialog(result)
			})
		} else {
			f.setStatusError(fmt.Sprintf("Processing failed: %s", result.Message))
		}
		
		// Refresh rate limit after GitHub API calls during processing
		f.refreshRateLimit()
	}()
}

func (f *FyneApp) showResultsDialog(result *ProcessingResult) {
	// Create results window
	resultsWindow := f.app.NewWindow("Processing Results")
	resultsWindow.Resize(fyne.NewSize(800, 600))
	resultsWindow.CenterOnScreen()

	content := container.New(layout.NewVBoxLayout())

	for _, repoResult := range result.RepositoryResults {
		var statusIcon string

		if repoResult.Success {
			statusIcon = "✅"
		} else {
			statusIcon = "❌"
		}

		// Repository result
		resultText := fmt.Sprintf("%s %s\n%s\nFiles changed: %d, Replacements: %d",
			statusIcon, repoResult.Repository, repoResult.Message,
			len(repoResult.FilesChanged), repoResult.Replacements)

		resultLabel := widget.NewLabel(resultText)
		resultLabel.Wrapping = fyne.TextWrapWord

		var resultCard *widget.Card
		if repoResult.PRUrl != "" {
			// Add PR link button
			prBtn := widget.NewButtonWithIcon("View Pull Request", theme.ComputerIcon(), func() {
				// Open PR URL in browser
				f.openURL(repoResult.PRUrl)
			})
			prBtn.Importance = widget.HighImportance

			cardContent := container.New(layout.NewVBoxLayout(), resultLabel, prBtn)
			resultCard = widget.NewCard("", "", cardContent)
		} else {
			resultCard = widget.NewCard("", "", resultLabel)
		}

		content.Add(resultCard)
	}

	// Scroll container
	scroll := container.NewScroll(content)

	// Close button
	closeBtn := widget.NewButton("Close", func() {
		resultsWindow.Close()
	})

	mainContent := container.New(
		layout.NewBorderLayout(nil, closeBtn, nil, nil),
		scroll,
		closeBtn,
	)

	resultsWindow.SetContent(mainContent)
	resultsWindow.Show()
}

// loadConfigurationFromFile loads configuration from ./config.yaml and prefills the GUI
func (f *FyneApp) loadConfigurationFromFile() {
	configData, err := f.service.ReadConfigFromFile()
	if err != nil {
		f.setStatusError(fmt.Sprintf("Failed to load configuration: %v", err))
		return
	}

	// Set provider and update fields (this will trigger the provider change callback)
	if configData.Provider != "" {
		f.providerSelect.SetSelected(configData.Provider)
		// Manually trigger the update since SetSelected doesn't always trigger callback
		f.updateProviderFields(configData.Provider)
	} else {
		// Default to GitHub if no provider is set
		f.providerSelect.SetSelected("github")
		f.updateProviderFields("github")
	}

	// Prefill provider-specific fields using dedicated widget instances
	if configData.Provider == "github" {
		if configData.GitHubURL != "" {
			f.githubURLEntry.SetText(configData.GitHubURL)
		}
		if configData.Token != "" {
			f.githubTokenEntry.SetText(configData.Token)
		}
		if configData.Organization != "" {
			f.githubOrgEntry.SetText(configData.Organization)
		}
		if configData.Team != "" {
			f.githubTeamEntry.SetText(configData.Team)
		}
	} else if configData.Provider == "bitbucket" {
		if configData.BitbucketURL != "" {
			f.bitbucketURLEntry.SetText(configData.BitbucketURL)
		}
		if configData.Username != "" {
			f.bitbucketUsernameEntry.SetText(configData.Username)
		}
		if configData.Password != "" {
			f.bitbucketPasswordEntry.SetText(configData.Password)
		}
		if configData.Project != "" {
			f.bitbucketProjectEntry.SetText(configData.Project)
		}
	}

	// Prefill pattern fields
	if len(configData.IncludePatterns) > 0 {
		f.includePatternEditor.SetPatterns(configData.IncludePatterns)
	}
	if len(configData.ExcludePatterns) > 0 {
		f.excludePatternEditor.SetPatterns(configData.ExcludePatterns)
	}

	// Prefill PR template fields
	if configData.PRTitleTemplate != "" {
		f.prTitleEntry.SetText(configData.PRTitleTemplate)
	}
	if configData.PRBodyTemplate != "" {
		f.prBodyEntry.SetText(configData.PRBodyTemplate)
	}
	if configData.BranchPrefix != "" {
		f.branchPrefixEntry.SetText(configData.BranchPrefix)
	}

	// Prefill migration fields (always set, even if empty to allow clearing)
	f.sourceURLEntry.SetText(configData.MigrationSourceURL)
	f.targetOrgEntry.SetText(configData.MigrationTargetOrg)
	f.targetRepoEntry.SetText(configData.MigrationTargetRepo)
	f.webhookURLEntry.SetText(configData.MigrationWebhookURL)
	
	// Prefill migration teams (always clear and reload to sync with config)
	f.teamsContainer.RemoveAll()
	if len(configData.MigrationTeams) > 0 {
		// Add teams from config
		for teamName, permission := range configData.MigrationTeams {
			f.addTeamFromConfig(teamName, permission)
		}
	}

	// Initialize the service configuration without saving to disk
	err = f.service.InitializeServiceConfig(*configData)
	if err != nil {
		f.setStatusError(fmt.Sprintf("Failed to initialize GitHub client: %v", err))
		return
	}

	f.setStatusSuccess("Configuration loaded from ./config.yaml")
}

// openURL opens the given URL in the default browser
func (f *FyneApp) openURL(urlString string) {
	f.logger.Info("Opening URL in browser", "url", urlString)

	// Parse the URL string
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		f.logger.Error("Failed to parse URL", "url", urlString, "error", err)
		f.setStatus(fmt.Sprintf("Invalid URL: %s", urlString))
		return
	}

	// Use the Fyne app's OpenURL method which handles cross-platform browser opening
	err = f.app.OpenURL(parsedURL)
	if err != nil {
		f.logger.Error("Failed to open URL", "url", urlString, "error", err)
		f.setStatus(fmt.Sprintf("Failed to open URL: %v", err))
		return
	}

	f.setStatus(fmt.Sprintf("Opened %s in browser", urlString))
}

// refreshRateLimit fetches and updates rate limit information
func (f *FyneApp) refreshRateLimit() {
	if f.service == nil {
		f.rateLimitStatus.ShowError(fmt.Errorf("service not initialized"))
		return
	}
	
	// Temporarily show that we're checking rate limits
	currentOp := f.operationStatus.StatusLabel.Text
	if currentOp == "Idle" {
		f.operationStatus.SetOperation(OperationAPIRateLimit, "")
	}
	
	rateLimitInfo, err := f.service.GetRateLimitInfo()
	if err != nil {
		f.rateLimitStatus.ShowError(err)
		if currentOp == "Idle" {
			f.operationStatus.SetOperation(OperationIdle, "")
		}
		return
	}
	
	// Increment API call counter for rate limit check
	f.operationStatus.IncrementAPICall()
	
	// Reset operation status if it was idle before
	if currentOp == "Idle" {
		f.operationStatus.SetOperation(OperationIdle, "")
	}
	
	f.rateLimitStatus.UpdateRateLimit(
		rateLimitInfo.Core.Remaining,
		rateLimitInfo.Core.Limit,
		rateLimitInfo.Search.Remaining,
		rateLimitInfo.Search.Limit,
		rateLimitInfo.Core.Reset,
		rateLimitInfo.Search.Reset,
	)
}

// startRateLimitRefreshTimer starts a background timer to refresh rate limit information
func (f *FyneApp) startRateLimitRefreshTimer() {
	// Initial refresh after a short delay to allow UI setup
	go func() {
		time.Sleep(2 * time.Second)
		fyne.Do(func() {
			f.refreshRateLimit()
		})
		
		// Set up periodic refresh every 30 seconds
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			fyne.Do(func() {
				f.refreshRateLimit()
			})
		}
	}()
}
