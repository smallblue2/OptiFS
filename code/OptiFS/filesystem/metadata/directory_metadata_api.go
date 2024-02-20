// This file contains all public functions for external modules to communicate with the directoryMetadataHash

package metadata

import (
	"log"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
)

// Creates a default directory entry in the directoryMetadataHash
func CreateDirEntry(path string) *MapEntryMetadata {
	// needs a write lock as we are modifying the hashmap
	dirMutex.Lock()
	defer dirMutex.Unlock()

	log.Printf("Created a new directory metadata entry for (%v)\n", path)
    entry := &MapEntryMetadata{XAttr: make(map[string][]byte)}
	dirMetadataHash[path] = entry
	return entry
}

// Performs a lookup for a directory entry in the directoryMetadataHash with
// the path being the key
func LookupDirMetadata(path string) (syscall.Errno, *MapEntryMetadata) {
    log.Println("Entered DIRECTORY metadata lookup!")
	// needs a read lock as data is not being modified, only read, so multiple
	// operations can read at the same time (concurrently)
	dirMutex.RLock()
	defer dirMutex.RUnlock()
    log.Println("Got lock")

	log.Printf("Looking up metadata for dir (%v)\n", path)
	metadata, ok := dirMetadataHash[path]
	if !ok {
		log.Println("Couldn't find a custom directory metadata entry")
		return fs.ToErrno(syscall.ENODATA), nil
	}
	log.Println("Found a custom directory metadata entry")
	return 0, metadata
}

// Updates an entry in the directoryMetadataHash with the full contents of the provided
// Stat_t object. Will error if there exists no entry for the provided ino.
func UpdateDirEntry(path string, unstableAttr *syscall.Stat_t, stableAttr *fs.StableAttr) syscall.Errno {
	// locks for this function are implemented in the functions being called
	// done to prevent deadlock

	log.Println("Updating dir metadata through lookup...")
	// Ensure that contentHash and refNum is valid
	// this function has locks already!
	err, metadata := LookupDirMetadata(path)
	if err != 0 {
		log.Println("Couldn't find the metadata struct")
		return err
	}
	log.Println("Found the metadata struct")

	// Now we can be sure the entry exists, let's update it
    updateAllFromStat(metadata, unstableAttr, stableAttr, path)

	log.Printf("metadata: %+v\n", metadata)
	log.Println("Updated all custom metadata attributes through lookup")

	return 0
}

// Deletes an entry in the directoryMetadataHash with the ino provided.
// Provides no response, function deletes if the entry is there, does nothing
// if it is not.
func RemoveDirEntry(path string) {
	// needs a write lock as we are modifying the hashmap (deleting an entry)
	dirMutex.Lock()
	defer dirMutex.Unlock()

	delete(dirMetadataHash, path)
}
