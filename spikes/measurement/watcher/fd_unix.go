//go:build unix

package watcher

import (
	"os"
	"syscall"
)

// CountFDs counts the file descriptors this process holds open.
//
// On Linux this reads /proc/self/fd. On macOS it falls back to /dev/fd,
// which is the same mechanism, so the same code runs on both.
func CountFDs() (int, error) {
	for _, dir := range []string{"/proc/self/fd", "/dev/fd"} {
		d, err := os.Open(dir)
		if err != nil {
			continue
		}
		names, err := d.Readdirnames(-1)
		d.Close()
		if err != nil {
			continue
		}
		return len(names), nil
	}
	return 0, os.ErrNotExist
}

// FDList returns the open file descriptors, with their targets.
func FDList() ([]string, error) {
	for _, dir := range []string{"/proc/self/fd", "/dev/fd"} {
		d, err := os.Open(dir)
		if err != nil {
			continue
		}
		names, _ := d.Readdirnames(-1)
		var out []string
		for _, n := range names {
			target, err := os.Readlink(dir + "/" + n)
			if err != nil {
				target = "?"
			}
			out = append(out, n+" -> "+target)
		}
		d.Close()
		return out, nil
	}
	return nil, os.ErrNotExist
}

// RLimitNoFile reports the process's open-file limit.
func RLimitNoFile() (uint64, error) {
	var r syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &r); err != nil {
		return 0, err
	}
	return r.Cur, nil
}

// Inode returns the inode number of a file, or 0.
func Inode(fi os.FileInfo) uint64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Ino)
	}
	return 0
}
