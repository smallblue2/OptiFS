// This file contains all public functions for external modules to communicate with the directoryMetadataHash

package metadata

import (
	"errors"
	"log"
	"syscall"
)

// Creates a default directory entry in the directoryMetadataHash
func CreateDirEntry(ino uint64) *MapEntryMetadata {
    log.Printf("Created a new directory metadata entry for (%v)\n", ino)
    dirMetadataHash[ino] = &MapEntryMetadata{}
    return dirMetadataHash[ino]
}

// Performs a lookup for a directory entry in the directoryMetadataHash with
// the 'ino' being the key
func LookupDirMetadata(ino uint64) (error, *MapEntryMetadata) {
    log.Printf("Looking up metadata for dir (%v)\n", ino)
    metadata, ok := dirMetadataHash[ino]
    if !ok {
        log.Println("Couldn't find a custom directory metadata entry")
        return errors.New("No metadata entry available!"), nil
    }
        log.Println("Found a custom directory metadata entry")
    return nil, metadata
}

// Updates an entry in the directoryMetadataHash with the full contents of the provided
// Stat_t object. Will error if there exists no entry for the provided ino.
func UpdateDirEntry(ino uint64, unstableAttr *syscall.Stat_t) error {

	log.Println("Updating dir metadata through lookup...")
	// Ensure that contentHash and refNum is valid
	err, metadata := LookupDirMetadata(ino)
	if err != nil {
		log.Println("Couldn't find the metadata struct")
		return err
	}
	log.Println("Found the metadata struct")

	// Now we can be sure the entry exists, let's update it
    updateAllFromStat(metadata, unstableAttr)

	log.Printf("metadata: %+v\n", metadata)
	log.Println("Updated all custom metadata attributes through lookup")

	return nil
}

// Deletes an entry in the directoryMetadataHash with the ino provided.
// Provides no response, function deletes if the entry is there, does nothing
// if it is not.
func RemoveDirEntry(ino uint64) {
    delete(dirMetadataHash, ino)
}
