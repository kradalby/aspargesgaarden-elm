//go:build linux

package images

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

const capsHelper = "ASPARGES_CAPS_HELPER"

// The unit gives the server CAP_NET_ADMIN as an ambient capability, which a
// child keeps across execve. A user namespace hands the test process the
// same set, unprivileged; it then starts a child both ways and each child
// reports its own sets.
func TestStartDropsAmbientCaps(t *testing.T) {
	switch os.Getenv(capsHelper) {
	case "report":
		status, err := os.ReadFile("/proc/self/status")
		if err != nil {
			os.Exit(2)
		}

		for line := range strings.Lines(string(status)) {
			if strings.HasPrefix(line, "CapPrm:") || strings.HasPrefix(line, "CapEff:") || strings.HasPrefix(line, "CapAmb:") {
				os.Stdout.WriteString(line)
			}
		}

		os.Exit(0)
	case "parent":
		var out bytes.Buffer

		for _, launch := range []func(*exec.Cmd) error{(*exec.Cmd).Start, start} {
			cmd := exec.Command(os.Args[0], "-test.run=^TestStartDropsAmbientCaps$")
			cmd.Env = []string{capsHelper + "=report"}
			cmd.Stdout = &out
			if err := launch(cmd); err != nil || cmd.Wait() != nil {
				os.Exit(2)
			}

			out.WriteString("--\n")
		}

		os.Stdout.Write(out.Bytes())
		os.Exit(0)
	}

	const nobody = 65534

	cmd := exec.Command(os.Args[0], "-test.run=^TestStartDropsAmbientCaps$")
	cmd.Env = []string{capsHelper + "=parent"}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: nobody, HostID: os.Getuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: nobody, HostID: os.Getgid(), Size: 1}},
		Credential:  &syscall.Credential{Uid: nobody, Gid: nobody},
		AmbientCaps: []uintptr{12}, // CAP_NET_ADMIN
	}

	out, err := cmd.Output()
	if err != nil && cmd.ProcessState == nil {
		t.Skipf("no unprivileged user namespace here: %v", err)
	}

	require.NoError(t, err)

	plain, dropped, ok := strings.Cut(string(out), "--\n")
	require.True(t, ok, out)

	// Proves the parent held the capability, or the check below says nothing.
	require.Contains(t, plain, "CapAmb:\t0000000000001000", plain)

	for _, set := range []string{"CapPrm", "CapEff", "CapAmb"} {
		require.Contains(t, dropped, set+":\t0000000000000000", dropped)
	}
}
