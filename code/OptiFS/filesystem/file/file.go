package file

import (
	"context"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fs"
)

// represents open files in the system, use for handling filehandles
// introducing mutex's means that synchronous events can happen with no worry of safety
type OptiFSFile struct {
	mu sync.Mutex

	// file descriptor for filehandling
	fdesc int
}

// Interfaces for Filehandles
var _ = (fs.FileHandle)((*OptiFSFile)(nil))
var _ = (fs.FileReader)((*OptiFSFile)(nil)) // reading a file

// makes a filehandle, to give more control over operations on files in the system
// abstract reference to files, where the state of the file (open, offsets, reading etc) can be tracked
func NewOptiFSFile(fdesc int) fs.FileHandle {
	return &OptiFSFile{fdesc: fdesc}
}

// handles read operations (implements concurrency)
func (f *OptiFSFile) Read(ctx context.Context, dest []byte, offset int64) (fuse.ReadResult, syscall.Errno) {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	f.mu.Lock()
	defer f.mu.Unlock()

	// read a specific amount of data (dest) from a specific point (offset) in the file
	read := fuse.ReadResultFd(uintptr(f.fdesc), offset, len(dest))

	return read, fs.OK
}
