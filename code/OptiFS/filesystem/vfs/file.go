//please wokr please work im begging please please pelase pelase A FileHandle represents an open file or directory, acting as
// an abstraction for an open file descriptor.
package vfs

import (
	"context"
	"filesystem/metadata"
	"filesystem/permissions"

	"log"
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

	// store the hash of the file content
	currentHash [64]byte

    // Reference number in our memory
    refNum uint64

	// Stable attributes of the file
	attr fs.StableAttr

    // The flags of the file (OAPPEND, RWONLY, etc...)
    flags uint32
}

// statuses used commonly throughout the system, to do with locks
const (
	_OFD_GETLK  = 36
	_OFD_SETLK  = 37
	_OFD_SETLKW = 38
)

// Interfaces for Filehandles
var _ = (fs.FileHandle)((*OptiFSFile)(nil))
var _ = (fs.FileReader)((*OptiFSFile)(nil))    // reading a file
var _ = (fs.FileFsyncer)((*OptiFSFile)(nil))   // Ensuring things are written to disk
var _ = (fs.FileFlusher)((*OptiFSFile)(nil))   // Flushes the file
//var _ = (fs.FileGetattrer)((*OptiFSFile)(nil)) // get attrs of a file
var _ = (fs.FileReleaser)((*OptiFSFile)(nil))  // release (close) a file
var _ = (fs.FileGetlker)((*OptiFSFile)(nil))   // find conflicting locks for given lock
var _ = (fs.FileSetlker)((*OptiFSFile)(nil))   // gets a lock on a file
var _ = (fs.FileSetlkwer)((*OptiFSFile)(nil))  // gets a lock on a file, waits for it to be ready

// makes a filehandle, to give more control over operations on files in the system
// abstract reference to files, where the state of the file (open, offsets, reading etc)
// can be tracked
func NewOptiFSFile(fdesc int, attr fs.StableAttr, flags uint32, currentHash [64]byte, refNum uint64) *OptiFSFile {
	//log.Println("NEW OPTIFSFILE CREATED")
    return &OptiFSFile{fdesc: fdesc, attr: attr, flags: flags, currentHash: currentHash, refNum: refNum}
}

// handles read operations (implements concurrency)
func (f *OptiFSFile) Read(ctx context.Context, dest []byte, offset int64) (fuse.ReadResult, syscall.Errno) {

    log.Println("Reading file")

    log.Println("Checking for custom permissions")
    // Check permissions of custom metadata (if available)
    herr, fileMetadata := metadata.LookupRegularFileMetadata(f.currentHash, f.refNum)
    if herr == nil {
        log.Println("Custom permissions found!")
        allowed := permissions.CheckPermissions(ctx, fileMetadata, 0) // Check read perm
        if !allowed {
            log.Println("User isn't allowed to read file handle!")
            return nil, syscall.EACCES
        }
        log.Println("User is allowed to read file handle")
    }

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

// get the attributes of a file/dir, using the filehandle
//func (f *OptiFSFile) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
//	// lock the operation, and make sure it doesnt unlock until function is exited
//	// unlocks when function is exited
//	f.mu.Lock()
//	defer f.mu.Unlock()
//
//    log.Println("FILE || entered GETATTR")
//
//    // If we can, fill attributes from our filehash
//    err1, fileMetadata := metadata.LookupRegularFileMetadata(f.currentHash, f.refNum)
//    if err1 == nil {
//        metadata.FillAttrOut(fileMetadata, out)
//        return fs.OK
//    }
//
//    // OTHERWISE, just stat the file
//
//	s := syscall.Stat_t{}
//	serr := syscall.Fstat(f.fdesc, &s) // stat the file descriptor to get the attrs (no path needed)
//
//	if serr != nil {
//		return fs.ToErrno(serr)
//	}
//
//	out.FromStat(&s) // fill the attr into struct if no errors
//
//	return fs.OK
//}

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

// gets a lock on a file, if it can't get the lock it fails
func (f *OptiFSFile) Setlk(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	f.mu.Lock()
	defer f.mu.Unlock()

	lock := syscall.Flock_t{}
	lk.ToFlockT(&lock) // convert the FUSE file lock to a system file lock

	if lk.Typ == syscall.F_RDLCK {
		// if we have a read lock
		lock.Type = syscall.F_RDLCK
	} else if lk.Typ == syscall.F_WRLCK {
		// if we have a write lock
		lock.Type = syscall.F_WRLCK
	} else if lk.Typ == syscall.F_UNLCK {
		// if we want to unlock the file
		lock.Type = syscall.F_UNLCK
	} else {
		return syscall.EINVAL // invalid argument passed
	}

	// OFD_SETLK tries to get the lock on a file, if it can't, it will fail and return an error
	// query the status of the file descr
	err := fs.ToErrno(syscall.FcntlFlock(uintptr(f.fdesc), _OFD_SETLK, &lock))

	return err

}

// gets a lock on a file, if it can't get the lock then it waits for the lock to be obtainable
func (f *OptiFSFile) Setlkw(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	f.mu.Lock()
	defer f.mu.Unlock()

	lock := syscall.Flock_t{}
	lk.ToFlockT(&lock) // convert the FUSE file lock to a system file lock

	if lk.Typ == syscall.F_RDLCK {
		// if we have a read lock
		lock.Type = syscall.F_RDLCK
	} else if lk.Typ == syscall.F_WRLCK {
		// if we have a write lock
		lock.Type = syscall.F_WRLCK
	} else if lk.Typ == syscall.F_UNLCK {
		// if we want to unlock the file
		lock.Type = syscall.F_UNLCK
	} else {
		return syscall.EINVAL // invalid argument passed
	}

	// OFD_SETLKW tries to get the lock on a file, if it can't, it will wait for it to become available
	// query the status of the file descr
	err := fs.ToErrno(syscall.FcntlFlock(uintptr(f.fdesc), _OFD_SETLKW, &lock))

	return err

}
