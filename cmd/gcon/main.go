package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/config"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui"
	"github.com/slayer/gcon/internal/ui/symbols"
)

var (
	projectFlag  string
	noEmojisFlag bool
)

func init() {
	flag.StringVar(&projectFlag, "p", "", "GCP project ID (shorthand)")
	flag.StringVar(&projectFlag, "project", "", "GCP project ID")
	flag.BoolVar(&noEmojisFlag, "no-emojis", false, "Use ASCII characters instead of emojis")
	flag.BoolVar(&noEmojisFlag, "ascii", false, "Use ASCII characters instead of emojis (alias)")
}

func main() {
	flag.Parse()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Enable ASCII mode if requested (disables emojis)
	if noEmojisFlag {
		symbols.SetASCIIMode(true)
	}

	// Initialize GCP client using Application Default Credentials
	gcpClient, err := gcp.NewClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Make sure you're authenticated: gcloud auth application-default login")
		return fmt.Errorf("failed to initialize GCP client: %w", err)
	}
	defer func() { _ = gcpClient.Close() }()

	// Resolve project from flag/env/gcloud config
	projectID := config.ResolveProject(projectFlag)

	// Create and run the TUI application
	app := ui.NewApp(gcpClient, ui.AppOptions{
		InitialProjectID: projectID,
	})
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running application: %w", err)
	}
	return nil
}
