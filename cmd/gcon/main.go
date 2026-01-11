package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/config"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui"
)

var projectFlag string

func init() {
	flag.StringVar(&projectFlag, "p", "", "GCP project ID (shorthand)")
	flag.StringVar(&projectFlag, "project", "", "GCP project ID")
}

func main() {
	flag.Parse()

	// Initialize GCP client using Application Default Credentials
	gcpClient, err := gcp.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize GCP client: %v\n", err)
		fmt.Fprintln(os.Stderr, "Make sure you're authenticated: gcloud auth application-default login")
		os.Exit(1)
	}
	defer gcpClient.Close()

	// Resolve project from flag/env/gcloud config
	projectID := config.ResolveProject(projectFlag)

	// Create and run the TUI application
	app := ui.NewApp(gcpClient, ui.AppOptions{
		InitialProjectID: projectID,
	})
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running application: %v\n", err)
		os.Exit(1)
	}
}
