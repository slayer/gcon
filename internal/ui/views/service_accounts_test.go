package views

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/confirm"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Static errors for linter compliance (err113)
var errPermissionDenied = errors.New("permission denied")
var errAPIError = errors.New("API error")
var errPreviousError = errors.New("previous error")

// --- Test data ---

var testServiceAccounts = []gcp.ServiceAccount{
	{
		Email:       "sa-one@test-project.iam.gserviceaccount.com",
		DisplayName: "Service Account One",
		UniqueID:    "111111111111111111111",
		Disabled:    false,
	},
	{
		Email:       "sa-two@test-project.iam.gserviceaccount.com",
		DisplayName: "Service Account Two",
		UniqueID:    "222222222222222222222",
		Disabled:    true,
	},
}

func newTestServiceAccountsView() *ServiceAccountsView {
	return NewServiceAccountsView("test-project")
}

// loadServiceAccountsInView simulates the full load sequence: client ready + accounts loaded.
func loadServiceAccountsInView(v *ServiceAccountsView, accounts []gcp.ServiceAccount) {
	v.Update(iamClientReadyMsg{client: &gcp.IAMClient{}})
	v.Update(serviceAccountsLoadedMsg{accounts: accounts})
}

// --- Constructor / initial state ---

func TestServiceAccountsView_New(t *testing.T) {
	v := newTestServiceAccountsView()

	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.True(t, v.loading, "View should start in loading state")
	assert.Nil(t, v.iamClient, "IAM client should not be set before Init")
	assert.Empty(t, v.accounts)
}

// --- Rendering ---

func TestServiceAccountsView_RenderLoading_NoClient(t *testing.T) {
	v := newTestServiceAccountsView()

	output := v.View()

	assert.Contains(t, output, "Initializing IAM client...")
}

func TestServiceAccountsView_RenderLoading_WithClient(t *testing.T) {
	v := newTestServiceAccountsView()
	// Client is set but still loading accounts
	v.Update(iamClientReadyMsg{client: &gcp.IAMClient{}})

	output := v.View()

	assert.Contains(t, output, "Loading service accounts...")
}

func TestServiceAccountsView_RenderError(t *testing.T) {
	v := newTestServiceAccountsView()
	v.Update(iamClientReadyMsg{client: &gcp.IAMClient{}})
	v.Update(serviceAccountsErrorMsg{err: errPermissionDenied})

	output := v.View()

	assert.Contains(t, output, "permission denied")
}

func TestServiceAccountsView_RenderEmpty(t *testing.T) {
	v := newTestServiceAccountsView()
	loadServiceAccountsInView(v, []gcp.ServiceAccount{})

	output := v.View()

	assert.Contains(t, output, "No service accounts found")
	assert.Contains(t, output, "Press 'c' to create one")
}

func TestServiceAccountsView_RenderLoaded(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 120, ContentHeight: 40, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	loadServiceAccountsInView(v, testServiceAccounts)

	output := v.View()

	// Table should show the account emails
	assert.Contains(t, output, "sa-one@test-project.iam.gserviceaccount.com")
	assert.Contains(t, output, "sa-two@test-project.iam.gserviceaccount.com")
}

func TestServiceAccountsView_HelpText(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 120, ContentHeight: 40, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	loadServiceAccountsInView(v, testServiceAccounts)

	output := v.View()

	assert.Contains(t, output, "enter: details")
	assert.Contains(t, output, "actions")
	assert.Contains(t, output, "create")
	assert.Contains(t, output, "toggle")
	assert.Contains(t, output, "delete")
	assert.Contains(t, output, "refresh")
}

// --- serviceAccountToRow ---

func TestServiceAccountToRow(t *testing.T) {
	tests := []struct {
		name     string
		account  gcp.ServiceAccount
		validate func(t *testing.T, row table.Row)
	}{
		{
			name: "enabled account",
			account: gcp.ServiceAccount{
				Email:       "sa@project.iam.gserviceaccount.com",
				DisplayName: "My SA",
				UniqueID:    "123456789",
				Disabled:    false,
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Equal(t, "sa@project.iam.gserviceaccount.com", row.Data[0])
				assert.Equal(t, "My SA", row.Data[1])
				assert.NotEmpty(t, row.Data[2], "Status should have an indicator")
				assert.Equal(t, "123456789", row.Data[3])
				assert.Equal(t, "sa@project.iam.gserviceaccount.com", row.ID)
			},
		},
		{
			name: "disabled account",
			account: gcp.ServiceAccount{
				Email:       "disabled@project.iam.gserviceaccount.com",
				DisplayName: "Disabled SA",
				UniqueID:    "987654321",
				Disabled:    true,
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Equal(t, "disabled@project.iam.gserviceaccount.com", row.Data[0])
				assert.Equal(t, "Disabled SA", row.Data[1])
				assert.NotEmpty(t, row.Data[2], "Status should have an indicator")
				assert.Equal(t, "987654321", row.Data[3])
				assert.Equal(t, "disabled@project.iam.gserviceaccount.com", row.ID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := serviceAccountToRow(tt.account)
			tt.validate(t, row)
		})
	}
}

func TestServiceAccountToRow_FilterValue(t *testing.T) {
	sa := gcp.ServiceAccount{
		Email:       "sa@project.iam.gserviceaccount.com",
		DisplayName: "My Service Account",
		UniqueID:    "123456789",
	}

	row := serviceAccountToRow(sa)

	assert.Contains(t, row.FilterValue, "sa@project.iam.gserviceaccount.com")
	assert.Contains(t, row.FilterValue, "My Service Account")
	assert.Contains(t, row.FilterValue, "123456789")
}

// --- findAccountByEmail ---

func TestServiceAccountsView_FindAccountByEmail(t *testing.T) {
	v := newTestServiceAccountsView()
	v.accounts = testServiceAccounts

	// Found
	sa, ok := v.findAccountByEmail("sa-two@test-project.iam.gserviceaccount.com")
	assert.True(t, ok)
	assert.Equal(t, "sa-two@test-project.iam.gserviceaccount.com", sa.Email)
	assert.True(t, sa.Disabled)

	// Not found
	_, ok = v.findAccountByEmail("nonexistent@example.com")
	assert.False(t, ok)
}

// --- Key bindings ---

func TestServiceAccountsView_RefreshKey(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	loadServiceAccountsInView(v, testServiceAccounts)

	cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	require.NotNil(t, cmd, "pressing 'r' should return a command for refresh")
	assert.True(t, v.loading, "view should be in loading state after refresh")
}

func TestServiceAccountsView_SelectKey(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	loadServiceAccountsInView(v, testServiceAccounts)

	cmd := v.Update(tea.KeyMsg{Type: tea.KeyEnter})

	require.NotNil(t, cmd, "pressing Enter should return a command")

	msg := cmd()
	selectedMsg, ok := msg.(ServiceAccountSelectedMsg)
	require.True(t, ok, "cmd should produce ServiceAccountSelectedMsg, got %T", msg)
	assert.Equal(t, testServiceAccounts[0].Email, selectedMsg.ServiceAccount.Email)
}

func TestServiceAccountsView_CreateKey(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	loadServiceAccountsInView(v, testServiceAccounts)

	cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	require.NotNil(t, cmd, "pressing 'c' should return a command")

	msg := cmd()
	createMsg, ok := msg.(ServiceAccountCreateRequestMsg)
	require.True(t, ok, "cmd should produce ServiceAccountCreateRequestMsg, got %T", msg)
	assert.Equal(t, "test-project", createMsg.ProjectID)
}

func TestServiceAccountsView_ToggleKey_EnableDisabled(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	// Load with only the disabled account so it's selected by default
	loadServiceAccountsInView(v, []gcp.ServiceAccount{testServiceAccounts[1]})

	cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})

	require.NotNil(t, cmd, "pressing 't' on a disabled account should return a command")

	msg := cmd()
	toggleMsg, ok := msg.(ToggleServiceAccountMsg)
	require.True(t, ok, "cmd should produce ToggleServiceAccountMsg, got %T", msg)
	assert.Equal(t, testServiceAccounts[1].Email, toggleMsg.Email)
	// Disabled account should get Disable=false (i.e., enable it)
	assert.False(t, toggleMsg.Disable, "toggling a disabled account should send Disable=false")
}

func TestServiceAccountsView_ToggleKey_DisableEnabled(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	// Load with only the enabled account
	loadServiceAccountsInView(v, []gcp.ServiceAccount{testServiceAccounts[0]})

	cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})

	require.NotNil(t, cmd, "pressing 't' on an enabled account should return a command")

	msg := cmd()
	toggleMsg, ok := msg.(ToggleServiceAccountMsg)
	require.True(t, ok, "cmd should produce ToggleServiceAccountMsg, got %T", msg)
	assert.Equal(t, testServiceAccounts[0].Email, toggleMsg.Email)
	// Enabled account should get Disable=true
	assert.True(t, toggleMsg.Disable, "toggling an enabled account should send Disable=true")
}

func TestServiceAccountsView_DeleteKey(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	loadServiceAccountsInView(v, testServiceAccounts)

	cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})

	// 'D' should open the delete confirmation dialog, not directly delete
	assert.True(t, v.showDeleteConfirm, "delete confirmation dialog should be shown")
	assert.NotNil(t, v.pendingDelete, "pendingDelete should be set")
	assert.Equal(t, testServiceAccounts[0].Email, v.pendingDelete.Email)
	// The cmd is for initializing the confirm dialog
	assert.NotNil(t, cmd)
}

func TestServiceAccountsView_KeysIgnoredWhileLoading(t *testing.T) {
	v := newTestServiceAccountsView()
	// Simulate client ready but still loading
	v.Update(iamClientReadyMsg{client: &gcp.IAMClient{}})
	// v.loading is still true

	cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	assert.Nil(t, cmd, "key presses should be ignored while loading")
}

// --- Action menu ---

func TestServiceAccountsView_ActionMenu_OpenClose(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	loadServiceAccountsInView(v, testServiceAccounts)

	// Open action menu
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.'}})
	assert.True(t, v.menuOpen, "action menu should be open after pressing '.'")
	assert.NotNil(t, v.actionMenu)
}

func TestServiceAccountsView_BuildActions_EnabledAccount(t *testing.T) {
	v := newTestServiceAccountsView()
	sa := gcp.ServiceAccount{
		Email:    "active@project.iam.gserviceaccount.com",
		Disabled: false,
	}

	actions := v.buildActions(sa)

	// Should have: Refresh, Create, Disable, Delete
	require.Len(t, actions, 4)
	assert.Equal(t, "Refresh", actions[0].Label)
	assert.Equal(t, "Create", actions[1].Label)
	assert.Equal(t, "Disable", actions[2].Label, "enabled account shows Disable")
	assert.Equal(t, "Delete", actions[3].Label)
	assert.True(t, actions[3].Dangerous, "Delete should be marked dangerous")
}

func TestServiceAccountsView_BuildActions_DisabledAccount(t *testing.T) {
	v := newTestServiceAccountsView()
	sa := gcp.ServiceAccount{
		Email:    "disabled@project.iam.gserviceaccount.com",
		Disabled: true,
	}

	actions := v.buildActions(sa)

	require.Len(t, actions, 4)
	assert.Equal(t, "Enable", actions[2].Label, "disabled account shows Enable")
}

func TestServiceAccountsView_ExecuteAction_Refresh(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	loadServiceAccountsInView(v, testServiceAccounts)

	cmd := v.executeAction('r')

	require.NotNil(t, cmd, "executeAction('r') should return a command")
	assert.True(t, v.loading, "view should be in loading state after refresh action")
}

func TestServiceAccountsView_ExecuteAction_Create(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	loadServiceAccountsInView(v, testServiceAccounts)

	cmd := v.executeAction('c')

	require.NotNil(t, cmd, "executeAction('c') should return a command")

	msg := cmd()
	createMsg, ok := msg.(ServiceAccountCreateRequestMsg)
	require.True(t, ok, "cmd should produce ServiceAccountCreateRequestMsg, got %T", msg)
	assert.Equal(t, "test-project", createMsg.ProjectID)
}

func TestServiceAccountsView_ExecuteAction_Toggle(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	loadServiceAccountsInView(v, testServiceAccounts)

	cmd := v.executeAction('t')

	require.NotNil(t, cmd, "executeAction('t') should return a command")

	msg := cmd()
	toggleMsg, ok := msg.(ToggleServiceAccountMsg)
	require.True(t, ok, "cmd should produce ToggleServiceAccountMsg, got %T", msg)
	// First account is enabled, so toggling should disable it
	assert.True(t, toggleMsg.Disable)
}

func TestServiceAccountsView_ExecuteAction_Delete(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	loadServiceAccountsInView(v, testServiceAccounts)

	cmd := v.executeAction('D')

	assert.True(t, v.showDeleteConfirm, "delete confirmation should be shown")
	assert.NotNil(t, v.pendingDelete)
	assert.NotNil(t, cmd)
}

// --- HasTextInputFocused / IsMenuOpen ---

func TestServiceAccountsView_HasTextInputFocused(t *testing.T) {
	v := newTestServiceAccountsView()

	// No filter or dialog active by default
	assert.False(t, v.HasTextInputFocused())
}

func TestServiceAccountsView_IsMenuOpen(t *testing.T) {
	v := newTestServiceAccountsView()

	assert.False(t, v.IsMenuOpen(), "menu should start closed")

	// Open action menu
	v.menuOpen = true
	assert.True(t, v.IsMenuOpen())

	// Reset and check delete confirm
	v.menuOpen = false
	v.showDeleteConfirm = true
	assert.True(t, v.IsMenuOpen())
}

// --- SetContext ---

func TestServiceAccountsView_SetContext(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 100, ContentHeight: 50, Tasks: make(map[string]context.Task)}

	v.SetContext(ctx)

	assert.Equal(t, ctx, v.ctx)
	assert.Equal(t, 100, v.width)
	assert.Equal(t, 50, v.height)
}

// --- GetIAMClient ---

func TestServiceAccountsView_GetIAMClient(t *testing.T) {
	v := newTestServiceAccountsView()

	// nil before Init
	assert.Nil(t, v.GetIAMClient())

	// Set after client ready message
	client := &gcp.IAMClient{}
	v.Update(iamClientReadyMsg{client: client})
	assert.Equal(t, client, v.GetIAMClient())
}

// --- Message handling ---

func TestServiceAccountsView_ClientReadyMsg(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)

	client := &gcp.IAMClient{}
	cmd := v.Update(iamClientReadyMsg{client: client})

	assert.Equal(t, client, v.iamClient)
	// Should return a command to load accounts
	assert.NotNil(t, cmd)
}

func TestServiceAccountsView_AccountsLoadedMsg(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)

	v.Update(iamClientReadyMsg{client: &gcp.IAMClient{}})
	v.Update(serviceAccountsLoadedMsg{accounts: testServiceAccounts})

	assert.False(t, v.loading)
	assert.Len(t, v.accounts, 2)
	assert.Nil(t, v.err)
}

func TestServiceAccountsView_ErrorMsg(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)

	v.Update(iamClientReadyMsg{client: &gcp.IAMClient{}})
	v.Update(serviceAccountsErrorMsg{err: errAPIError})

	assert.False(t, v.loading)
	assert.ErrorIs(t, v.err, errAPIError)
}

// --- Delete confirmation flow ---

func TestServiceAccountsView_DeleteConfirmFlow(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	loadServiceAccountsInView(v, testServiceAccounts)

	// Initiate delete
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	assert.True(t, v.showDeleteConfirm)
	assert.NotNil(t, v.pendingDelete)

	// Cancel the delete
	v.Update(confirm.TypeCancelMsg{})
	assert.False(t, v.showDeleteConfirm)
	assert.Nil(t, v.pendingDelete)
}

func TestServiceAccountsView_DeleteConfirmAccept(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	loadServiceAccountsInView(v, testServiceAccounts)

	// Initiate delete
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	assert.True(t, v.showDeleteConfirm)

	// Confirm the delete
	cmd := v.Update(confirm.TypeConfirmMsg{})
	assert.False(t, v.showDeleteConfirm)
	require.NotNil(t, cmd)

	msg := cmd()
	deleteMsg, ok := msg.(DeleteServiceAccountConfirmedMsg)
	require.True(t, ok, "cmd should produce DeleteServiceAccountConfirmedMsg, got %T", msg)
	assert.Equal(t, testServiceAccounts[0].Email, deleteMsg.Email)
}

func TestServiceAccountsView_RefreshClearsError(t *testing.T) {
	v := newTestServiceAccountsView()
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30, Tasks: make(map[string]context.Task)}
	v.SetContext(ctx)
	loadServiceAccountsInView(v, testServiceAccounts)

	// Set an error state
	v.err = errPreviousError
	v.loading = false

	// Press refresh
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	assert.Nil(t, v.err, "error should be cleared after refresh")
	assert.True(t, v.loading, "should be loading after refresh")
}
