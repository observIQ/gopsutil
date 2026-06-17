// SPDX-License-Identifier: BSD-3-Clause
//go:build aix

package host

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockInvoker returns canned output for specific commands.
type mockInvoker struct {
	responses map[string]string
}

func (m *mockInvoker) Command(name string, arg ...string) ([]byte, error) {
	key := name + " " + strings.Join(arg, " ")
	key = strings.TrimSpace(key)
	if resp, ok := m.responses[key]; ok {
		return []byte(resp), nil
	}
	return nil, fmt.Errorf("unexpected command: %s", key)
}

func (m *mockInvoker) CommandWithContext(_ context.Context, name string, arg ...string) ([]byte, error) {
	return m.Command(name, arg...)
}

func withMockInvoker(t *testing.T, responses map[string]string) {
	t.Helper()
	old := testInvoker
	testInvoker = &mockInvoker{responses: responses}
	t.Cleanup(func() { testInvoker = old })
}

func TestBootTimeWithContext(t *testing.T) {
	// This is a wrapper function that delegates to common.BootTimeWithContext
	// Actual implementation testing is done in common_aix_test.go
	bootTime, err := BootTimeWithContext(context.TODO())
	require.NoError(t, err)
	assert.Positive(t, bootTime)
}

func TestUptimeWithContext(t *testing.T) {
	// This is a wrapper function that delegates to common.UptimeWithContext
	// Actual implementation testing is done in common_aix_test.go
	uptime, err := UptimeWithContext(context.TODO())
	require.NoError(t, err)
	assert.Positive(t, uptime)
}

// makeAIXUtmp builds a utmp record with the given fields; unset bytes stay
// zero, which the parser trims.
func makeAIXUtmp(user, line, host string, when int64, typ int16) aixUtmp {
	var e aixUtmp
	copy(e.User[:], user)
	copy(e.Line[:], line)
	copy(e.Host[:], host)
	e.Time = when
	e.Type = typ
	return e
}

func TestUsersWithContext(t *testing.T) {
	// Deterministic: write known utmp records to a temp file and verify parsing,
	// including that empty-user and non-USER_PROCESS records are skipped.
	path := filepath.Join(t.TempDir(), "utmp")
	f, err := os.Create(path)
	require.NoError(t, err)
	entries := []aixUtmp{
		makeAIXUtmp("alice", "pts/0", "10.0.0.1", 1700000000, user_PROCESS),
		makeAIXUtmp("", "pts/9", "", 1700000001, user_PROCESS),        // empty user: skipped
		makeAIXUtmp("ghost", "pts/8", "", 1699999999, user_PROCESS+1), // not USER_PROCESS: skipped
		makeAIXUtmp("bob", "pts/1", "host2", 1700000002, user_PROCESS),
	}
	for i := range entries {
		require.NoError(t, binary.Write(f, binary.BigEndian, &entries[i]))
	}
	require.NoError(t, f.Close())

	orig := utmpPath
	utmpPath = path
	defer func() { utmpPath = orig }()

	users, err := UsersWithContext(context.TODO())
	require.NoError(t, err)
	require.Len(t, users, 2, "empty-user and non-USER_PROCESS records should be skipped")

	assert.Equal(t, "alice", users[0].User)
	assert.Equal(t, "pts/0", users[0].Terminal)
	assert.Equal(t, "10.0.0.1", users[0].Host)
	assert.Equal(t, 1700000000, users[0].Started)

	assert.Equal(t, "bob", users[1].User)
	assert.Equal(t, "pts/1", users[1].Terminal)
	assert.Equal(t, "host2", users[1].Host)
	assert.Equal(t, 1700000002, users[1].Started)
}

func TestHostIDWithContext(t *testing.T) {
	withMockInvoker(t, map[string]string{
		"uname -u": "IBM,0221D80FV\n",
	})

	id, err := HostIDWithContext(context.TODO())
	require.NoError(t, err)
	assert.Equal(t, "IBM,0221D80FV", id)
}

func TestPlatformInformationWithContext(t *testing.T) {
	withMockInvoker(t, map[string]string{
		"uname -s": "AIX\n",
		"oslevel":  "7.3.0.0\n",
	})

	platform, family, version, err := PlatformInformationWithContext(context.TODO())
	require.NoError(t, err)
	assert.Equal(t, "AIX", platform)
	assert.Equal(t, "AIX", family)
	assert.Equal(t, "7.3.0.0", version)
}

func TestKernelVersionWithContext(t *testing.T) {
	withMockInvoker(t, map[string]string{
		"oslevel -s": "7300-03-00-2446\n",
	})

	version, err := KernelVersionWithContext(context.TODO())
	require.NoError(t, err)
	assert.Equal(t, "7300-03-00-2446", version)
}

func TestKernelArch(t *testing.T) {
	withMockInvoker(t, map[string]string{
		"bootinfo -y": "64\n",
	})

	arch, err := KernelArch()
	require.NoError(t, err)
	assert.Equal(t, "64", arch)
}

func TestVirtualizationWithContext(t *testing.T) {
	system, role, err := VirtualizationWithContext(context.TODO())
	require.NoError(t, err)
	// On a real AIX system, we expect either powervm or wpar
	if system != "" {
		assert.Contains(t, []string{"powervm", "wpar"}, system)
		assert.Equal(t, "guest", role)
	}
}

func TestVirtualizationWithContext_LPAR(t *testing.T) {
	withMockInvoker(t, map[string]string{
		"uname -W": "0\n",
		"uname -L": "25 soaix422\n",
	})

	system, role, err := VirtualizationWithContext(context.TODO())
	require.NoError(t, err)
	assert.Equal(t, "powervm", system)
	assert.Equal(t, "guest", role)
}

func TestVirtualizationWithContext_WPAR(t *testing.T) {
	withMockInvoker(t, map[string]string{
		"uname -W": "2\n",
		"uname -L": "25 soaix422\n",
	})

	system, role, err := VirtualizationWithContext(context.TODO())
	require.NoError(t, err)
	assert.Equal(t, "wpar", system)
	assert.Equal(t, "guest", role)
}

func TestVirtualizationWithContext_BareMetal(t *testing.T) {
	withMockInvoker(t, map[string]string{
		"uname -W": "0\n",
		"uname -L": "-1 NULL\n",
	})

	system, role, err := VirtualizationWithContext(context.TODO())
	require.NoError(t, err)
	assert.Empty(t, system)
	assert.Empty(t, role)
}
