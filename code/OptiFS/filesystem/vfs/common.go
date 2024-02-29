// Contains code that is common between the virtual node and virtual filehandle

package vfs

import (
	"context"
	"filesystem/metadata"
	"filesystem/permissions"
	"fmt"
	"os/user"
	"syscall"
	"time"
	"unsafe"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Sets the attributes of the provided MapEntryMetadata struct.
//
// Assumes either an OptiFSNode input, or an OptiFSFile input.
func SetAttributes(ctx context.Context, customMetadata *metadata.MapEntryMetadata, in *fuse.SetAttrIn, n *OptiFSNode, f *OptiFSFile, out *fuse.AttrOut, isDir bool) syscall.Errno {


	// If we need to - Manually change the underlying attributes ourselves
	var isOwner bool
	var hasWrite bool
	var path string

	// Check owner and write permission status
	if customMetadata != nil {
		isOwner = permissions.IsOwner(ctx, customMetadata)
		hasWrite = permissions.CheckPermissions(ctx, customMetadata, 1)
		path = customMetadata.Path
	} else {
		path = n.RPath()
	}

	// If the mode needs to be changed
	if mode, ok := in.GetMode(); ok {
		if customMetadata != nil {
			// Ensure the user has ownership of the file
			if !isOwner {
				return syscall.EACCES
			}
			// Try and modify our custom metadata system first
			metadata.UpdateMode(customMetadata, &mode, isDir)
			// Otherwise, just handle the underlying node
		} else {
			// Change the mode to the new mode in the node if provided
			if n != nil {
				if err := syscall.Chmod(path, mode); err != nil {
					return fs.ToErrno(err)
				}
			} else if f != nil {
			// Change the mode to the new mode in the file if provided
				if err := syscall.Fchmod(f.fdesc, mode); err != nil {
					return fs.ToErrno(err)
				}
			}
		}
	}

	// Try and get UID and GID
	uid, uok := in.GetUID()
	gid, gok := in.GetGID()
	// If we have a UID or GID to set
	if uok || gok {
		// Set their default values to -1
		// -1 indicates that the respective value shouldn't change
		safeUID, safeGID := -1, -1
		if uok {
			// Ensure it exists!
			_, uidErr := user.LookupId(fmt.Sprintf("%x", uid))
			if uidErr != nil {
				return fs.ToErrno(syscall.EINVAL)
			}
			safeUID = int(uid)
		}
		if gok {
			// Ensure it exists!
			_, gidErr := user.LookupGroupId(fmt.Sprintf("%x", gid))
			if gidErr != nil {
				return fs.ToErrno(syscall.EINVAL)
			}
			safeGID = int(gid)
		}
		// Try and update our custom metadata system isntead
		if customMetadata != nil {
			// Ensure the user is the current owner
			if !isOwner {
				return syscall.EACCES
			}
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
			metadata.UpdateOwner(customMetadata, saferUID, saferGID, isDir)
			// Otherwise, just update the underlying node instead
		} else {
			if n != nil { // Update the underlying node if provided
				// Chown these values
				err := syscall.Chown(path, safeUID, safeGID)
				if err != nil {
					return fs.ToErrno(err)
				}
			} else if f != nil { // Update the underlying file hande if provided
				// Chown these values
				err := syscall.Fchown(f.fdesc, safeUID, safeGID)
				if err != nil {
					return fs.ToErrno(err)
				}
			}
		}
	}

	// Same thing for modification and access times
	mtime, mok := in.GetMTime()
	atime, aok := in.GetATime()
	ctime, cok := in.GetCTime()
	if mok || aok {
		// Initialize pointers to the time values
		ap := &atime
		mp := &mtime
		cp := &ctime
		// Take into account if access of mod times are not both provided
		if !aok {
			ap = nil
		}
		if !mok {
			mp = nil
		}
		if !cok {
			cp = nil
		}

		// Create an array to hold timespec values for syscall
		// This is a data structure that represents a time value
		// with precision up to nanoseconds
		var times [3]syscall.Timespec
		times[0] = fuse.UtimeToTimespec(ap)
		times[1] = fuse.UtimeToTimespec(mp)
		times[2] = fuse.UtimeToTimespec(cp)

		// Try and update our own custom metadata system first
		if customMetadata != nil {
			// Ensure we have write permissions
			if !hasWrite {
				return syscall.EACCES
			}
			metadata.UpdateTime(customMetadata, &times[0], &times[1], &times[2], isDir)
			// OTHERWISE update the underlying file
		} else {
			// Call the utimenano syscall, ensuring to convert our time array
			// into a slice, as it expects one
			if n != nil { // Change the underlying node if possible
				if err := syscall.UtimesNano(path, times[:]); err != nil {
					return fs.ToErrno(err)
				}
			} else if f != nil {
				// Line below is from github user Hanwen's go-fuse/fuse/nodefs/syscall_linux.go
				_, _, err := syscall.Syscall6(syscall.SYS_UTIMENSAT, uintptr(f.fdesc), 0, uintptr(unsafe.Pointer(&times)), uintptr(0), 0, 0)
				err = syscall.Errno(err)
				if err != 0 {
					return fs.ToErrno(err)
				}
			}
		}
	}

	// If we have a size to update, do so as well
	if size, ok := in.GetSize(); ok {
		// First try and change the custom metadata system
		if customMetadata != nil {
			// Ensure we have write permissions to be updating the size
			if !hasWrite {
				return syscall.EACCES
			}
			tmp := int64(size)
			metadata.UpdateSize(customMetadata, &tmp, isDir)
		} else {
			if n != nil { // Update the underlying node if available
				if err := syscall.Truncate(path, int64(size)); err != nil {
					return fs.ToErrno(err)
				}
			}
			if f != nil { // Update the underlying filehandle if available
				// Change the size
				if err := fs.ToErrno(syscall.Ftruncate(f.fdesc, int64(size))); err != 0 {
					return err
				}
			}
		}
	}

	if out != nil { // If we have attributes to fill out (ONLY FOR NODES)
		// Now reflect these changes in the out stream
		// Use our custom datastruct if we can
		if customMetadata != nil {
			// Fill the AttrOut with our custom attributes stored in our hash
			metadata.FillAttrOut(customMetadata, out)

			return fs.OK
		}

		// Otherwise just stat the underlying file
		stat := syscall.Stat_t{}
		err := syscall.Lstat(path, &stat) // respect symlinks with lstat
		if err != nil {
			return fs.ToErrno(err)
		}
		out.FromStat(&stat)
	}

	// Update change time
	if customMetadata != nil {
		now := time.Now()
		metadata.UpdateTime(customMetadata, nil, nil, &syscall.Timespec{Sec: now.Unix(), Nsec: int64(now.Nanosecond())}, isDir)
	}

	return fs.OK
}

// Handles the creation of intended hardlinks
func HandleHardlinkInstantiation(ctx context.Context, n *OptiFSNode, targetPath, sourcePath, name string, s *syscall.Stat_t, out *fuse.EntryOut) (syscall.Errno, *fs.Inode) {
	// Ensure that there is an existing entry for the source
	sErr, sStableIno, sStableMode, sStableGen, _, _, sHash, sRef := metadata.RetrieveNodeInfo(sourcePath)
	if sErr != fs.OK {
		return syscall.ENOENT, nil
	}
	stable := &fs.StableAttr{Ino: sStableIno, Mode: sStableMode, Gen: sStableGen}

	// Ensure that there ISNT an existing entry for the target
	tErr, _, _, _, _, _, _, _ := metadata.RetrieveNodeInfo(targetPath)
	if tErr == fs.OK {
		return syscall.EEXIST, nil
	}

	// Create a new node to represent the underlying hardlink
	nd := n.RootNode.newNode(n.EmbeddedInode(), name, s)

	// Create the inode structure within FUSE, using the SOURCE's stable attr
	x := n.NewInode(ctx, nd, *stable)

	out.Attr.FromStat(s)

	// Persistently store the info
	metadata.StoreRegFileInfo(targetPath, stable, s.Mode, sHash, sRef)

	return fs.OK, x
}

// Handles the creation of virtual nodes, ensuring we check and prioritise our persistent store to maintain data persistence
func HandleNodeInstantiation(ctx context.Context, n *OptiFSNode, nodePath string, name string, s *syscall.Stat_t, out *fuse.EntryOut, fdesc *int, flags *uint32) (syscall.Errno, *fs.Inode, *OptiFSFile) {


	var fh *OptiFSFile

	// TRY AND FIND CUSTOM NODE
	ferr, sIno, sMode, sGen, _, isDir, existingHash, existingRef := metadata.RetrieveNodeInfo(nodePath)
	// If we got an error (it doesn't exist) OR we have an uninitialised node (ref == 0, hash == [000...000] that isn't a directory)
	if ferr != fs.OK || ((existingRef == 0 && existingHash == [64]byte{}) && s.Mode&syscall.S_IFMT != syscall.S_IFDIR) { // If custom node doesn't exist, create a new one

		// Create a new node to represent the underlying looked up file
		// or directory in our VFS
		nd := n.RootNode.newNode(n.EmbeddedInode(), name, s)

		// Create the inode structure within FUSE, copying the underlying
		// file's attributes with an auto generated inode in idFromStat
		newStable := n.RootNode.getNewStableAttr(s, &nodePath)
		x := n.NewInode(ctx, nd, newStable)

		// Fill the output attributes from out stat struct
		out.Attr.FromStat(s)

		// Check if the lookup is for a directory or not
		stable := x.StableAttr()
		if s.Mode&syscall.S_IFMT == syscall.S_IFDIR {
			// Store the persistent data
			metadata.StoreDirInfo(nodePath, &stable, s.Mode)
			// Create and store the custom metadata
			metadata.CreateDirEntry(nodePath)
			metadata.UpdateDirEntry(nodePath, s, &stable)
		} else {
			// Store the persistent data
			metadata.StoreRegFileInfo(nodePath, &stable, s.Mode, [64]byte{}, 0)
			// Don't create a custom metadata entry here;
			//     custom metadata for regular files are indexed by their content's index
		}

		if fdesc != nil && flags != nil {
			fh = NewOptiFSFile(*fdesc, stable, *flags, [64]byte{}, 0)
		}

		return fs.OK, x, fh
	}


	stable := &fs.StableAttr{Ino: sIno, Mode: sMode, Gen: sGen}

	var nd fs.InodeEmbedder
	// Create a node with the existing attributes we found
	if !isDir {

		nd = n.RootNode.existingNode(existingHash, existingRef)

		cerr, customMetadata := metadata.LookupRegularFileMetadata(existingHash, existingRef)
		if cerr != fs.OK {
			// Must be an empty or special file
			// TODO: Do we need to do anything special here?
		}

		metadata.FillAttr(customMetadata, &out.Attr)

		if fdesc != nil && flags != nil {
			fh = NewOptiFSFile(*fdesc, *stable, *flags, existingHash, existingRef)
		}

		x := n.NewInode(ctx, nd, *stable)

		return fs.OK, x, fh

	} else {

		nd = n.RootNode.newNode(n.EmbeddedInode(), name, s)

		cerr, customMetadata := metadata.LookupDirMetadata(nodePath)
		if cerr != 0 {
			return fs.ToErrno(syscall.ENODATA), nil, nil
		}
		metadata.FillAttr(customMetadata, &out.Attr)

		x := n.NewInode(ctx, nd, *stable)

		return fs.OK, x, fh
	}
}
