package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/forms"
)

// ServiceAccountCreateView allows creating new service accounts
type ServiceAccountCreateView struct {
	CreateViewBase

	iamClient *gcp.IAMClient
	projectID string
}

// NewServiceAccountCreateView creates a new service account create view
func NewServiceAccountCreateView(projectID string, iamClient *gcp.IAMClient) *ServiceAccountCreateView {
	v := &ServiceAccountCreateView{
		CreateViewBase: NewCreateViewBase("Creating service account..."),
		iamClient:      iamClient,
		projectID:      projectID,
	}

	v.buildForm()
	return v
}

func (v *ServiceAccountCreateView) buildForm() {
	v.Form = forms.NewForm("Create Service Account", forms.FormModeCreate).
		SetSubtitle(fmt.Sprintf("Project: %s", v.projectID)).
		EnableViewport()

	basicSection := forms.NewSection("basic", "Basic Settings").
		AddField(forms.NewTextField("account_id", "Account ID").
			SetRequired(true).
			SetPlaceholder("my-service-account").
			SetHelpText("6-30 characters, lowercase letters, numbers, and hyphens. Will become {id}@{project}.iam.gserviceaccount.com").
			SetValidator(forms.ComposeValidators(
				forms.ValidateRequired,
				forms.ValidateStringLength(6, 30),
				forms.ValidateGCPResourceName,
			))).
		AddField(forms.NewReadOnlyField("email_preview", "Email Preview",
			fmt.Sprintf("...@%s.iam.gserviceaccount.com", v.projectID))).
		AddField(forms.NewTextField("display_name", "Display Name").
			SetPlaceholder("My Service Account").
			SetHelpText("Optional friendly name, max 100 characters").
			SetCharLimit(100)).
		AddField(forms.NewTextAreaField("description", "Description").
			SetPlaceholder("Optional description for this service account").
			SetRows(3).
			SetHelpText("Max 256 characters"))

	v.Form.AddSection(basicSection)
}

// Update handles messages for the view
func (v *ServiceAccountCreateView) Update(msg tea.Msg) tea.Cmd {
	// Let base handle spinner ticks and cancel-during-saving
	if cmd, handled := v.HandleBaseUpdate(msg, ServiceAccountCreateCanceledMsg{}); handled {
		return cmd
	}

	switch msg.(type) {
	case forms.FormSubmitMsg:
		return v.handleSubmit()

	case forms.FormCancelMsg:
		return func() tea.Msg {
			return ServiceAccountCreateCanceledMsg{}
		}
	}

	// Update email preview when account ID changes
	if v.Form != nil {
		data := v.Form.GetData()
		if f := v.Form.GetField("email_preview"); f != nil {
			if accountID, ok := data["account_id"].(string); ok && accountID != "" {
				f.SetValue(fmt.Sprintf("%s@%s.iam.gserviceaccount.com", accountID, v.projectID))
			} else {
				f.SetValue(fmt.Sprintf("...@%s.iam.gserviceaccount.com", v.projectID))
			}
		}
	}

	return v.UpdateForm(msg)
}

func (v *ServiceAccountCreateView) handleSubmit() tea.Cmd {
	if errors := v.Form.Validate(); len(errors) > 0 {
		return nil
	}

	data := v.Form.GetData()

	accountID := ""
	if id, ok := data["account_id"].(string); ok {
		accountID = strings.TrimSpace(id)
	}
	displayName := ""
	if name, ok := data["display_name"].(string); ok {
		displayName = strings.TrimSpace(name)
	}
	description := ""
	if desc, ok := data["description"].(string); ok {
		description = strings.TrimSpace(desc)
	}

	cmd := v.BeginSaving()

	return tea.Batch(
		cmd,
		func() tea.Msg {
			return CreateServiceAccountMsg{
				ProjectID:   v.projectID,
				AccountID:   accountID,
				DisplayName: displayName,
				Description: description,
			}
		},
	)
}

// GetIAMClient returns the IAM client for reuse
func (v *ServiceAccountCreateView) GetIAMClient() *gcp.IAMClient {
	return v.iamClient
}
