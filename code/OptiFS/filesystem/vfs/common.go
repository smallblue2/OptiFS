// Contains code that is common between the virtual node and virtual filehandle

package vfs

import (
	"filesystem/metadata"
	"log"
	"syscall"
	"unsafe"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Sets the attributes of the provided MapEntryMetadata struct.
//
// Assumes either an OptiFSNode input, or an OptiFSFile input.
func SetAttributes(customMetadata *metadata.MapEntryMetadata, in *fuse.SetAttrIn, n *OptiFSNode, f *OptiFSFile, out *fuse.AttrOut) syscall.Errno {
	// If we need to - Manually change the underlying attributes ourselves
    path := n.RPath()

    log.Printf("Setting attributes for '%v'\n", path)

	// If the mode needs to be changed
	if mode, ok := in.GetMode(); ok {
        log.Println("Setting Mode")
        if customMetadata != nil {
            // Try and modify our custom metadata system first
            metadata.UpdateMode(customMetadata, &mode)
            log.Println("Set custom mode")
            // Otherwise, just handle the underlying node
        } else {
			// Change the mode to the new mode in the node if provided
            if n != nil {
                if err := syscall.Chmod(path, mode); err != nil {
                    return fs.ToErrno(err)
                }
                log.Println("Set underlying NODE mode")
            }
			// Change the mode to the new mode in the file if provided
            if f != nil {
                if err := syscall.Fchmod(f.fdesc, mode); err != nil {
                    return fs.ToErrno(err)
                }
                log.Println("Set underlying FILEHANDLE mode")
            }
		}
	}

	// Try and get UID and GID
	uid, uok := in.GetUID()
	gid, gok := in.GetGID()
	// If we have a UID or GID to set
	if uok || gok {
        log.Println("Setting UID & GID")
		// Set their default values to -1
		// -1 indicates that the respective value shouldn't change
		safeUID, safeGID := -1, -1
		if uok {
			safeUID = int(uid)
		}
		if gok {
			safeGID = int(gid)
		}
		// Try and update our custom metadata system isntead
		if customMetadata != nil {
			// As our update function works on optional pointers, convert
			// the safeguarding to work with pointers
			var saferUID *uint32
			var saferGID *uint32
			if safeUID != -1 {
				tmp := uint32(safeUID)
				saferUID = &tmp
			}
			if safeGID != -1 {
				tmp := uint32(safeGID)
				saferGID = &tmp
			}
            metadata.UpdateOwner(customMetadata, saferUID, saferGID)
            log.Println("Set custom UID & GID")
			// Otherwise, just update the underlying node instead
		} else {
            if n != nil { // Update the underlying node if provided
                // Chown these values
                err := syscall.Chown(path, safeUID, safeGID)
                if err != nil {
                    return fs.ToErrno(err)
                }
                log.Println("Set underlying NODE UID & GID")
            }
            if f != nil { // Update the underlying file hande if provided
                // Chown these values
                err := syscall.Fchown(f.fdesc, safeUID, safeGID)
                if err != nil {
                    return fs.ToErrno(err)
                }
                log.Println("Set underlying FILEHANDLE UID & GID")
            }
		}
	}

	// Same thing for modification and access times
	mtime, mok := in.GetMTime()
	atime, aok := in.GetATime()
	if mok || aok {
        log.Println("Setting time")
		// Initialize pointers to the time values
		ap := &atime
		mp := &mtime
		// Take into account if access of mod times are not both provided
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

		// Try and update our own custom metadata system first
		if customMetadata != nil {
			metadata.UpdateTime(customMetadata, &times[0], &times[1], nil)
            log.Println("Set custom time")
			// OTHERWISE update the underlying file
		} else {
			// Call the utimenano syscall, ensuring to convert our time array
			// into a slice, as it expects one
            if n != nil { // Change the underlying node if possible
                if err := syscall.UtimesNano(path, times[:]); err != nil {
                    return fs.ToErrno(err)
                }
                log.Println("Updated underlying NODE time")
            }
            if f != nil {
                // BELOW LINE IS FROM `fs` package, hanwen - TODO: REFERENCE PROPERLY
                _, _, err := syscall.Syscall6(syscall.SYS_UTIMENSAT, uintptr(f.fdesc), 0, uintptr(unsafe.Pointer(&times)), uintptr(0), 0, 0)
                err = syscall.Errno(err)
                if err != 0 {
                    return fs.ToErrno(err)
                }
                log.Println("Updated underlying FILEHANDLE time")
            }
		}
	}

	// If we have a size to update, do so as well
	if size, ok := in.GetSize(); ok {
        log.Println("Updating size")
		// First try and change the custom metadata system
		if customMetadata != nil {
			tmp := int64(size)
			metadata.UpdateSize(customMetadata, &tmp)
            log.Println("Updated custom size")
        } else {
            if n != nil { // Update the underlying node if available
                if err := syscall.Truncate(path, int64(size)); err != nil {
                    return fs.ToErrno(err)
                }
                log.Println("Updated underlying NODE size")
            }
            if f != nil { // Update the underlying filehandle if available
                // Change the size
                if err := fs.ToErrno(syscall.Ftruncate(f.fdesc, int64(size))); err != 0 {
                    return err
                }
                log.Println("Updated underlying FILEHANDLE size")
            }
		}
	}

    if out != nil { // If we have attributes to fill out (ONLY FOR NODES)
        log.Println("Filling AttrOut")
        // Now reflect these changes in the out stream
        // Use our custom datastruct if we can
        if customMetadata != nil {
            log.Println("Reflecting custom attributes changes!")
            // Fill the AttrOut with our custom attributes stored in our hash
            metadata.FillAttrOut(customMetadata, out)

            return fs.OK
        }

        // Otherwise just stat the underlying file
        log.Println("Reflecting underlying attributes changes!")
        stat := syscall.Stat_t{}
        err := syscall.Lstat(path, &stat) // respect symlinks with lstat
        if err != nil {
            return fs.ToErrno(err)
        }
        out.FromStat(&stat)
    }

	return fs.OK
}
