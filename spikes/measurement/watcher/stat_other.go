//go:build !unix

package watcher

import "os"

// CountFDs is not measurable off Unix.
func CountFDs() (int, error) { return 0, os.ErrNotExist }

// FDList is not measurable off Unix.
func FDList() ([]string, error) { return nil, os.ErrNotExist }

// RLimitNoFile is not measurable off Unix.
func RLimitNoFile() (uint64, error) { return 0, os.ErrNotExist }

// Inode returns 0 off Unix.
func Inode(fi os.FileInfo) uint64 { return 0 }
