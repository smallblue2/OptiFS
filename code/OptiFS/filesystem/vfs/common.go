// Contains code that is common between the virtual node and virtual filehandle

package vfs

import (
	"context"
	"filesystem/metadata"
	"filesystem/permissions"
	"fmt"
	"log"
	"os/user"
	"syscall"
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
		log.Println("We have custom metadata!")
		isOwner = permissions.IsOwner(ctx, customMetadata)
		log.Printf("Is user owner: {%v}\n", isOwner)
		hasWrite = permissions.CheckPermissions(ctx, customMetadata, 1)
		log.Printf("Has write perm: {%v}\n", hasWrite)
		path = customMetadata.Path
	} else {
		log.Println("No custom metadata!")
		path = n.RPath()
	}

	log.Printf("Setting attributes for '%v'\n", path)

	// If the mode needs to be changed
	if mode, ok := in.GetMode(); ok {
		log.Println("Setting Mode")
		if customMetadata != nil {
			// Ensure the user has ownership of the file
			if !isOwner {
				return syscall.EACCES
			}
			// Try and modify our custom metadata system first
			metadata.UpdateMode(customMetadata, &mode, isDir)
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
			// Ensure we have write permissions
			if !hasWrite {
				log.Println("User doesn't have permission to change time!")
				return syscall.EACCES
			}
			metadata.UpdateTime(customMetadata, &times[0], &times[1], nil, isDir)
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
			// Ensure we have write permissions to be updating the size
			if !hasWrite {
				log.Println("User doesn't have permission to change the size!")
				return syscall.EACCES
			}
			tmp := int64(size)
			metadata.UpdateSize(customMetadata, &tmp, isDir)
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

// Handles the creation of intended hardlinks
func HandleHardlinkInstantiation(ctx context.Context, n *OptiFSNode, targetPath, sourcePath, name string, s *syscall.Stat_t, out *fuse.EntryOut) (syscall.Errno, *fs.Inode) {
	// Ensure that there is an existing entry for the source
	sErr, sStableIno, sStableMode, sStableGen, _, _, sHash, sRef := metadata.RetrieveNodeInfo(sourcePath)
	if sErr != nil {
		return syscall.ENOENT, nil
	}
	stable := &fs.StableAttr{Ino: sStableIno, Mode: sStableMode, Gen: sStableGen}

	// Ensure that there ISNT an existing entry for the target
	tErr, _, _, _, _, _, _, _ := metadata.RetrieveNodeInfo(targetPath)
	if tErr == nil {
		return syscall.EEXIST, nil
	}

	// Create a new node to represent the underlying hardlink
	nd := n.RootNode.newNode(n.EmbeddedInode(), name, s)
	log.Println("Created new InodeEmbedder")

	// Create the inode structure within FUSE, using the SOURCE's stable attr
	x := n.NewInode(ctx, nd, *stable)
	log.Println("Created Inode")

	out.Attr.FromStat(s)
	log.Println("Filled out from stat")

	// Persistently store the info
	metadata.StoreRegFileInfo(targetPath, stable, s.Mode, sHash, sRef)

	return fs.OK, x
}

// Handles the creation of virtual nodes, ensuring we check and prioritise our persistent store to maintain data persistence
func HandleNodeInstantiation(ctx context.Context, n *OptiFSNode, nodePath string, name string, s *syscall.Stat_t, out *fuse.EntryOut, fdesc *int, flags *uint32) (syscall.Errno, *fs.Inode, *OptiFSFile) {

	log.Println("Handling Node Instantiation")

	var fh *OptiFSFile

	// TRY AND FIND CUSTOM NODE
	ferr, sIno, sMode, sGen, _, isDir, existingHash, existingRef := metadata.RetrieveNodeInfo(nodePath)
	if ferr != nil { // If custom node doesn't exist, create a new one

		log.Println("Persistent node entry doesn't exist")

		log.Printf("Mode: 0x%X\n", s.Mode)

		// Create a new node to represent the underlying looked up file
		// or directory in our VFS
		nd := n.RootNode.newNode(n.EmbeddedInode(), name, s)
		log.Println("Created new InodeEmbedder")

		// Create the inode structure within FUSE, copying the underlying
		// file's attributes with an auto generated inode in idFromStat
		newStable := n.RootNode.getNewStableAttr(s, &nodePath)
		log.Printf("Mode inside newStable: 0x%X\n", newStable.Mode)
		x := n.NewInode(ctx, nd, newStable)
		log.Println("Created Inode")

		log.Printf("Mode: 0x%X\n", x.StableAttr())

		// Fill the output attributes from out stat struct
		out.Attr.FromStat(s)
		log.Println("Filled out from stat")

		// Check if the lookup is for a directory or not
		stable := x.StableAttr()
		if s.Mode&syscall.S_IFMT == syscall.S_IFDIR {
			// Store the persistent data
			metadata.StoreDirInfo(nodePath, &stable, s.Mode)
			log.Println("STORED DIRECTORY PERSISTENT DATA")
			// Create and store the custom metadata
			metadata.CreateDirEntry(nodePath)
			metadata.UpdateDirEntry(nodePath, s, &stable)
			log.Println("STORED DIRECTORY CUSTOM METADATA")
		} else {
			// Store the persistent data
			log.Println("STORED REGULAR FILE PERSISTENT DATA")
			metadata.StoreRegFileInfo(nodePath, &stable, s.Mode, [64]byte{}, 0)
			// Don't create a custom metadata entry here;
			//     custom metadata for regular files are indexed by their content's index
		}

		if fdesc != nil && flags != nil {
			fh = NewOptiFSFile(*fdesc, stable, *flags, [64]byte{}, 0)
		}

		return fs.OK, x, fh
	}

	log.Printf("Found existing persistent node entry - ISDIR: {%v} REFNUM {%v} HASH {%+v}\n", isDir, existingRef, existingHash)

	stable := &fs.StableAttr{Ino: sIno, Mode: sMode, Gen: sGen}

	var nd fs.InodeEmbedder
	// Create a node with the existing attributes we found
	if !isDir {
		log.Println("IS a regular file!")

		nd = n.RootNode.existingNode(existingHash, existingRef)
		log.Println("Created existing InodeEmbedder!")

		cerr, customMetadata := metadata.LookupRegularFileMetadata(existingHash, existingRef)
		if cerr != nil {
			// Must be an empty or special file
			log.Println("Must be an empty or special file")
			// TODO: Do we need to do anything special here?
		}

		metadata.FillAttr(customMetadata, &out.Attr)
		log.Println("Filled out attributes with custom metadata!")

		if fdesc != nil && flags != nil {
			fh = NewOptiFSFile(*fdesc, *stable, *flags, existingHash, existingRef)
		}

		x := n.NewInode(ctx, nd, *stable)

		return fs.OK, x, fh

	} else {
		log.Println("IS a directory!")

		nd = n.RootNode.newNode(n.EmbeddedInode(), name, s)
		log.Println("Created new InodeEmbedder!")

		cerr, customMetadata := metadata.LookupDirMetadata(nodePath)
		if cerr != nil {
			log.Println("No custom metadata found - EXITING!")
			return fs.ToErrno(syscall.ENODATA), nil, nil
		}
		metadata.FillAttr(customMetadata, &out.Attr)
		log.Println("Filled out attributes with custom metadata!")

		x := n.NewInode(ctx, nd, *stable)

		return fs.OK, x, fh
	}
}
