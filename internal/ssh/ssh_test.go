package ssh

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildGcloudArgs(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want []string
	}{
		{
			name: "minimal",
			opts: Options{
				Method:   MethodGcloud,
				Project:  "my-proj",
				Zone:     "us-central1-a",
				Instance: "my-vm",
			},
			want: []string{
				"compute", "ssh", "my-vm",
				"--project=my-proj",
				"--zone=us-central1-a",
			},
		},
		{
			name: "iap and internal-ip",
			opts: Options{
				Method:     MethodGcloud,
				Project:    "p",
				Zone:       "z",
				Instance:   "vm",
				IAPTunnel:  true,
				InternalIP: true,
			},
			want: []string{
				"compute", "ssh", "vm",
				"--project=p", "--zone=z",
				"--tunnel-through-iap", "--internal-ip",
			},
		},
		{
			name: "user override is split into two --ssh-flag args",
			opts: Options{
				Method:   MethodGcloud,
				Project:  "p", Zone: "z", Instance: "vm",
				User: "alice",
			},
			want: []string{
				"compute", "ssh", "vm",
				"--project=p", "--zone=z",
				"--ssh-flag=-l", "--ssh-flag=alice",
			},
		},
		{
			name: "port forward",
			opts: Options{
				Method:      MethodGcloud,
				Project:     "p", Zone: "z", Instance: "vm",
				PortForward: "5432:localhost:5432",
			},
			want: []string{
				"compute", "ssh", "vm",
				"--project=p", "--zone=z",
				"--ssh-flag=-L", "--ssh-flag=5432:localhost:5432",
			},
		},
		{
			name: "all gcloud flags combined",
			opts: Options{
				Method:      MethodGcloud,
				Project:     "p", Zone: "z", Instance: "vm",
				User:        "bob",
				IAPTunnel:   true,
				InternalIP:  true,
				PortForward: "8080:localhost:80",
			},
			want: []string{
				"compute", "ssh", "vm",
				"--project=p", "--zone=z",
				"--tunnel-through-iap", "--internal-ip",
				"--ssh-flag=-l", "--ssh-flag=bob",
				"--ssh-flag=-L", "--ssh-flag=8080:localhost:80",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, BuildGcloudArgs(tc.opts))
		})
	}
}

func TestBuildSSHArgs(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want []string
	}{
		{
			name: "bare host",
			opts: Options{Method: MethodSSH, Host: "10.0.0.5"},
			want: []string{"10.0.0.5"},
		},
		{
			name: "user @ host",
			opts: Options{Method: MethodSSH, Host: "1.2.3.4", User: "alice"},
			want: []string{"alice@1.2.3.4"},
		},
		{
			name: "port forward",
			opts: Options{
				Method: MethodSSH, Host: "h", User: "u",
				PortForward: "5432:localhost:5432",
			},
			want: []string{"-L", "5432:localhost:5432", "u@h"},
		},
		{
			name: "internalIP is ignored in ssh mode",
			opts: Options{
				Method: MethodSSH, Host: "10.0.0.1", InternalIP: true,
			},
			want: []string{"10.0.0.1"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, BuildSSHArgs(tc.opts))
		})
	}
}

func TestBuildArgs_NoShellInjection(t *testing.T) {
	opts := Options{
		Method:   MethodGcloud,
		Project:  "p", Zone: "z", Instance: "vm",
		User: "alice; rm -rf /",
	}
	args := BuildGcloudArgs(opts)
	found := 0
	for _, a := range args {
		if a == "--ssh-flag=alice; rm -rf /" {
			found++
		}
	}
	assert.Equal(t, 1, found, "user value must arrive as one argv element")
	for _, a := range args {
		assert.False(t, strings.Contains(a, "rm -rf") && a != "--ssh-flag=alice; rm -rf /",
			"unexpected fragment leaked: %q", a)
	}
}

func TestLookupBinary(t *testing.T) {
	path, ok := LookupBinary("sh")
	assert.True(t, ok)
	assert.NotEmpty(t, path)

	_, ok = LookupBinary("definitely-not-a-real-binary-xyzzy")
	assert.False(t, ok)
}
