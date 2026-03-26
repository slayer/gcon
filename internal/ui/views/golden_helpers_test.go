package views

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/context"
)

// Force TrueColor so ANSI output is deterministic across CI and local runs.
func init() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

const (
	goldenWidth  = 120
	goldenHeight = 40
)

// goldenContext returns a ProgramContext with standard golden test dimensions.
func goldenContext() *context.ProgramContext {
	ctx := context.New()
	ctx.SetDimensions(goldenWidth, goldenHeight, goldenWidth, goldenHeight)
	ctx.ProjectID = "test-project"
	return ctx
}

// sendKey simulates a rune key press and returns the resulting command.
func sendKey(view View, r rune) tea.Cmd {
	return view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
}

// sendSpecialKey simulates a special key press (Enter, Esc, Tab, etc.).
func sendSpecialKey(view View, k tea.KeyType) tea.Cmd {
	return view.Update(tea.KeyMsg{Type: k})
}

// testInstances returns a fixed set of Compute Engine instances for snapshot tests.
func testInstances() []gcp.Instance {
	return []gcp.Instance{
		{
			Name:        "web-server-1",
			Zone:        "us-central1-a",
			MachineType: "e2-medium",
			Status:      "RUNNING",
			InternalIP:  "10.128.0.2",
			ExternalIP:  "35.192.0.1",
			CreatedAt:   "2024-01-15T10:30:00.000-07:00",
		},
		{
			Name:        "db-server-1",
			Zone:        "us-central1-b",
			MachineType: "n2-standard-4",
			Status:      "RUNNING",
			InternalIP:  "10.128.0.3",
			ExternalIP:  "",
			CreatedAt:   "2024-02-20T14:00:00.000-07:00",
		},
		{
			Name:        "dev-instance",
			Zone:        "us-east1-b",
			MachineType: "e2-micro",
			Status:      "TERMINATED",
			InternalIP:  "10.142.0.5",
			ExternalIP:  "",
			CreatedAt:   "2024-03-10T08:15:00.000-07:00",
		},
		{
			Name:        "staging-api",
			Zone:        "europe-west1-b",
			MachineType: "e2-small",
			Status:      "SUSPENDED",
			InternalIP:  "10.132.0.10",
			ExternalIP:  "34.76.0.5",
			CreatedAt:   "2024-04-05T16:45:00.000-07:00",
		},
	}
}

// testProjects returns a fixed set of GCP projects for snapshot tests.
func testProjects() []gcp.Project {
	return []gcp.Project{
		{
			ID:     "my-project-prod",
			Name:   "My Project (Prod)",
			Number: 123456789,
			State:  "ACTIVE",
		},
		{
			ID:     "my-project-staging",
			Name:   "My Project (Staging)",
			Number: 987654321,
			State:  "ACTIVE",
		},
		{
			ID:     "old-project",
			Name:   "Old Project",
			Number: 555555555,
			State:  "DELETE_REQUESTED",
		},
	}
}
