// This file contains all public functions for external modules to communicate with the regularFileMetadataHash

package metadata

import (
	"context"
	"errors"
	"log"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Performs a lookup on the regularFileMetadataHash to tell if the provided content hash is unique.
//
// Returns a bool for whether the contentHash can be found and also returns the underlying Inode
func IsContentHashUnique(contentHash [64]byte) (bool, uint32) {
	// needs a read lock as data is not being modified, only read, so multiple
	// operations can read at the same time (concurrently)
	metadataMutex.RLock()
	defer metadataMutex.RUnlock()

	// Check to see if there's an entry for the contentHash and refNum above
	entry, exists := regularFileMetadataHash[contentHash]
	// If it doesn't exist
	if !exists {
		log.Println("Content is unique!")
		return !exists, 0 // TODO: return the underlying node OR get rid of it
	}

	// If it exists, return the underlying Inode
	log.Println("Content isn't unique!")
	return !exists, entry.UnderlyingInode
}

// Gets the most recent entry in a MapEntry
// Returns nil if there is no entry
func RetrieveRecent(entry *MapEntry) *MapEntryMetadata {
    indx := entry.IndexCounter
    // If it's empty, return nothing
    if indx == 0 {
        return nil
    }
    for indx := entry.IndexCounter; indx >= 0; indx-- {
        meta, ok := entry.EntryList[indx]
        if ok {
            log.Printf("Found metadata at {%v}\n", indx)
            return meta
        }
        log.Printf("Nothing at {%v}\n", indx)
    }
    log.Println("Couldn't find an entry!")
    return nil
}

// Retrieves regular file metadata for a hash and refnum provided. Returns an error if it cannot be found
func LookupRegularFileMetadata(contentHash [64]byte, refNum uint64) (error, *MapEntryMetadata) {
	// needs a read lock as data is not being modified, only read, so multiple
	// operations can read at the same time (concurrently)
	metadataMutex.RLock()
	defer metadataMutex.RUnlock()

	log.Println("Looking up a contentHash and refNum...")

	// First check for default values
	var defaultByteArray [64]byte
	if contentHash == defaultByteArray || refNum == 0 {
		log.Println("Default values detected, no MapEntryMetadata available")
		return errors.New("Default values detected"), &MapEntryMetadata{}
	}

	// Now actually query the hashmap
	if contentEntry, ok := regularFileMetadataHash[contentHash]; ok {
		if nodeMetadata, ok := contentEntry.EntryList[refNum]; ok {
			return nil, nodeMetadata
		}
	}
	log.Println("contentHash and refNum didn't lead to MapEntryMetadata")
	return errors.New("Couldn't find entry!"), &MapEntryMetadata{}
}

// Retrieves a MapEntry instance from regularFileMetadataHash using the content hash provided.
//
// Returns the retrived MapEntry, or an error if it doesn't exist
func LookupRegularFileEntry(contentHash [64]byte) (error, *MapEntry) {
	// needs a read lock as data is not being modified, only read, so multiple
	// operations can read at the same time (concurrently)
	metadataMutex.RLock()
	defer metadataMutex.RUnlock()

	entry, ok := regularFileMetadataHash[contentHash]
	if !ok {
		return errors.New("Entry doesn't exist!"), nil
	}

	return nil, entry
}

// Removes a MapEntryMetadata instance in regularFileMetadataHash based on content hash and refnum provided.
// Also handles if this potentially creates an empty MapEntry struct.
func RemoveRegularFileMetadata(contentHash [64]byte, refNum uint64) error {
	log.Printf("Removing Metadata for refNum{%v}, contentHash{%+v}\n", refNum, contentHash)

	// Check to see if an entry exists
	err, entry, _ := RetrieveRegularFileMapEntryAndMetadataFromHashAndRef(contentHash, refNum)
	if err != nil {
		log.Println("Couldn't find an entry!")
		return err
	}
	log.Println("Found an entry!")


	metadataMutex.Lock()
	// Delete the metadata from our entry
	delete(entry.EntryList, refNum)
	// Reflect these changes in the MapEntry
	entry.ReferenceCount--
    metadataMutex.Unlock()

	log.Println("Deleted metadata, checking to see if we need to delete the MapEntry")

	// Check to see if the MapEntry is empty
	if entry.ReferenceCount == 0 {
		// If it is, delete the whole entry
        metadataMutex.Lock()
		delete(regularFileMetadataHash, contentHash)
        metadataMutex.Unlock()
		log.Println("Deleted MapEntry")
	}
	log.Println("Finished removing metadata")

	return nil
}

// Retrieves the MapEntry struct from which the Metadata entry struct that the refNum and contentHash links to
func RetrieveRegularFileMapEntryFromHashAndRef(contentHash [64]byte, refNum uint64) (error, *MapEntry) {
	// needs a read lock as data is not being modified, only read, so multiple
	// operations can read at the same time (concurrently)
	metadataMutex.RLock()
	defer metadataMutex.RUnlock()

	log.Println("Looking up MapEntry from Hash and Ref")

	// First check for default values
	var defaultByteArray [64]byte
	if contentHash == defaultByteArray || refNum == 0 {
		log.Println("Default values detected, no MapEntry available")
		return errors.New("Default values detected"), &MapEntry{}
	}

	// Now actually query the hashmap
	if contentEntry, ok := regularFileMetadataHash[contentHash]; ok {
		if _, ok := contentEntry.EntryList[refNum]; ok {
			log.Println("Found a MapEntry for valid hash and refnum!")
			return nil, contentEntry
		}
	}

	log.Println("Couldn't find a MapEntry for provided hash and refnum")
	return errors.New("Couldn't find entry!"), &MapEntry{}
}

// Retrieves the MapEntry and MapEntryMetadata struct from which the reference num and content hash links to
func RetrieveRegularFileMapEntryAndMetadataFromHashAndRef(contentHash [64]byte, refNum uint64) (error, *MapEntry, *MapEntryMetadata) {

	log.Println("Looking up MapEntry and MapEntryMetadata from Hash and Ref")

	// First check for default values
	var defaultByteArray [64]byte
	if contentHash == defaultByteArray || refNum == 0 {
		log.Println("Default values detected, no MapEntry or MapEntryData available")
		return errors.New("Default values detected"), &MapEntry{}, &MapEntryMetadata{}
	}

	// needs a read lock as data is not being modified, only read, so multiple
	// operations can read at the same time (concurrently)
	metadataMutex.RLock()
	defer metadataMutex.RUnlock()

	// Now actually query the hashmap
	if contentEntry, ok := regularFileMetadataHash[contentHash]; ok {
		if metadataEntry, ok := contentEntry.EntryList[refNum]; ok {
			log.Println("Found a MapEntry and MapEntryMetadata for valid hash and refnum!")
			return nil, contentEntry, metadataEntry
		}
	}

	log.Println("Couldn't find a MapEntry and MapEntryMetadata for provided hash and refnum")
	return errors.New("Couldn't find entry!"), &MapEntry{}, &MapEntryMetadata{}
}

// Updates a MapEntryMetadata instance corresponding to the content hash and reference num provided
//
// If refNum or contentHash is invalid, it returns an error
func UpdateFullRegularFileMetadata(contentHash [64]byte, refNum uint64, unstableAttr *syscall.Stat_t, stableAttr *fs.StableAttr, path string) error {

	// locks for this function are implemented in the functions being called
	// done to prevent deadlock
	log.Println("Updating metadata through lookup...")
	// Ensure that contentHash and refNum is valid
	err, metadata := LookupRegularFileMetadata(contentHash, refNum)
	if err != nil {
		log.Println("Couldn't find the metadata struct")
		return err
	}
	log.Println("Found the metadata struct")

	log.Printf("unstableAttr: %+v\n", unstableAttr)

	// Now we can be sure the entry exists, let's update it
    updateAllFromStat(metadata, unstableAttr, stableAttr, path)

	log.Printf("metadata: %+v\n", metadata)
	log.Println("Updated all custom metadata attributes through lookup")

	return nil
}

// Moves old metadata to a new node being created
func MigrateRegularFileMetadata(oldMeta *MapEntryMetadata, newMeta *MapEntryMetadata, unstableAttr *syscall.Stat_t) error {
	// needs a write lock as we are modifying the metadata
	metadataMutex.Lock()
	defer metadataMutex.Unlock()

	log.Println("Migrating old metadata across to new entry.")

	// Old attributes to carry across
    (*newMeta).Path = (*oldMeta).Path
	(*newMeta).Mode = (*oldMeta).Mode
	(*newMeta).Ctim = (*oldMeta).Ctim
	(*newMeta).Uid = (*oldMeta).Uid
	(*newMeta).Gid = (*oldMeta).Gid
	(*newMeta).Dev = (*oldMeta).Dev
	(*newMeta).Ino = (*oldMeta).Ino

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

	return nil
}

// Handle the passover or metadata in a duplicate scenario, where the underlying node is a hardlink, but we don't want it to appear as so
func MigrateDuplicateFileMetadata(oldMeta *MapEntryMetadata, newMeta *MapEntryMetadata, unstableAttr *syscall.Stat_t) error {

    log.Println("Migrating old metadata across to new entry for duplicate file")


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

    // Attributes to update from hardlink stat - not sure if we need more from the underlying hardlink?
	(*newMeta).Size = (*unstableAttr).Size
	(*newMeta).Blksize = (*unstableAttr).Blksize
	(*newMeta).Blocks = (*unstableAttr).Blocks

    return nil

}

// Handle the creation of a metadata entry for a new duplicate file with no previous metadata entry
func InitialiseNewDuplicateFileMetadata(ctx context.Context, newMeta *MapEntryMetadata, spareUnstableAttr *syscall.Stat_t, linkUnstableAttr *syscall.Stat_t, path string) error {

    caller, check := fuse.FromContext(ctx)
    if !check {
        log.Println("No caller info available")
        return errors.New("No caller info available")
    }
    uid, gid := uint32(caller.Uid), uint32(caller.Gid)

    // Old attributes to carry across
    (*newMeta).Path = path
    (*newMeta).Mode = (*spareUnstableAttr).Mode
    (*newMeta).Ctim = (*spareUnstableAttr).Ctim
    (*newMeta).Uid = uid
    (*newMeta).Gid = gid
    (*newMeta).Dev = (*spareUnstableAttr).Dev
    (*newMeta).Atim = (*spareUnstableAttr).Atim
    (*newMeta).Rdev = (*spareUnstableAttr).Rdev
	(*newMeta).Nlink = (*spareUnstableAttr).Nlink
	(*newMeta).X__pad0 = (*spareUnstableAttr).X__pad0
	(*newMeta).X__unused = (*spareUnstableAttr).X__unused

    // Attributes to update from hardlink stat - not sure if we need more from the underlying hardlink?
	(*newMeta).Size = (*linkUnstableAttr).Size
	(*newMeta).Blksize = (*linkUnstableAttr).Blksize
	(*newMeta).Blocks = (*linkUnstableAttr).Blocks

    return nil
}

// Creates a new MapEntry in the main hash map when provided with a contentHash
// If the MapEntry already exists, we will simply pass back the already created MapEntry
func CreateRegularFileMapEntry(contentHash [64]byte) *MapEntry {
	// read lock for reading the hashmap
	metadataMutex.RLock()
	if entry, ok := regularFileMetadataHash[contentHash]; ok {
		metadataMutex.RUnlock() // unlock the process
		log.Println("MapEntry already exists, returning it")
		return entry
	}
	metadataMutex.RUnlock() // unlock the process

	// now we lock for writing, as we are creating a new entry
	metadataMutex.Lock()
	defer metadataMutex.Unlock()

	log.Println("Creating a new MapEntry")
	// Create the entry - it doesn't exist
	newEntry := &MapEntry{
		ReferenceCount:  0,
		EntryList:       make(map[uint64]*MapEntryMetadata),
		IndexCounter:    0,
		UnderlyingInode: 0,
	}

	// TODO: Get the underlying inode

	log.Println("Placing MapEntry in FileHashes")

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

	log.Println("Creating a new MapEntryMetadata")

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

	log.Printf("New MapEntryMetadata struct at refNum{%v}\n", refNum)

	return (*entry).IndexCounter, newEntry
}
