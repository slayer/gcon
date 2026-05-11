// Package ssh builds argument vectors for invoking either `gcloud compute ssh`
// or the system `ssh` binary, given a set of user-chosen Options.
//
// The package is intentionally UI-free: it has no Bubble Tea dependencies and
// is fully unit-testable.
package ssh

import "os/exec"

// Method selects which binary the caller will launch.
type Method int

const (
	MethodGcloud Method = iota
	MethodSSH
)

// Options bundles every choice the SSH dialog can produce.
type Options struct {
	Method   Method
	Project  string // gcloud only
	Zone     string // gcloud only
	Instance string // gcloud only — used as the SSH target

	Host string // ssh only — the destination address
	User string // optional, both modes

	IAPTunnel   bool   // gcloud only
	InternalIP  bool   // gcloud only — ssh mode encodes the IP into Host instead
	PortForward string // optional, both modes — "L:H:R"
}

// BuildGcloudArgs returns argv to pass to `exec.Command("gcloud", args...)`.
//
// Each --ssh-flag value becomes its own argv element. gcloud forwards each
// element to ssh as a single token, so "-l USER" must be split as
// "--ssh-flag=-l --ssh-flag=USER" rather than joined-and-quoted.
func BuildGcloudArgs(opts Options) []string {
	args := []string{
		"compute", "ssh", opts.Instance,
		"--project=" + opts.Project,
		"--zone=" + opts.Zone,
	}
	if opts.IAPTunnel {
		args = append(args, "--tunnel-through-iap")
	}
	if opts.InternalIP {
		args = append(args, "--internal-ip")
	}
	if opts.User != "" {
		args = append(args, "--ssh-flag=-l", "--ssh-flag="+opts.User)
	}
	if opts.PortForward != "" {
		args = append(args, "--ssh-flag=-L", "--ssh-flag="+opts.PortForward)
	}
	return args
}

// BuildSSHArgs returns argv to pass to `exec.Command("ssh", args...)`.
//
// opts.InternalIP is intentionally ignored here: in ssh mode the dialog has
// already encoded the desired address into opts.Host.
func BuildSSHArgs(opts Options) []string {
	target := opts.Host
	if opts.User != "" {
		target = opts.User + "@" + opts.Host
	}
	args := []string{}
	if opts.PortForward != "" {
		args = append(args, "-L", opts.PortForward)
	}
	args = append(args, target)
	return args
}

// LookupBinary returns the absolute path to `name` if found on $PATH.
func LookupBinary(name string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return path, true
}
