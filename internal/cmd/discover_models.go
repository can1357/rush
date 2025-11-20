package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/can1357/rush/internal/config"
	"github.com/can1357/rush/internal/env"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/spf13/cobra"
)

var (
	discoverModelsAll  bool
	discoverModelsSave bool
)

var discoverModelsCmd = &cobra.Command{
	Use:   "discover-models [provider-id]",
	Short: "Discover available models from a provider",
	Long:  `Discover available models from an OpenAI-compatible provider's /v1/models endpoint.`,
	Example: `
# Discover models from a specific provider
rush discover-models litellm

# Discover models from all providers with discovery enabled
rush discover-models --all

# Discover and save models to config
rush discover-models litellm --save
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && !discoverModelsAll {
			return fmt.Errorf("provider ID required, or use --all to discover from all providers")
		}

		workingDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		cfg, err := config.Load(workingDir, "", false)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		resolver := config.NewShellVariableResolver(env.New())

		headerStyle := lipgloss.NewStyle().
			Foreground(charmtone.Butter).
			Background(charmtone.Guac).
			Bold(true).
			Padding(0, 1).
			Margin(1).
			MarginLeft(2)

		providerStyle := lipgloss.NewStyle().
			Foreground(charmtone.Butter).
			Bold(true)

		modelStyle := lipgloss.NewStyle().
			Foreground(charmtone.Malibu)

		metaStyle := lipgloss.NewStyle().
			Foreground(charmtone.Coral).
			Italic(true)

		if discoverModelsAll {
			// Discover from all providers
			discoveredAny := false
			for p := range cfg.Providers.Seq() {
				if !p.DiscoverModels {
					continue
				}

				fmt.Printf("\n%s\n", headerStyle.Copy().SetString("DISCOVERING").Render())
				fmt.Printf("  Provider: %s\n\n", providerStyle.Render(p.Name))

				models, err := config.DiscoverModelsFromProvider(p, resolver)
				if err != nil {
					slog.Warn("Failed to discover models", "provider", p.ID, "error", err)
					continue
				}

				if len(models) == 0 {
					fmt.Printf("  No models found.\n")
					continue
				}

				discoveredAny = true
				fmt.Printf("  Discovered %d models:\n\n", len(models))
				for _, m := range models {
					fmt.Printf("    • %s\n", modelStyle.Render(m.Name))
					fmt.Printf("      %s\n", metaStyle.Render(fmt.Sprintf("ID: %s | Context: %d | Max Tokens: %d",
						m.ID, m.ContextWindow, m.DefaultMaxTokens)))
				}
				fmt.Println()
			}

			if !discoveredAny {
				return fmt.Errorf("no providers have discovery enabled")
			}

			return nil
		}

		// Discover from specific provider
		providerID := args[0]
		providerConfig, ok := cfg.Providers.Get(providerID)
		if !ok {
			return fmt.Errorf("provider %q not found in configuration", providerID)
		}

		fmt.Printf("\n%s\n", headerStyle.Copy().SetString("DISCOVERING").Render())
		fmt.Printf("  Provider: %s\n", providerStyle.Render(providerConfig.Name))
		fmt.Printf("  Endpoint: %s\n\n", metaStyle.Render(providerConfig.BaseURL))

		models, err := config.DiscoverModelsFromProvider(providerConfig, resolver)
		if err != nil {
			return fmt.Errorf("failed to discover models: %w", err)
		}

		if len(models) == 0 {
			fmt.Printf("  No models found.\n\n")
			return nil
		}

		fmt.Printf("  Discovered %d models:\n\n", len(models))
		for _, m := range models {
			fmt.Printf("    • %s\n", modelStyle.Render(m.Name))
			fmt.Printf("      %s\n", metaStyle.Render(fmt.Sprintf("ID: %s | Context: %d | Max Tokens: %d",
				m.ID, m.ContextWindow, m.DefaultMaxTokens)))
		}
		fmt.Println()

		if discoverModelsSave {
			// Update provider config with discovered models
			providerConfig.Models = models
			cfg.Providers.Set(providerID, providerConfig)

			// Save to config file
			if err := cfg.SetConfigField("providers."+providerID+".models", models); err != nil {
				return fmt.Errorf("failed to save models to config: %w", err)
			}

			successStyle := headerStyle.Copy().SetString("SAVED")
			fmt.Printf("%s\n", successStyle.Render())
			fmt.Printf("  Models saved to configuration file.\n\n")
		}

		return nil
	},
}

func init() {
	discoverModelsCmd.Flags().BoolVar(&discoverModelsAll, "all", false, "Discover models from all providers with discovery enabled")
	discoverModelsCmd.Flags().BoolVar(&discoverModelsSave, "save", false, "Save discovered models to configuration file")
}
