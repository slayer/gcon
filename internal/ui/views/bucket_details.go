package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/gcp/usage"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// bucketDetailsKeyMap defines key bindings for BucketDetailsView.
type bucketDetailsKeyMap struct {
	DeepScan key.Binding
	Refresh  key.Binding
	NextTab  key.Binding
	PrevTab  key.Binding
	Tab1     key.Binding
	Tab2     key.Binding
}

func defaultBucketDetailsKeyMap() bucketDetailsKeyMap {
	return bucketDetailsKeyMap{
		DeepScan: key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "calculate usage")),
		Refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh monitoring")),
		NextTab:  key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "next tab")),
		PrevTab:  key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "prev tab")),
		Tab1:     key.NewBinding(key.WithKeys("1")),
		Tab2:     key.NewBinding(key.WithKeys("2")),
	}
}

// BucketDetailsView shows static bucket metadata plus a Usage tab driven by
// the usage.Scanner. The Usage tab displays monitoring totals immediately and
// supports running an on-demand deep scan ('C') for breakdowns by storage
// class, top-level prefix, and file extension.
type BucketDetailsView struct {
	bucket  gcp.Bucket
	ctx     *context.ProgramContext
	tabs    *tabs.Tabs // tabs.New returns *Tabs (not *Model)
	keys    bucketDetailsKeyMap
	spinner spinner.Model

	// usage holds the most recent BucketUsage for this bucket (any source).
	usage *usage.BucketUsage
	// scanInProgress is true between StartDeepScan and ReadyMsg.
	scanInProgress bool
	scanErr        error
}

// NewBucketDetailsView constructs the view. Caller must call Init() afterwards.
func NewBucketDetailsView(bucket gcp.Bucket) *BucketDetailsView {
	t := tabs.New([]tabs.Tab{
		{ID: "details", Label: "Details"},
		{ID: "usage", Label: "Usage"},
	})
	return &BucketDetailsView{
		bucket:  bucket,
		tabs:    t,
		keys:    defaultBucketDetailsKeyMap(),
		spinner: components.NewGCPSpinner(),
	}
}

// Init kicks off a monitoring fetch so the Usage tab has at least the totals.
func (v *BucketDetailsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		func() tea.Msg {
			return UsageMonitoringRequestMsg{Bucket: v.bucket.Name}
		},
	)
}

// SetSize delegates dimensions to inner components.
func (v *BucketDetailsView) SetSize(width, _ int) {
	if v.tabs != nil {
		v.tabs.SetSize(width)
	}
}

// SetContext stores the shared context.
func (v *BucketDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	if ctx != nil {
		v.SetSize(ctx.ContentWidth, ctx.ContentHeight)
	}
}

// HasTextInputFocused returns false; this view has no text inputs.
func (v *BucketDetailsView) HasTextInputFocused() bool { return false }

// IsMenuOpen returns false; no action menu in v1.
func (v *BucketDetailsView) IsMenuOpen() bool { return false }

// View renders the active tab.
func (v *BucketDetailsView) View() string {
	if v.ctx == nil {
		return ""
	}
	header := v.tabs.View()
	body := ""
	switch v.tabs.ActiveTab().ID {
	case "details":
		body = v.renderDetailsTab()
	case "usage":
		body = v.renderUsageTab()
	}
	return header + "\n" + body
}

// Update handles messages.
func (v *BucketDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case usage.ReadyMsg:
		if msg.Err == nil && msg.Usage.Bucket != v.bucket.Name {
			return nil // not for us
		}
		if msg.Err != nil {
			v.scanErr = msg.Err
			v.scanInProgress = false
			return nil
		}
		u := msg.Usage
		v.usage = &u
		v.scanInProgress = false
		return nil

	case usage.ProgressMsg:
		if msg.Bucket != v.bucket.Name {
			return nil
		}
		v.scanInProgress = true
		// Show running tally in the Usage tab.
		v.usage = &usage.BucketUsage{
			Bucket:      msg.Bucket,
			Prefix:      msg.Prefix,
			TotalBytes:  msg.BytesScanned,
			ObjectCount: msg.ObjectsScanned,
			Source:      usage.SourceDeepScan,
		}
		return nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		v.spinner, cmd = v.spinner.Update(msg)
		return cmd

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, v.keys.NextTab):
			n := v.tabs.Count()
			if n > 0 {
				v.tabs.SetActive((v.tabs.ActiveIndex() + 1) % n)
			}
			return nil
		case key.Matches(msg, v.keys.PrevTab):
			n := v.tabs.Count()
			if n > 0 {
				v.tabs.SetActive((v.tabs.ActiveIndex() - 1 + n) % n)
			}
			return nil
		case key.Matches(msg, v.keys.Tab1):
			v.tabs.SetActive(0)
			return nil
		case key.Matches(msg, v.keys.Tab2):
			v.tabs.SetActive(1)
			return nil
		case key.Matches(msg, v.keys.DeepScan):
			if v.tabs.ActiveTab().ID != "usage" {
				return nil
			}
			v.scanInProgress = true
			v.scanErr = nil
			return func() tea.Msg {
				return UsageDeepScanRequestMsg{Bucket: v.bucket.Name, Prefix: ""}
			}
		case key.Matches(msg, v.keys.Refresh):
			return func() tea.Msg {
				return UsageMonitoringRequestMsg{Bucket: v.bucket.Name}
			}
		}
	}
	return nil
}

// renderDetailsTab shows static bucket metadata.
func (v *BucketDetailsView) renderDetailsTab() string {
	labelStyle := lipgloss.NewStyle().Foreground(v.ctx.Styles.Colors.Muted).Width(15)
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Name:") + " " + v.bucket.Name + "\n")
	b.WriteString(labelStyle.Render("Location:") + " " + v.bucket.Location + "\n")
	b.WriteString(labelStyle.Render("Storage Class:") + " " + v.bucket.StorageClass + "\n")
	b.WriteString(labelStyle.Render("Created:") + " " + timeutil.FormatDate(v.bucket.Created) + "\n")
	b.WriteString("\n")
	linkStyle := lipgloss.NewStyle().Foreground(v.ctx.Styles.Colors.Primary).Underline(true)
	b.WriteString(linkStyle.Render("Press Enter to browse objects") + "\n")
	return b.String()
}

// renderUsageTab shows totals and (when available) breakdowns.
func (v *BucketDetailsView) renderUsageTab() string {
	var b strings.Builder
	b.WriteString("\n")
	if v.scanErr != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		b.WriteString(errStyle.Render("  Scan error: "+v.scanErr.Error()) + "\n\n")
	}
	if v.usage == nil {
		b.WriteString("  Loading monitoring metrics...\n")
		b.WriteString("\n  Press 'C' to run a deep scan.\n")
		return b.String()
	}
	u := *v.usage
	muted := lipgloss.NewStyle().Foreground(v.ctx.Styles.Colors.Muted)
	b.WriteString(fmt.Sprintf("  Total size:    %s\n", gcp.FormatSize(u.TotalBytes)))
	b.WriteString(fmt.Sprintf("  Object count:  %s\n\n", formatObjectCount(u.ObjectCount)))
	switch u.Source {
	case usage.SourceMonitoring:
		hint := "Source: Monitoring"
		if !u.ScannedAt.IsZero() {
			hint += " - as of " + timeutil.FormatDateTime(u.ScannedAt)
		}
		b.WriteString("  " + muted.Render(hint) + "\n")
		b.WriteString("\n  Press 'C' to run a deep scan for breakdowns.\n")
	case usage.SourceDeepScan:
		if v.scanInProgress {
			b.WriteString("  " + muted.Render("Scan in progress ("+v.spinner.View()+")...") + "\n")
		} else {
			b.WriteString("  " + muted.Render("Source: Deep scan") + "\n")
		}
		b.WriteString("\n")
		writeStatTable(&b, "By Storage Class", u.ByStorageClass, 0)
		writeStatTable(&b, "By Top-Level Prefix", u.ByTopPrefix, 0)
		writeStatTable(&b, "By Extension (top 20 by size)", u.ByExtension, 20)
	}
	return b.String()
}

// writeStatTable renders one breakdown section (sorted by Bytes desc).
// limit > 0 caps the number of rows shown.
func writeStatTable(b *strings.Builder, title string, m map[string]usage.Stat, limit int) {
	if len(m) == 0 {
		return
	}
	type kv struct {
		k string
		v usage.Stat
	}
	all := make([]kv, 0, len(m))
	for k, v := range m {
		all = append(all, kv{k, v})
	}
	// Sort descending by Bytes (simple selection sort; lists are small).
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].v.Bytes > all[i].v.Bytes {
				all[i], all[j] = all[j], all[i]
			}
		}
	}
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	b.WriteString("  --- " + title + " ---\n")
	for _, e := range all {
		b.WriteString(fmt.Sprintf("    %-30s %12s  %12s\n",
			e.k, gcp.FormatSize(e.v.Bytes), formatObjectCount(e.v.Count)))
	}
	b.WriteString("\n")
}
