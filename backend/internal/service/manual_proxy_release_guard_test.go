//go:build unit

package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManualProxyReleaseWaitsForOldWorkers(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "rollout")
	require.NoError(t, validateManualProxyRelease(marker))
	require.NoError(t, os.WriteFile(marker, []byte("older workers draining"), 0600))
	require.Error(t, validateManualProxyRelease(marker))
	require.NoError(t, os.Remove(marker))
	require.NoError(t, validateManualProxyRelease(marker))
	require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "missing"), marker))
	require.Error(t, validateManualProxyRelease(marker), "a broken marker must fail closed")
}
