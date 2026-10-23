package service

import (
	"errors"
	"os"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const manualProxyRolloutMarker = "/app/data/.manual-proxy-rollout-in-progress"

// A rolling release can temporarily contain older workers that do not know
// manual ownership. Edits become available after those workers have drained.
func validateManualProxyRelease(marker string) error {
	if _, err := os.Lstat(marker); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return infraerrors.Conflict("MANUAL_PROXY_RELEASE_IN_PROGRESS", "Manual IP settings are being updated. Please try again shortly.")
}
