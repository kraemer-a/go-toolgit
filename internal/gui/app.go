package gui

import (
	"context"
	"embed"
	"fmt"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"go-toolgit/internal/core/config"
	"go-toolgit/internal/core/utils"
)

type App struct {
	ctx     context.Context
	config  *config.Config
	logger  *utils.Logger
	service *Service
}

func NewApp() *App {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: Failed to load config: %v", err)
		cfg = &config.Config{} // Use empty config as fallback
	}

	// Force debug logging for GUI
	logLevel := cfg.Logging.Level
	if logLevel == "" {
		logLevel = "debug"
	}
	logger := utils.NewLogger(logLevel, cfg.Logging.Format)
	service := NewService(cfg, logger)

	return &App{
		config:  cfg,
		logger:  logger,
		service: service,
	}
}

func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
	a.logger.Info("GUI application started")
}

func (a *App) OnDomReady(ctx context.Context) {
	a.logger.Info("DOM ready")
}

func (a *App) OnShutdown(ctx context.Context) {
	a.logger.Info("GUI application shutting down")
}

func (a *App) Run(ctx context.Context) error {
	return a.RunWithAssets(ctx, (*embed.FS)(nil))
}

func (a *App) RunWithAssets(ctx context.Context, assets *embed.FS) error {
	a.logger.Info("Starting Wails GUI application")

	var assetServerOptions *assetserver.Options
	if assets != nil {
		assetServerOptions = &assetserver.Options{
			Assets: *assets,
		}
	} else {
		emptyFS := embed.FS{}
		assetServerOptions = &assetserver.Options{
			Assets: emptyFS,
		}
	}

	err := wails.Run(&options.App{
		Title:            "GitHub & Bitbucket Replace Tool",
		Width:            1200,
		Height:           800,
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		AssetServer:      assetServerOptions,
		OnStartup:        a.OnStartup,
		OnDomReady:       a.OnDomReady,
		OnShutdown:       a.OnShutdown,
		Bind: []interface{}{
			a,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to run Wails application: %w", err)
	}

	return nil
}

func (a *App) GetService() *Service {
	return a.service
}

// Expose service methods directly for frontend calls

// Test method to verify binding works
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, it's show time!", name)
}

func (a *App) GetConfig() ConfigData {
	return a.service.GetConfig()
}

func (a *App) UpdateConfig(cfg ConfigData) error {
	a.logger.Info("UpdateConfig called from frontend", "config", cfg)
	err := a.service.UpdateConfig(cfg)
	if err != nil {
		a.logger.Error("UpdateConfig failed", "error", err)
	} else {
		a.logger.Info("UpdateConfig successful")
	}
	return err
}

func (a *App) ValidateAccess() error {
	return a.service.ValidateAccess()
}

func (a *App) ListRepositories() ([]Repository, error) {
	return a.service.ListRepositories()
}

func (a *App) ProcessReplacements(rules []ReplacementRule, selectedRepos []Repository, options ProcessingOptions) (*ProcessingResult, error) {
	return a.service.ProcessReplacements(rules, selectedRepos, options)
}

func (a *App) SearchRepositories(criteria SearchCriteria) ([]Repository, error) {
	return a.service.SearchRepositories(criteria)
}

func (a *App) ProcessSearchReplacements(criteria SearchCriteria, rules []ReplacementRule, options ProcessingOptions) (*ProcessingResult, error) {
	return a.service.ProcessSearchReplacements(criteria, rules, options)
}

func (a *App) GetDefaultIncludePatterns() []string {
	return a.service.GetDefaultIncludePatterns()
}

func (a *App) GetDefaultExcludePatterns() []string {
	return a.service.GetDefaultExcludePatterns()
}

func (a *App) GetCurrentProvider() string {
	return a.service.GetCurrentProvider()
}

func (a *App) GetSupportedProviders() []string {
	return a.service.GetSupportedProviders()
}
