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
	return stat.Bavail * uint64(stat.Bsize), nil
}
func reserveFile(f *os.File, size int64) error {
	store := unix.Fstore_t{Flags: unix.F_ALLOCATEALL, Posmode: unix.F_PEOFPOSMODE, Length: size}
	if err := unix.FcntlFstore(f.Fd(), unix.F_PREALLOCATE, &store); err != nil {
		return err
	}
	return f.Truncate(size)
}

func mapFile(f *os.File, offset int64, size int) ([]byte, error) {
	return unix.Mmap(int(f.Fd()), offset, size, unix.PROT_READ, unix.MAP_SHARED)
}
func unmapFile(data []byte) error { return unix.Munmap(data) }
