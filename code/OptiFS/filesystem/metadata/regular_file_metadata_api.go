// This file contains all public functions for external modules to communicate with the regularFileMetadataHash

package metadata

import (
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
)

// Detects if a file is empty (or technically a directory, special file, etc)
func EmptyFileIdentifier(contentHash [64]byte) bool {
	var defaultHash [64]byte
	// if the hash is empty (0000000000...) then the file must be empty
	if contentHash == defaultHash {
		return true
	} else {
		return false
	}
}

// Performs a lookup on the regularFileMetadataHash to tell if the provided content hash is unique.
//
// Returns a bool for whether the contentHash can be found
func IsContentHashUnique(contentHash [64]byte) bool {

	// if it is an empty file
	if EmptyFileIdentifier(contentHash) {
		return true
	}

	// needs a read lock as data is not being modified, only read, so multiple
	// operations can read at the same time (concurrently)
	metadataMutex.RLock()
	defer metadataMutex.RUnlock()

	// Check to see if there's an entry for the contentHash and refNum above
	_, exists := regularFileMetadataHash[contentHash]
	// If it doesn't exist
	if !exists {
		return !exists
	}

	// If it exists, return the underlying Inode
	return !exists
}

// Gets the most recent entry in a MapEntry
// Returns nil if there is no entry
func RetrieveRecent(entry *MapEntry) *MapEntryMetadata {
	// needs a read lock as data is not being modified, only read, so multiple
	// operations can read at the same time (concurrently)
	metadataMutex.RLock()
	defer metadataMutex.RUnlock()

	indx := entry.IndexCounter
	// If it's empty, return nothing
	if indx == 0 {
		return nil
	}
	for indx := entry.IndexCounter; indx >= 0; indx-- {
		meta, ok := entry.EntryList[indx]
		if ok {
			return meta
		}
	}
	return nil
}

// Retrieves regular file metadata for a hash and refnum provided. Returns an error if it cannot be found
func LookupRegularFileMetadata(contentHash [64]byte, refNum uint64) (syscall.Errno, *MapEntryMetadata) {
	// needs a read lock as data is not being modified, only read, so multiple
	// operations can read at the same time (concurrently)
	metadataMutex.RLock()
	defer metadataMutex.RUnlock()

	// Now actually query the hashmap
	if contentEntry, ok := regularFileMetadataHash[contentHash]; ok {
		if nodeMetadata, ok := contentEntry.EntryList[refNum]; ok {
			return fs.OK, nodeMetadata
		}
	}

	return fs.ToErrno(syscall.ENODATA), nil
}

// Retrieves a MapEntry instance from regularFileMetadataHash using the content hash provided.
//
// Returns the retrived MapEntry, or an error if it doesn't exist
func LookupRegularFileEntry(contentHash [64]byte) (syscall.Errno, *MapEntry) {
	// needs a read lock as data is not being modified, only read, so multiple
	// operations can read at the same time (concurrently)
	metadataMutex.RLock()
	defer metadataMutex.RUnlock()

	entry, ok := regularFileMetadataHash[contentHash]
	if !ok {
		return fs.ToErrno(syscall.ENODATA), nil
	}

	return fs.OK, entry
}

// Removes a MapEntryMetadata instance in regularFileMetadataHash based on content hash and refnum provided.
// Also handles if this potentially creates an empty MapEntry struct.
func RemoveRegularFileMetadata(contentHash [64]byte, refNum uint64) syscall.Errno {
	// Check to see if an entry exists
	err, entry, _ := RetrieveRegularFileMapEntryAndMetadataFromHashAndRef(contentHash, refNum)
	if err != fs.OK {
		return err
	}

	metadataMutex.Lock()
	// Delete the metadata from our entry
	delete(entry.EntryList, refNum)
	// Reflect these changes in the MapEntry
	entry.ReferenceCount--
	metadataMutex.Unlock()

	// Check to see if the MapEntry is empty
	if entry.ReferenceCount == 0 {
		// If it is, delete the whole entry
		metadataMutex.Lock()
		delete(regularFileMetadataHash, contentHash)
		metadataMutex.Unlock()
	}

	return fs.OK
}

// Retrieves the MapEntry struct from which the Metadata entry struct that the refNum and contentHash links to
func RetrieveRegularFileMapEntryFromHashAndRef(contentHash [64]byte, refNum uint64) (syscall.Errno, *MapEntry) {

	// First check for default values
	var defaultByteArray [64]byte
	if contentHash == defaultByteArray || refNum == 0 {
		return fs.ToErrno(syscall.ENODATA), nil
	}

	// needs a read lock as data is not being modified, only read, so multiple
	// operations can read at the same time (concurrently)
	metadataMutex.RLock()
	defer metadataMutex.RUnlock()

	// Now actually query the hashmap
	if contentEntry, ok := regularFileMetadataHash[contentHash]; ok {
		if _, ok := contentEntry.EntryList[refNum]; ok {
			return fs.OK, contentEntry
		}
	}

	return fs.ToErrno(syscall.ENODATA), nil
}

// Retrieves the MapEntry and MapEntryMetadata struct from which the reference num and content hash links to
func RetrieveRegularFileMapEntryAndMetadataFromHashAndRef(contentHash [64]byte, refNum uint64) (syscall.Errno, *MapEntry, *MapEntryMetadata) {

	// First check for default values
	var defaultByteArray [64]byte
	if contentHash == defaultByteArray || refNum == 0 {
		return fs.ToErrno(syscall.ENODATA), nil, nil
	}

	// needs a read lock as data is not being modified, only read, so multiple
	// operations can read at the same time (concurrently)
	metadataMutex.RLock()
	defer metadataMutex.RUnlock()

	// Now actually query the hashmap
	if contentEntry, ok := regularFileMetadataHash[contentHash]; ok {
		if metadataEntry, ok := contentEntry.EntryList[refNum]; ok {
			return fs.OK, contentEntry, metadataEntry
		}
	}

	return fs.ToErrno(syscall.ENODATA), nil, nil
}

// Updates a MapEntryMetadata instance corresponding to the content hash and reference num provided
//
// If refNum or contentHash is invalid, it returns an error
func UpdateFullRegularFileMetadata(contentHash [64]byte, refNum uint64, unstableAttr *syscall.Stat_t, stableAttr *fs.StableAttr, path string) syscall.Errno {

	// locks for this function are implemented in the functions being called
	// done to prevent deadlock

	// Ensure that contentHash and refNum is valid
	// this function already has locks!
	err, metadata := LookupRegularFileMetadata(contentHash, refNum)
	if err != fs.OK {
		return err
	}

	// Now we can be sure the entry exists, let's update it
	// this function also already has locks!
	updateAllFromStat(metadata, unstableAttr, stableAttr, path)

	return fs.OK
}

// Moves old metadata to a new node being created
func MigrateRegularFileMetadata(oldMeta *MapEntryMetadata, newMeta *MapEntryMetadata, unstableAttr *syscall.Stat_t) syscall.Errno {
	// needs a write lock as we are modifying the metadata
	metadataMutex.Lock()
	defer metadataMutex.Unlock()

	// Old attributes to carry across
	(*newMeta).Path = (*oldMeta).Path
	(*newMeta).Mode = (*oldMeta).Mode
	(*newMeta).Ctim = (*oldMeta).Ctim
	(*newMeta).Uid = (*oldMeta).Uid
	(*newMeta).Gid = (*oldMeta).Gid
	(*newMeta).Dev = (*oldMeta).Dev
	(*newMeta).Ino = (*oldMeta).Ino
	(*newMeta).Gen = (*oldMeta).Gen
	(*newMeta).XAttr = (*oldMeta).XAttr

	// New attributes to refresh from stat
	(*newMeta).Atim = (*unstableAttr).Atim
	(*newMeta).Mtim = (*unstableAttr).Mtim
	(*newMeta).Rdev = (*unstableAttr).Rdev
	(*newMeta).Nlink = (*unstableAttr).Nlink
	(*newMeta).Size = (*unstableAttr).Size
	(*newMeta).Blksize = (*unstableAttr).Blksize
	(*newMeta).Blocks = (*unstableAttr).Blocks
	(*newMeta).X__pad0 = (*unstableAttr).X__pad0
	(*newMeta).X__unused = (*unstableAttr).X__unused

	return fs.OK
}

// Handle the passover or metadata in a duplicate scenario, where the underlying node is a hardlink, but we don't want it to appear as so
func MigrateDuplicateFileMetadata(oldMeta *MapEntryMetadata, newMeta *MapEntryMetadata, unstableAttr *syscall.Stat_t) syscall.Errno {
	// needs a write lock as we are modifying the metadata
	metadataMutex.Lock()
	defer metadataMutex.Unlock()

	// Old attributes to carry across
	(*newMeta).Path = (*oldMeta).Path
	(*newMeta).Mode = (*oldMeta).Mode
	(*newMeta).Ctim = (*oldMeta).Ctim
	(*newMeta).Uid = (*oldMeta).Uid
	(*newMeta).Gid = (*oldMeta).Gid
	(*newMeta).Dev = (*oldMeta).Dev
	(*newMeta).Atim = (*oldMeta).Atim
	(*newMeta).Rdev = (*oldMeta).Rdev
	(*newMeta).Nlink = (*oldMeta).Nlink
	(*newMeta).X__pad0 = (*oldMeta).X__pad0
	(*newMeta).X__unused = (*oldMeta).X__unused
	(*newMeta).XAttr = (*oldMeta).XAttr

	// Attributes to update from hardlink stat - not sure if we need more from the underlying hardlink?
	(*newMeta).Size = (*unstableAttr).Size
	(*newMeta).Blksize = (*unstableAttr).Blksize
	(*newMeta).Blocks = (*unstableAttr).Blocks

	return fs.OK

}

// Handle the creation of a metadata entry for a new duplicate file with no previous metadata entry
func InitialiseNewDuplicateFileMetadata(newMeta *MapEntryMetadata, spareUnstableAttr *syscall.Stat_t, linkUnstableAttr *syscall.Stat_t, path string, uid uint32, gid uint32) error {
	// needs a write lock as we are modifying the metadata
	metadataMutex.Lock()
	defer metadataMutex.Unlock()

	// Attributes to take from new file
	(*newMeta).Path = path
	(*newMeta).Mode = (*spareUnstableAttr).Mode
	(*newMeta).Atim = (*spareUnstableAttr).Atim
	(*newMeta).Mtim = (*spareUnstableAttr).Mtim
	(*newMeta).Ctim = (*spareUnstableAttr).Ctim
	(*newMeta).Uid = uid
	(*newMeta).Gid = gid
	(*newMeta).Dev = (*spareUnstableAttr).Dev
	(*newMeta).Atim = (*spareUnstableAttr).Atim
	(*newMeta).Rdev = (*spareUnstableAttr).Rdev
	(*newMeta).Nlink = (*spareUnstableAttr).Nlink
	(*newMeta).X__pad0 = (*spareUnstableAttr).X__pad0
	(*newMeta).X__unused = (*spareUnstableAttr).X__unused
	(*newMeta).XAttr = make(map[string][]byte)

	// Attributes to update from hardlink stat - not sure if we need more from the underlying hardlink?
	(*newMeta).Size = (*linkUnstableAttr).Size
	(*newMeta).Blksize = (*linkUnstableAttr).Blksize
	(*newMeta).Blocks = (*linkUnstableAttr).Blocks

	return nil
}

// Creates a new MapEntry in the main hash map when provided with a contentHash
// If the MapEntry already exists, we will simply pass back the already created MapEntry
func CreateRegularFileMapEntry(contentHash [64]byte) *MapEntry {
	metadataMutex.RLock()
	if entry, ok := regularFileMetadataHash[contentHash]; ok {
		metadataMutex.RUnlock() // unlock the process
		return entry
	}
	metadataMutex.RUnlock() // unlock the process

	// now we lock for writing, as we are creating a new entry
	metadataMutex.Lock()
	defer metadataMutex.Unlock()

	// Create the entry - it doesn't exist
	newEntry := &MapEntry{
		ReferenceCount:  0,
		EntryList:       make(map[uint64]*MapEntryMetadata),
		IndexCounter:    0,
		UnderlyingInode: 0,
	}

	// Place the new MapEntry inside the file hash
	regularFileMetadataHash[contentHash] = newEntry
	return newEntry
}

// Create a new createMapEntryMetadata struct (with default values) in the provided MapEntry.
// Returns the new createMapEntryMetadata along with the refNum to it.
func CreateRegularFileMetadata(entry *MapEntry) (refNum uint64, newEntry *MapEntryMetadata) {
	// lock for writing, as we are creating a new entry
	metadataMutex.Lock()
	defer metadataMutex.Unlock()

	// Check the current index number
	currentCounter := (*entry).IndexCounter
	// Create our new MapEntryMetadata (with default values)
	newEntry = &MapEntryMetadata{}
	// Place our MapEntryMetadata inside the MapEntry
	(*entry).EntryList[currentCounter+1] = newEntry
	// Increment the MapEntry counters
	(*entry).IndexCounter++
	(*entry).ReferenceCount++
	// Define the refNum attached to the MapEntryMetadata
	refNum = (*entry).IndexCounter

	return (*entry).IndexCounter, newEntry
}
