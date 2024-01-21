package file

import (
	"context"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// represents open files in the system, use for handling filehandles
// introducing mutex's means that synchronous events can happen with no worry of safety
type OptiFSFile struct {
	mu sync.Mutex

	// file descriptor for filehandling
	fdesc int
}

// statuses iused commonly throughout the system, to do with locks
const (
	_OFD_GETLK  = 36 //
	_OFD_SETLK  = 37
	_OFD_SETLKW = 38
)

// Interfaces for Filehandles
var _ = (fs.FileHandle)((*OptiFSFile)(nil))
var _ = (fs.FileReader)((*OptiFSFile)(nil))    // reading a file
var _ = (fs.FileGetattrer)((*OptiFSFile)(nil)) // get attrs of a file
var _ = (fs.FileReleaser)((*OptiFSFile)(nil))  // release (close) a file
var _ = (fs.FileGetlker)((*OptiFSFile)(nil))   // find conflicting locks for given lock

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

// get the attributes of a file/dir, using the filehandle
func (f *OptiFSFile) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	f.mu.Lock()
	defer f.mu.Unlock()

	s := syscall.Stat_t{}
	err := syscall.Fstat(f.fdesc, &s) // stat the file descriptor to get the attrs (no path needed)

	if err != nil {
		return fs.ToErrno(err)
	}

	out.FromStat(&s) // fill the attr into struct if no errors

	return fs.OK
}

// FUSE's version of a close
func (f *OptiFSFile) Release(ctx context.Context) syscall.Errno {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	f.mu.Lock()
	defer f.mu.Unlock()

	var err error

	// check to see if the file has already been released
	// -1 is the standard fdesc for closed/released files
	if f.fdesc != -1 {
		err = syscall.Close(f.fdesc)
	}

	if err != nil {
		return fs.ToErrno(err)
	}

	// notify that it's been released
	f.fdesc = -1

	return fs.OK
}

// gets the status' of locks on a file
func (f *OptiFSFile) Getlk(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32, out *fuse.FileLock) syscall.Errno {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	f.mu.Lock()
	defer f.mu.Unlock()

	lock := syscall.Flock_t{}
	lk.ToFlockT(&lock) // convert the FUSE file lock to a system file lock

	// OFD_GETLK associates the lock with the file descriptor, not the process itself
	// query the file lock status of the file descr
	err := fs.ToErrno(syscall.FcntlFlock(uintptr(f.fdesc), _OFD_GETLK, &lock))

	return err
}
