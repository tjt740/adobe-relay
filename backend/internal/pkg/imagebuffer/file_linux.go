package imagebuffer

import (
	"golang.org/x/sys/unix"
	"os"
)

func availableDisk(f *os.File) (uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Fstatfs(int(f.Fd()), &stat); err != nil {
		return 0, err
	}
	if stat.Type == unix.TMPFS_MAGIC || stat.Type == unix.RAMFS_MAGIC {
		return 0, ErrCapacity
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}
func reserveFile(f *os.File, size int64) error {
	return unix.Fallocate(int(f.Fd()), 0, 0, size)
}

func mapFile(f *os.File, offset int64, size int) ([]byte, error) {
	return unix.Mmap(int(f.Fd()), offset, size, unix.PROT_READ, unix.MAP_SHARED)
}
func unmapFile(data []byte) error { return unix.Munmap(data) }
