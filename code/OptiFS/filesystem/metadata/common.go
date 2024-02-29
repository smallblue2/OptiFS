// This file contains internal code to the metadata module

package metadata

import (
	"bytes"
	"sort"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
)

// Function updates all MapEntryMetadata attributes from the given unstable attributes
func updateAllFromStat(metadata *MapEntryMetadata, unstableAttr *syscall.Stat_t, stableAttr *fs.StableAttr, path string) {

    // Check to see the appropriate hashmap to lock
    if unstableAttr.Mode&syscall.S_IFDIR != 0 {
        dirMutex.Lock()
        defer dirMutex.Unlock()
    } else {
        metadataMutex.Lock()
        defer metadataMutex.Unlock()
    }

	// Save the path here for dedup purposes
	(*metadata).Path = path

	// Take these from our stable attributes
	(*metadata).Ino = (*stableAttr).Ino
	(*metadata).Gen = (*stableAttr).Gen

	// Take these from the underlying node's stat
	(*metadata).Mode = (*unstableAttr).Mode
	(*metadata).Atim = (*unstableAttr).Atim
	(*metadata).Mtim = (*unstableAttr).Mtim
	(*metadata).Ctim = (*unstableAttr).Ctim
	(*metadata).Uid = (*unstableAttr).Uid
	(*metadata).Gid = (*unstableAttr).Gid
	(*metadata).Dev = (*unstableAttr).Dev
	(*metadata).Rdev = (*unstableAttr).Rdev
	(*metadata).Nlink = (*unstableAttr).Nlink
	(*metadata).Size = (*unstableAttr).Size
	(*metadata).Blksize = (*unstableAttr).Blksize
	(*metadata).Blocks = (*unstableAttr).Blocks
	(*metadata).X__pad0 = (*unstableAttr).X__pad0
	(*metadata).X__unused = (*unstableAttr).X__unused

	// Create the xAttr map if it doesn't exist
	if (*metadata).XAttr == nil {
		(*metadata).XAttr = make(map[string][]byte)
	}
}

// Gets custom extended attributes
func GetCustomXAttr(customMetadata *MapEntryMetadata, attr string, dest *[]byte, isDir bool) (uint32, syscall.Errno) {

	if customMetadata == nil || customMetadata.XAttr == nil {
		return 0, fs.ToErrno(syscall.ENODATA) // Internal error or uninitialized structure
	}

	// Ensure to get the correct lock
	if isDir {
		dirMutex.Lock()
		defer dirMutex.Unlock()
	} else {
		metadataMutex.Lock()
		defer metadataMutex.Unlock()
	}

	// Retrieve and ensure it exists
	bytes, ok := customMetadata.XAttr[attr]
	if !ok {
		return 0, fs.ToErrno(syscall.ENODATA)
	}
	// Fill the destination byte buffer
	*dest = bytes
	// Return the length and OK signal
	return uint32(len(bytes)), fs.OK
}

// Sets custom extended attributes
func SetCustomXAttr(customMetadata *MapEntryMetadata, attr string, data []byte, flags uint32, isDir bool) syscall.Errno {

	if customMetadata == nil || customMetadata.XAttr == nil {
		return fs.ToErrno(syscall.ENODATA) // Internal error or uninitialized structure
	}

	// Ensure to get the correct lock
	if isDir {
		dirMutex.Lock()
		defer dirMutex.Unlock()
	} else {
		metadataMutex.Lock()
		defer metadataMutex.Unlock()
	}

	// Check flags
	if flags&0x1 != 0 { // XATTR_CREATE FLAG
		// Should fail if it already exists
		_, ok := customMetadata.XAttr[attr]
		if ok {
			return fs.ToErrno(syscall.EEXIST)
		}
		customMetadata.XAttr[attr] = data
	} else if flags&0x2 != 0 { // XATTR_REPLACE FLAG
		// Should fail if it doesn't exist
		_, ok := customMetadata.XAttr[attr]
		if !ok {
			return fs.ToErrno(syscall.ENODATA)
		}
		customMetadata.XAttr[attr] = data
	} else {
		// Assume no specific operation defined, just set
		customMetadata.XAttr[attr] = data
	}

	return fs.OK
}

// Sets custom extended attributes
func RemoveCustomXAttr(customMetadata *MapEntryMetadata, attr string, isDir bool) syscall.Errno {

	if customMetadata == nil || customMetadata.XAttr == nil {
		return fs.ToErrno(syscall.ENODATA) // Internal error or uninitialized structure
	}

	// Ensure to get the correct lock
	if isDir {
		dirMutex.Lock()
		defer dirMutex.Unlock()
	} else {
		metadataMutex.Lock()
		defer metadataMutex.Unlock()
	}

	// Ensure it exists, POSIX standard to return ENODATA
	_, ok := customMetadata.XAttr[attr]
	if !ok {
		return fs.ToErrno(syscall.ENODATA)
	}
	delete(customMetadata.XAttr, attr)

	return fs.OK
}

func ListCustomXAttr(customMetadata *MapEntryMetadata, dest *[]byte, isDir bool) (uint32, syscall.Errno) {
	if customMetadata == nil || customMetadata.XAttr == nil {
		return 0, fs.ToErrno(syscall.ENODATA)
	}

	if len(customMetadata.XAttr) == 0 {
		return 0, fs.OK
	}

	// Lock handling remains the same...
	if isDir {
		dirMutex.Lock()
		defer dirMutex.Unlock()
	} else {
		metadataMutex.Lock()
		defer metadataMutex.Unlock()
	}

	// Put attributes into a string slice and sort them to create deterministic behaviour
	var attrNames []string
	for attrName := range customMetadata.XAttr {
		attrNames = append(attrNames, attrName)
	}
	sort.Strings(attrNames)

	// Iterate over the sorted slice and build the result
	var tempBuffer bytes.Buffer
	var totalSizeNeeded uint32
	for _, attrName := range attrNames {
		totalSizeNeeded += uint32(len(attrName)) + 1
		tempBuffer.WriteString(attrName)
		tempBuffer.WriteByte(0)
	}

	if uint32(len(*dest)) < totalSizeNeeded {
		return totalSizeNeeded, fs.ToErrno(syscall.ERANGE)
	}

	copy(*dest, tempBuffer.Bytes())
	return totalSizeNeeded, fs.OK
}

// Updates the Gen Number of the current MapEntryMetadata
func UpdateGenNumber(current *MapEntryMetadata, isDir bool) {
	if isDir {
		dirMutex.Lock()
		defer dirMutex.Unlock()
	} else {
		metadataMutex.Lock()
		defer metadataMutex.Unlock()
	}

	current.Gen++
}
