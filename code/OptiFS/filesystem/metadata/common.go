// This file contains internal code to the metadata module

package metadata

import (
	"log"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
)

// Function updates all MapEntryMetadata attributes from the given unstable attributes
func updateAllFromStat(metadata *MapEntryMetadata, unstableAttr *syscall.Stat_t, stableAttr *fs.StableAttr, path string) {

    log.Printf("New Mode: 0x%X\n", (*stableAttr).Mode)
	// not sure if we should lock dirMutex or metadataMutex
	// -> used in both sets of functions but takes in a MapEntryMetadata??

	// needs a write lock as we are modifying the attributes
	dirMutex.Lock()
	defer dirMutex.Unlock()

    // Save the path here for dedup purposes
    (*metadata).Path = path

    // Take these from our stable attributes
    (*metadata).Ino = (*stableAttr).Ino
    (*metadata).Gen = (*stableAttr).Gen

	log.Printf("New Mode: 0x%X\n", (*stableAttr).Mode)
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
func GetCustomXAttr(customMetadata *MapEntryMetadata, attr string , dest *[]byte, isDir bool) (uint32, syscall.Errno) {

    log.Println("Getting custom xattr")

    if customMetadata == nil || customMetadata.XAttr == nil {
        log.Println("No custom metadata or XAttr available!")
        return 0, fs.ToErrno(syscall.EIO) // Internal error or uninitialized structure
    }

    // Ensure to get the correct lock
    log.Println("Getting correct lock")
    if isDir {
        log.Println("Requesting dirMutex write lock")
        dirMutex.Lock()
        defer dirMutex.Unlock()
    } else {
        log.Println("Requesting regfile write lock")
        metadataMutex.Lock()
        defer metadataMutex.Unlock()
    }
    log.Println("Obtained lock")

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
func SetCustomXAttr(customMetadata *MapEntryMetadata, attr string, data []byte, flags uint32, isDir bool) (syscall.Errno) {

    log.Println("Setting custom xattr")

    if customMetadata == nil || customMetadata.XAttr == nil {
        log.Println("No custom metadata or XAttr available!")
        return fs.ToErrno(syscall.EIO) // Internal error or uninitialized structure
    }

    // Ensure to get the correct lock
    log.Println("Getting correct lock")
    if isDir {
        log.Println("Requesting dirMutex write lock")
        dirMutex.Lock()
        defer dirMutex.Unlock()
    } else {
        log.Println("Requesting regfile write lock")
        metadataMutex.Lock()
        defer metadataMutex.Unlock()
    }
    log.Println("Obtained lock")

    log.Printf("XAttr Write Flag - {0x%X}\n", flags)

    // Check flags
    if flags&0x1 != 0 { // XATTR_CREATE FLAG
        // Should fail if it already exists
        _, ok := customMetadata.XAttr[attr]
        if ok {
            return fs.ToErrno(syscall.EEXIST)
        }
        customMetadata.XAttr[attr] = data
        log.Printf("XATTR_CREATE operation performed: {%v} -> {%v}\n", attr, customMetadata.XAttr[attr])
    } else if flags&0x2 != 0 { // XATTR_REPLACE FLAG
        // Should fail if it doesn't exist
        _, ok := customMetadata.XAttr[attr]
        if !ok {
            return fs.ToErrno(syscall.ENODATA)
        }
        customMetadata.XAttr[attr] = data
        log.Printf("XATTR_REPLACE operation performed: {%v} -> {%v}\n", attr, customMetadata.XAttr[attr])
    } else {
        // Assume no specific operation defined, just set
        customMetadata.XAttr[attr] = data
        log.Printf("NO FLAG operation performed: {%v} -> {%v}\n", attr, customMetadata.XAttr[attr])
    }

    return fs.OK
}

// Sets custom extended attributes
func RemoveCustomXAttr(customMetadata *MapEntryMetadata, attr string, isDir bool) (syscall.Errno) {

    log.Println("Removing custom xattr")

    if customMetadata == nil || customMetadata.XAttr == nil {
        log.Println("No custom metadata or XAttr available!")
        return fs.ToErrno(syscall.EIO) // Internal error or uninitialized structure
    }

    // Ensure to get the correct lock
    log.Println("Getting correct lock")
    if isDir {
        log.Println("Requesting dirMutex write lock")
        dirMutex.Lock()
        defer dirMutex.Unlock()
    } else {
        log.Println("Requesting regfile write lock")
        metadataMutex.Lock()
        defer metadataMutex.Unlock()
    }
    log.Println("Obtained lock")

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
        log.Println("No custom metadata or XAttr available!")
        return 0, fs.ToErrno(syscall.EIO)
    }

    // Lock handling remains the same...
    log.Println("Getting correct lock")
    if isDir {
        log.Println("Requesting dirMutex write lock")
        dirMutex.Lock()
        defer dirMutex.Unlock()
    } else {
        log.Println("Requesting regfile write lock")
        metadataMutex.Lock()
        defer metadataMutex.Unlock()
    }
    log.Println("Obtained lock")

    var totalSizeNeeded uint32
    // First, calculate the total size needed for all attributes including null terminators.
    for attrName := range customMetadata.XAttr {
        totalSizeNeeded += uint32(len(attrName)) + 1 // +1 for the null terminator
    }

    // Now, check if the provided buffer (*dest) is large enough to hold all attributes.
    if totalSizeNeeded > uint32(len(*dest)) {
        // If not, return the total size needed and ERANGE error to indicate the buffer is too small.
        log.Println("Buffer too small!")
        return totalSizeNeeded, fs.ToErrno(syscall.ERANGE)
    }
    log.Println("Buffer is big enough")

    // If the buffer is large enough, proceed to append attribute names with null terminators.
    var actualSize uint32
    for attrName := range customMetadata.XAttr {
        log.Printf("Filling {%v}\n", attrName)
        attrBytesWithNull := append([]byte(attrName), 0) // Convert and append null terminator
        *dest = append(*dest, attrBytesWithNull...)
        actualSize += uint32(len(attrBytesWithNull))
        log.Printf("Updated, size: {%v}\n", actualSize)
    }

    // Return the actual size used in the buffer, which should match totalSizeNeeded.
    return actualSize, fs.OK
}
