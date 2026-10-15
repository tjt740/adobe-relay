//go:build !linux && !darwin

package imagebuffer

import "os"

func availableDisk(*os.File) (uint64, error)       { return 0, ErrCapacity }
func reserveFile(*os.File, int64) error            { return ErrCapacity }
func mapFile(*os.File, int64, int) ([]byte, error) { return nil, ErrCapacity }
func unmapFile([]byte) error                       { return nil }
