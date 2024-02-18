// This file contains all public functions for external modules to communicate with general aspects of the Metadata module

package metadata

import (
	"log"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Updates a MapEntryMetadata object with all data provided from the Stat_t object passed
func FullMapEntryMetadataUpdate(metadata *MapEntryMetadata, unstableAttr *syscall.Stat_t, stableAttr *fs.StableAttr, path string) error {
	// locks for this function are implemented in the functions being called
	// done to prevent deadlock

	log.Println("Updating metadata through struct...")
	log.Printf("unstableAttr: %+v\n", unstableAttr)

	updateAllFromStat(metadata, unstableAttr, stableAttr, path)

	log.Printf("metadata: %+v\n", metadata)
	log.Println("Updated all custom metadata attributes through struct")

	return nil
}

func FillAttr(customMetadata *MapEntryMetadata, out *fuse.Attr) {
	// needs a write lock as we are modifying the metadata
	metadataMutex.Lock()
	defer metadataMutex.Unlock()

	(*out).Ino = (*customMetadata).Ino
    (*out).Owner = fuse.Owner{Uid: customMetadata.Uid, Gid: customMetadata.Gid}
	(*out).Size = uint64((*customMetadata).Size)
	(*out).Blocks = uint64((*customMetadata).Blocks)
	(*out).Atime = uint64((*customMetadata).Atim.Sec)
	(*out).Atimensec = uint32((*customMetadata).Atim.Nsec)
	(*out).Mtime = uint64((*customMetadata).Mtim.Sec)
	(*out).Mtimensec = uint32((*customMetadata).Mtim.Nsec)
	(*out).Ctime = uint64((*customMetadata).Ctim.Sec)
	(*out).Ctimensec = uint32((*customMetadata).Ctim.Nsec)
	(*out).Mode = (*customMetadata).Mode
	(*out).Nlink = uint32((*customMetadata).Nlink)
	(*out).Uid = (*customMetadata).Uid
	(*out).Gid = (*customMetadata).Gid
	(*out).Rdev = uint32((*customMetadata).Rdev)
	(*out).Blksize = uint32((*customMetadata).Blksize)
}

// Function updates the UID and GID of a MapEntryMetadata
// Accepts pointers, doesn't set nil values
func UpdateOwner(metadata *MapEntryMetadata, uid, gid *uint32, isDir bool) error {
	// we check to see if we are dealing with a directory or not
	// so we know which lock to instantiate
	if isDir {
		dirMutex.Lock()
		defer dirMutex.Unlock()
	} else {
		metadataMutex.Lock()
		defer metadataMutex.Unlock()
	}

	if uid != nil {
		(*metadata).Uid = *uid
		log.Println("Updated custom UID")
	}
	if gid != nil {
		(*metadata).Gid = *gid
		log.Println("Updated custom GID")
	}
	return nil
}

// Function updates the time data of a MapEntryMetadata
// Accepts pointers, doesn't set nil values
func UpdateTime(metadata *MapEntryMetadata, atim, mtim, ctim *syscall.Timespec, isDir bool) error {
	// we check to see if we are dealing with a directory or not
	// so we know which lock to instantiate
	if isDir {
		dirMutex.Lock()
		defer dirMutex.Unlock()
	} else {
		metadataMutex.Lock()
		defer metadataMutex.Unlock()
	}

	if atim != nil {
		(*metadata).Atim = *atim
		log.Println("Updated custom ATime")
	}
	if mtim != nil {
		(*metadata).Mtim = *mtim
		log.Println("Updated custom MTime")
	}
	if ctim != nil {
		(*metadata).Ctim = *ctim
		log.Println("Updated custom CTime")
	}
	return nil
}

// Function updates inode and device fields of a MapEntryMetadata
// Accepts pointers, doesn't set nil values
func UpdateLocation(metadata *MapEntryMetadata, inode, dev *uint64, isDir bool) error {
	// we check to see if we are dealing with a directory or not
	// so we know which lock to instantiate
	if isDir {
		dirMutex.Lock()
		defer dirMutex.Unlock()
	} else {
		metadataMutex.Lock()
		defer metadataMutex.Unlock()
	}

	if inode != nil {
		(*metadata).Ino = *inode
		log.Println("Updated custom Inode")
	}
	if dev != nil {
		(*metadata).Dev = *dev
		log.Println("Updated custom Device")
	}
	return nil
}

// Function updates size field of a MapEntryMetadata
// Accepts pointers, doesn't set nil values
func UpdateSize(metadata *MapEntryMetadata, size *int64, isDir bool) error {
	// we check to see if we are dealing with a directory or not
	// so we know which lock to instantiate
	if isDir {
		dirMutex.Lock()
		defer dirMutex.Unlock()
	} else {
		metadataMutex.Lock()
		defer metadataMutex.Unlock()
	}

	if size != nil {
		(*metadata).Size = *size
		log.Println("Updated custom Size")
	}
	return nil
}

// Function updates link count of a MapEntryMetadata
// Accepts pointers, doesn't set nil values
func UpdateLinkCount(metadata *MapEntryMetadata, linkCount *uint64, isDir bool) error {
	// we check to see if we are dealing with a directory or not
	// so we know which lock to instantiate
	if isDir {
		dirMutex.Lock()
		defer dirMutex.Unlock()
	} else {
		metadataMutex.Lock()
		defer metadataMutex.Unlock()
	}

	if linkCount != nil {
		(*metadata).Nlink = *linkCount
		log.Println("Updated custom Nlink")
	}
	return nil
}

// Function updates mode of a MapEntryMetadata
// Accepts pointers, doesn't set nil values
func UpdateMode(metadata *MapEntryMetadata, mode *uint32, isDir bool) error {
	// we check to see if we are dealing with a directory or not
	// so we know which lock to instantiate
	if isDir {
		dirMutex.Lock()
		defer dirMutex.Unlock()
	} else {
		metadataMutex.Lock()
		defer metadataMutex.Unlock()
	}

	if mode != nil {
		(*metadata).Mode = *mode
		log.Println("Updated custom Mode")
	}
	return nil
}

// Function update C++ struct padding optimisation variables - not sure if they're used or needed
// Accepts pointers, doesn't set nil values
func UpdateWeirdCPPStuff(metadata *MapEntryMetadata, X__pad0 *int32, X__unused *[3]int64, isDir bool) error {
	// we check to see if we are dealing with a directory or not
	// so we know which lock to instantiate
	if isDir {
		dirMutex.Lock()
		defer dirMutex.Unlock()
	} else {
		metadataMutex.Lock()
		defer metadataMutex.Unlock()
	}

	if X__pad0 != nil {
		(*metadata).X__pad0 = *X__pad0
		log.Println("Updated custom X__pad0")
	}
	if X__unused != nil {
		(*metadata).X__unused = *X__unused
		log.Println("Updated custom X__unused")
	}
	return nil
}

// Function fills the FUSE AttrOut struct with the metadata contained in the provided MapEntryMetadata
// struct.
func FillAttrOut(metadata *MapEntryMetadata, out *fuse.AttrOut) {
	// needs a write lock as we are modifying the metadata
	metadataMutex.Lock()
	defer metadataMutex.Unlock()

	log.Printf("metadata: %+v\n", metadata)

	// Fill the AttrOut with our custom attributes stored in our hash
    (*out).Attr.Ino = (*metadata).Ino
	(*out).Attr.Size = uint64((*metadata).Size)
    (*out).Attr.Owner = fuse.Owner{Uid: (*metadata).Uid, Gid: (*metadata).Gid}
	(*out).Attr.Blocks = uint64((*metadata).Blocks)
	(*out).Attr.Atime = uint64((*metadata).Atim.Sec)
	(*out).Attr.Atimensec = uint32((*metadata).Atim.Nsec)
	(*out).Attr.Mtime = uint64((*metadata).Mtim.Sec)
	(*out).Attr.Mtimensec = uint32((*metadata).Mtim.Nsec)
	(*out).Attr.Ctime = uint64((*metadata).Ctim.Sec)
	(*out).Attr.Ctimensec = uint32((*metadata).Ctim.Nsec)
	(*out).Attr.Mode = (*metadata).Mode
	(*out).Attr.Nlink = uint32((*metadata).Nlink)
	(*out).Attr.Uid = uint32((*metadata).Uid)
	(*out).Attr.Gid = uint32((*metadata).Gid)
	(*out).Attr.Rdev = uint32((*metadata).Rdev)
	(*out).Attr.Blksize = uint32((*metadata).Blksize)

	log.Println("Filled AttrOut from custom metadata")
}
