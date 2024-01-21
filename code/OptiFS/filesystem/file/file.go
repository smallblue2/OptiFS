package file

import (
	"context"
	"sync"
	"syscall"
	"unsafe"

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

// Interfaces for Filehandles
var _ = (fs.FileHandle)((*OptiFSFile)(nil))
var _ = (fs.FileReader)((*OptiFSFile)(nil)) // reading a file
var _ = (fs.FileFsyncer)((*OptiFSFile)(nil)) // Ensuring things are written to disk
var _ = (fs.FileFlusher)((*OptiFSFile)(nil)) // Flushes the file
var _ = (fs.FileSetattrer)((*OptiFSFile)(nil)) // Writes attributes to the file
var _ = (fs.FileWriter)((*OptiFSFile)(nil)) // For performing write operations

// makes a filehandle, to give more control over operations on files in the system
// abstract reference to files, where the state of the file (open, offsets, reading etc) 
// can be tracked
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
    // Use the FUSE library's built-in
	read := fuse.ReadResultFd(uintptr(f.fdesc), offset, len(dest))

	return read, fs.OK
}

func (f *OptiFSFile) Fsync(ctx context.Context, flags uint32) syscall.Errno {
    // Gain access to the mutex lock
    f.mu.Lock()
    defer f.mu.Unlock()

    // Perform an Fsync on the actual file in the underlying filesystem
    return fs.ToErrno(syscall.Fsync(f.fdesc))
}

func (f *OptiFSFile) Flush(ctx context.Context) syscall.Errno {
    // Gain access to the mutex lock
    f.mu.Lock()
    defer f.mu.Unlock()

    // In order to force FUSE to flush, we will dup the filedescriptor and then close it
	tmpfd, err := syscall.Dup(f.fdesc)

	if err != nil {
		return fs.ToErrno(err)
	}
	return fs.ToErrno(syscall.Close(tmpfd))
}

func (f *OptiFSFile) Setattr(ctx context.Context, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
    f.mu.Lock()
    defer f.mu.Unlock()

    // Check to see if we need to change the mode
    if mode, ok := in.GetMode(); ok {
        // If so, change the mode
        if err := syscall.Fchmod(f.fdesc, mode); err != nil {
            return fs.ToErrno(err)
        }
    }

    // Check to see if we need to change the UID or GID
    uid, uok := in.GetUID()
    gid, gok := in.GetGID()
    // If we have a UID or GID to set
    if uok || gok {
		// Set their default values to -1
		// -1 indicates that the respective value shouldn't change
		safeUID, safeGID := -1, -1
		if uok {
			safeUID = int(uid)
		}
		if gok {
			safeGID = int(gid)
		}
		// Chown these values
		err := syscall.Fchown(f.fdesc, safeUID, safeGID)
		if err != nil {
			return fs.ToErrno(err)
		}
	}


	// Same thing for modification and access times
	mtime, mok := in.GetMTime()
	atime, aok := in.GetATime()

	if mok || aok {
		// Initialize pointers to the time values
		ap := &atime
		mp := &mtime
		// Take into account if access or mod times are not both provided
		if !aok {
			ap = nil
		}
		if !mok {
			mp = nil
		}

		// Create an array to hold timespec values for syscall
		// This is a data structure that represents a time value
		// with precision up to nanoseconds
		var times [2]syscall.Timespec
		times[0] = fuse.UtimeToTimespec(ap)
		times[1] = fuse.UtimeToTimespec(mp)

        // BELOW LINE IS FROM `fs` package, hanwen - TODO: REFERENCE PROPERLY
        _, _, err := syscall.Syscall6(syscall.SYS_UTIMENSAT, uintptr(f.fdesc), 0, uintptr(unsafe.Pointer(&times)), uintptr(0), 0, 0)
        err = syscall.Errno(err)
        if err != 0 {
		    return fs.ToErrno(err)
		}
	}

    // Check to see if we need to change the size
	if sz, ok := in.GetSize(); ok {
        // Change the size
        if err := fs.ToErrno(syscall.Ftruncate(f.fdesc, int64(sz))); err != 0 {
			return err
		}
	}

    return fs.OK
}

func (f *OptiFSFile) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	f.mu.Lock()
	defer f.mu.Unlock()
    // pwrite writes to a filedescriptor from a given offset
	numOfBytesWritten, err := syscall.Pwrite(f.fdesc, data, off)
	return uint32(numOfBytesWritten), fs.ToErrno(err)
}
