// SPDX-License-Identifier: BSD-3-Clause
//go:build aix

package host

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
