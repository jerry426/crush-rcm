package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/llm/prompt"
	"github.com/charmbracelet/crush/internal/tui"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/charmbracelet/fang"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.PersistentFlags().StringP("cwd", "c", "", "Current working directory")
	rootCmd.PersistentFlags().StringP("data-dir", "D", "", "Custom crush data directory")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Debug")

	rootCmd.Flags().BoolP("help", "h", false, "Help")
	rootCmd.Flags().BoolP("yolo", "y", false, "Automatically accept all permissions (dangerous mode)")

	// Agent configuration flags
	rootCmd.Flags().String("large-model", "", "Override large model (format: provider/model)")
	rootCmd.Flags().String("small-model", "", "Override small model (format: provider/model)")
	rootCmd.Flags().String("model", "", "Set model for agent (simpler than large/small, format: provider/model)")
	rootCmd.Flags().String("agent-id", "", "Unique agent identifier for tracking")
	rootCmd.Flags().String("conversation-id", "", "Resume existing conversation")
	rootCmd.Flags().String("system-prompt-file", "", "Path to system prompt markdown file")

	// Provider configuration flags (for CLI-driven provider setup)
	rootCmd.Flags().String("provider-id", "", "Provider identifier (e.g., synthetic, anthropic)")
	rootCmd.Flags().String("provider-type", "", "Provider type (openai, anthropic, gemini)")
	rootCmd.Flags().String("provider-url", "", "Provider API endpoint URL")
	rootCmd.Flags().String("provider-api-key", "", "Provider API key")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(updateProvidersCmd)
}

var rootCmd = &cobra.Command{
	Use:   "crush",
	Short: "Terminal-based AI assistant for software development",
	Long: `Crush is a powerful terminal-based AI assistant that helps with software development tasks.
It provides an interactive chat interface with AI capabilities, code analysis, and LSP integration
to assist developers in writing, debugging, and understanding code directly from the terminal.`,
	Example: `
# Run in interactive mode
crush

# Run with debug logging
crush -d

# Run with debug logging in a specific directory
crush -d -c /path/to/project

# Run with custom data directory
crush -D /path/to/custom/.crush

# Print version
crush -v

# Run a single non-interactive prompt
crush run "Explain the use of context in Go"

# Run in dangerous mode (auto-accept all permissions)
crush -y
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		event.AppInitialized()

		// Set up the TUI.
		program := tea.NewProgram(
			tui.New(app),
			tea.WithAltScreen(),
			tea.WithContext(cmd.Context()),
			tea.WithMouseCellMotion(),            // Use cell motion instead of all motion to reduce event flooding
			tea.WithFilter(tui.MouseEventFilter), // Filter mouse events based on focus state
		)

		go app.Subscribe(program)

		if _, err := program.Run(); err != nil {
			event.Error(err)
			slog.Error("TUI run error", "error", err)
			return fmt.Errorf("TUI error: %v", err)
		}
		return nil
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		event.AppExited()
	},
}

func Execute() {
	if err := fang.Execute(
		context.Background(),
		rootCmd,
		fang.WithVersion(version.Version),
		fang.WithNotifySignal(os.Interrupt),
	); err != nil {
		os.Exit(1)
	}
}

// setupApp handles the common setup logic for both interactive and non-interactive modes.
// It returns the app instance, config, cleanup function, and any error.
func setupApp(cmd *cobra.Command) (*app.App, error) {
	debug, _ := cmd.Flags().GetBool("debug")
	yolo, _ := cmd.Flags().GetBool("yolo")
	dataDir, _ := cmd.Flags().GetString("data-dir")
	ctx := cmd.Context()

	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Init(cwd, dataDir, debug)
	if err != nil {
		return nil, err
	}

	// Override models from CLI flags if provided
	if err := applyModelOverrides(cmd, cfg); err != nil {
		return nil, err
	}

	// Set custom system prompt file if provided
	if systemPromptFile, _ := cmd.Flags().GetString("system-prompt-file"); systemPromptFile != "" {
		// Validate file exists
		if _, err := os.Stat(systemPromptFile); err != nil {
			return nil, fmt.Errorf("system prompt file not found: %s", systemPromptFile)
		}
		prompt.SetCustomPromptFile(systemPromptFile)
		slog.Debug("Using custom system prompt", "file", systemPromptFile)
	}

	if cfg.Permissions == nil {
		cfg.Permissions = &config.Permissions{}
	}
	cfg.Permissions.SkipRequests = yolo

	if err := createDotCrushDir(cfg.Options.DataDirectory); err != nil {
		return nil, err
	}

	// Connect to DB; this will also run migrations.
	conn, err := db.Connect(ctx, cfg.Options.DataDirectory)
	if err != nil {
		return nil, err
	}

	appInstance, err := app.New(ctx, conn, cfg)
	if err != nil {
		slog.Error("Failed to create app instance", "error", err)
		return nil, err
	}

	if shouldEnableMetrics() {
		event.Init()
	}

	return appInstance, nil
}

func shouldEnableMetrics() bool {
	if v, _ := strconv.ParseBool(os.Getenv("CRUSH_DISABLE_METRICS")); v {
		return false
	}
	if v, _ := strconv.ParseBool(os.Getenv("DO_NOT_TRACK")); v {
		return false
	}
	if config.Get().Options.DisableMetrics {
		return false
	}
	return true
}

func MaybePrependStdin(prompt string) (string, error) {
	if term.IsTerminal(os.Stdin.Fd()) {
		return prompt, nil
	}
	fi, err := os.Stdin.Stat()
	if err != nil {
		return prompt, err
	}
	if fi.Mode()&os.ModeNamedPipe == 0 {
		return prompt, nil
	}
	bts, err := io.ReadAll(os.Stdin)
	if err != nil {
		return prompt, err
	}
	return string(bts) + "\n\n" + prompt, nil
}

func ResolveCwd(cmd *cobra.Command) (string, error) {
	cwd, _ := cmd.Flags().GetString("cwd")
	if cwd != "" {
		err := os.Chdir(cwd)
		if err != nil {
			return "", fmt.Errorf("failed to change directory: %v", err)
		}
		return cwd, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %v", err)
	}
	return cwd, nil
}

func createDotCrushDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create data directory: %q %w", dir, err)
	}

	gitIgnorePath := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gitIgnorePath); os.IsNotExist(err) {
		if err := os.WriteFile(gitIgnorePath, []byte("*\n"), 0o644); err != nil {
			return fmt.Errorf("failed to create .gitignore file: %q %w", gitIgnorePath, err)
		}
	}

	return nil
}

// applyModelOverrides applies CLI flag overrides to the config
func applyModelOverrides(cmd *cobra.Command, cfg *config.Config) error {
	// If provider flags are set, inject a CLI-driven provider
	if err := injectCliProvider(cmd, cfg); err != nil {
		return err
	}

	// Check for simple --model flag first (sets both large and small)
	if model, _ := cmd.Flags().GetString("model"); model != "" {
		provider, modelName, err := parseModelFlag(model)
		if err != nil {
			return fmt.Errorf("invalid --model format: %w", err)
		}

		if cfg.Models == nil {
			cfg.Models = make(map[config.SelectedModelType]config.SelectedModel)
		}

		// Set both large and small to the same model
		cfg.Models[config.SelectedModelTypeLarge] = config.SelectedModel{
			Model:    modelName,
			Provider: provider,
		}
		cfg.Models[config.SelectedModelTypeSmall] = config.SelectedModel{
			Model:    modelName,
			Provider: provider,
		}

		// Add model to provider's model list if provider was CLI-injected
		if err := addModelToProvider(cfg, provider, modelName); err != nil {
			return err
		}

		slog.Debug("Setting model for agent", "provider", provider, "model", modelName)
		return nil
	}

	// Parse large model override
	if largeModel, _ := cmd.Flags().GetString("large-model"); largeModel != "" {
		provider, model, err := parseModelFlag(largeModel)
		if err != nil {
			return fmt.Errorf("invalid --large-model format: %w", err)
		}

		if cfg.Models == nil {
			cfg.Models = make(map[config.SelectedModelType]config.SelectedModel)
		}

		cfg.Models[config.SelectedModelTypeLarge] = config.SelectedModel{
			Model:    model,
			Provider: provider,
		}

		// Add model to provider's model list if provider was CLI-injected
		if err := addModelToProvider(cfg, provider, model); err != nil {
			return err
		}

		slog.Debug("Overriding large model", "provider", provider, "model", model)
	}

	// Parse small model override
	if smallModel, _ := cmd.Flags().GetString("small-model"); smallModel != "" {
		provider, model, err := parseModelFlag(smallModel)
		if err != nil {
			return fmt.Errorf("invalid --small-model format: %w", err)
		}

		if cfg.Models == nil {
			cfg.Models = make(map[config.SelectedModelType]config.SelectedModel)
		}

		cfg.Models[config.SelectedModelTypeSmall] = config.SelectedModel{
			Model:    model,
			Provider: provider,
		}

		// Add model to provider's model list if provider was CLI-injected
		if err := addModelToProvider(cfg, provider, model); err != nil {
			return err
		}

		slog.Debug("Overriding small model", "provider", provider, "model", model)
	}

	return nil
}

// injectCliProvider creates and injects a provider from CLI flags
func injectCliProvider(cmd *cobra.Command, cfg *config.Config) error {
	providerID, _ := cmd.Flags().GetString("provider-id")
	providerType, _ := cmd.Flags().GetString("provider-type")
	providerURL, _ := cmd.Flags().GetString("provider-url")
	providerAPIKey, _ := cmd.Flags().GetString("provider-api-key")

	// If any provider flag is set, all required flags must be set
	if providerID != "" || providerType != "" || providerURL != "" || providerAPIKey != "" {
		if providerID == "" || providerType == "" || providerURL == "" {
			return fmt.Errorf("when using provider flags, --provider-id, --provider-type, and --provider-url are required")
		}

		// Initialize providers map if needed
		if cfg.Providers == nil {
			cfg.Providers = csync.NewMap[string, config.ProviderConfig]()
		}

		// Create the provider config
		providerConfig := config.ProviderConfig{
			ID:      providerID,
			Type:    catwalk.Type(providerType),
			BaseURL: providerURL,
			APIKey:  providerAPIKey,
		}

		// Inject into config
		cfg.Providers.Set(providerID, providerConfig)
		slog.Debug("Injected CLI provider", "id", providerID, "type", providerType, "url", providerURL)
	}

	return nil
}

// parseModelFlag parses the provider/model format
// Example: "synthetic/hf:Qwen3-Coder-480B" -> ("synthetic", "hf:Qwen3-Coder-480B")
func parseModelFlag(modelFlag string) (provider, model string, err error) {
	parts := strings.SplitN(modelFlag, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected format 'provider/model', got: %s", modelFlag)
	}
	return parts[0], parts[1], nil
}

// addModelToProvider adds a model to the provider's model list
// This is needed when CLI-injecting providers to ensure GetModel() can find the model
func addModelToProvider(cfg *config.Config, providerID, modelID string) error {
	providerConfig, ok := cfg.Providers.Get(providerID)
	if !ok {
		// Provider doesn't exist, this is fine - model will be validated later
		return nil
	}

	// Check if model already exists in provider's list
	for _, m := range providerConfig.Models {
		if m.ID == modelID {
			return nil // Already exists
		}
	}

	// Add model to provider's model list
	// Use reasonable defaults for context window and max tokens
	providerConfig.Models = append(providerConfig.Models, catwalk.Model{
		ID:               modelID,
		Name:             modelID,
		ContextWindow:    128000, // Reasonable default
		DefaultMaxTokens: 4096,   // Reasonable default
	})

	cfg.Providers.Set(providerID, providerConfig)
	slog.Debug("Added model to provider", "provider", providerID, "model", modelID)
	return nil
}
