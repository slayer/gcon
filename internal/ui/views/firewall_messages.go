package views

import "github.com/slayer/gcon/internal/gcp"

// FirewallSelectedMsg is sent when a firewall rule is selected from the list
type FirewallSelectedMsg struct {
	Firewall gcp.FirewallRule
}

// DeleteFirewallConfirmedMsg is sent when user confirms firewall rule deletion
type DeleteFirewallConfirmedMsg struct {
	RuleName string
}

// ToggleFirewallMsg is sent when user requests enable/disable toggle
type ToggleFirewallMsg struct {
	RuleName string
	Disable  bool // true = disable the rule, false = enable it
}

// FirewallActionResultMsg reports the result of an async firewall operation
type FirewallActionResultMsg struct {
	Action  string // "delete", "enable", "disable"
	Success bool
	Error   error
}
