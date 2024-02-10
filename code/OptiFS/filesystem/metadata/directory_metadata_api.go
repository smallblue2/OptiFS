// This file contains all public functions for external modules to communicate with the directoryMetadataHash

package metadata

import (
	"errors"
	"log"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
)

// Creates a default directory entry in the directoryMetadataHash
func CreateDirEntry(path string) *MapEntryMetadata {
    log.Printf("Created a new directory metadata entry for (%v)\n", path)
    dirMetadataHash[path] = &MapEntryMetadata{}
    return dirMetadataHash[path]
}

// Performs a lookup for a directory entry in the directoryMetadataHash with
// the 'ino' being the key
func LookupDirMetadata(path string) (error, *MapEntryMetadata) {
    log.Printf("Looking up metadata for dir (%v)\n", path)
    metadata, ok := dirMetadataHash[path]
    if !ok {
        log.Println("Couldn't find a custom directory metadata entry")
        return errors.New("No metadata entry available!"), nil
    }
        log.Println("Found a custom directory metadata entry")
    return nil, metadata
}

// Updates an entry in the directoryMetadataHash with the full contents of the provided
// Stat_t object. Will error if there exists no entry for the provided ino.
func UpdateDirEntry(path string, unstableAttr *syscall.Stat_t, stableAttr *fs.StableAttr) error {

	log.Println("Updating dir metadata through lookup...")
	// Ensure that contentHash and refNum is valid
	err, metadata := LookupDirMetadata(path)
	if err != nil {
		log.Println("Couldn't find the metadata struct")
		return err
	}
	log.Println("Found the metadata struct")

	// Now we can be sure the entry exists, let's update it
    updateAllFromStat(metadata, unstableAttr, stableAttr)

	log.Printf("metadata: %+v\n", metadata)
	log.Println("Updated all custom metadata attributes through lookup")

	return nil
}

// Deletes an entry in the directoryMetadataHash with the ino provided.
// Provides no response, function deletes if the entry is there, does nothing
// if it is not.
func RemoveDirEntry(path string) {
    delete(dirMetadataHash, path)
}
