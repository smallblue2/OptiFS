// This file contains all relevant code for maintaining persistence between sessions of the OptiFSFile system

package metadata

import (
	"encoding/gob"
	"errors"
	"log"
	"os"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
)

var nodeMutex sync.RWMutex     // lock for hashmap saving node info
var metadataMutex sync.RWMutex // lock for hashmap saving custom metadata
var dirMutex sync.RWMutex      // lock for hashmap saving directory infor

// Stores the content hash and reference number for keeping a node persistent between OptiFS instances
func StoreRegFileInfo(path string, stableAttr *fs.StableAttr, mode uint32, contentHash [64]byte, refNum uint64) {
	// needs a write lock as we are modifying the hashmap
	nodeMutex.Lock()
	defer nodeMutex.Unlock()

	nodePersistenceHash[path] = &NodeInfo{StableGen: stableAttr.Gen, StableIno: stableAttr.Ino, StableMode: stableAttr.Mode, Mode: mode, IsDir: false, ContentHash: contentHash, RefNum: refNum}
}

// Specifically stores a directory into the persistence hash
func StoreDirInfo(path string, stableAttr *fs.StableAttr, mode uint32) {
	// needs a write lock as we are modifying the hashmap
	nodeMutex.Lock()
	defer nodeMutex.Unlock()

	nodePersistenceHash[path] = &NodeInfo{StableGen: stableAttr.Gen, StableIno: stableAttr.Ino, StableMode: stableAttr.Mode, Mode: mode, IsDir: true}
}

// Updates node info in the persistence hash, all values except the path are optional and won't be updated if nil
func UpdateNodeInfo(path string, isDir *bool, stableAttr *fs.StableAttr, mode *uint32, contentHash *[64]byte, refNum *uint64) {
	// needs a write lock as we are modifying the hashmap
	nodeMutex.Lock()
	defer nodeMutex.Unlock()

	log.Printf("Updating {%v}'s Persistent Data", path)
	store, ok := nodePersistenceHash[path]
	if !ok {
		log.Println("DOESNT EXIST!")
		return
	}
	if isDir != nil {
		log.Println("Setting IsDir")
		store.IsDir = *isDir
	}
	if stableAttr != nil {
		log.Println("Setting stableAttr")
		store.StableIno = stableAttr.Ino
		store.StableGen = stableAttr.Gen
		store.StableMode = stableAttr.Mode
	}
	if mode != nil {
		log.Println("Setting mode")
		store.Mode = *mode
	}
	if contentHash != nil {
		log.Println("Setting contentHash")
		store.ContentHash = *contentHash
	}
	if refNum != nil {
		log.Println("Setting refNum")
		store.RefNum = *refNum
	}
}

// Retrieves the content hash and reference number of a node in the nodePersistenceHash
func RetrieveNodeInfo(path string) (error, uint64, uint32, uint64, uint32, bool, [64]byte, uint64) {
	// needs a read lock as data is not being modified, only read, so multiple
	// operations can read at the same time (concurrently)
	nodeMutex.RLock()
	defer nodeMutex.RUnlock()

	log.Printf("Searching for {%v} in {%+v}\n", path, nodePersistenceHash)

	info, ok := nodePersistenceHash[path]
	if !ok {
		log.Println("Failed to retrieve node!")
		return errors.New("No node info available for path"), 0, 0, 0, 0, false, [64]byte{}, 0
	}
	log.Println("Retrieved node!")
	return nil, info.StableIno, info.StableMode, info.StableGen, info.Mode, info.IsDir, info.ContentHash, info.RefNum
}

// Removes an entry from the nodePersistenceHash
func RemoveNodeInfo(path string) error {
	// needs a write lock as we are modifying the hashmap
	nodeMutex.Lock()
	defer nodeMutex.Unlock()

	delete(nodePersistenceHash, path)
	return nil
}

// Function saves the regularFileMetadataHash
// Since a hashmap will be deleted when the system is restarted (stored in RAM),
// we encode the hashmap and store it in a file saved on disk to be loaded when OptiFS starts
func SaveMetadataMap(hashmap map[[64]byte]*MapEntry) error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	metadataMutex.Lock()
	defer metadataMutex.Unlock()

	log.Println("SAVING METADATA HASHMAP")
	dest := "hashing/OptiFSRegularFileMetadataSave.gob"

	// create the file if it doesn't exist, truncate it if it does
	// we assume nobody will be calling this file, as it is a very unique name
	file, err := os.Create(dest)

	if err != nil {
		log.Printf("ERROR WITH FILE - {%+v} - METADATA HASHMAP\n", err)
		return err
	}

	defer file.Close() // don't let the file close

	encode := gob.NewEncoder(file) // set the file that we created to the encoder
	eErr := encode.Encode(hashmap) // encode the hashmap into binary, put it in the file

	if eErr != nil {
		log.Printf("ERROR WITH ENCODER - {%+v} - METADATA HASHMAP\n", eErr)
		return eErr
	}

	return nil
}

// Retrieve the encoded hashmap from the file when the system restarts to maintain
// persistence between OptiFS instances
func RetrieveMetadataMap() error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	metadataMutex.Lock()
	defer metadataMutex.Unlock()

	log.Println("RETRIEVING METADATA HASHMAP")
	dest := "hashing/OptiFSRegularFileMetadataSave.gob"

	file, err := os.Open(dest) // open where the hashmap was encoded

	if err != nil {
		return err
	}

	defer file.Close() // don't let the file close

	decode := gob.NewDecoder(file)                  // set the file that we opened to the decoder
	dErr := decode.Decode(&regularFileMetadataHash) // decode the file back into the hashmap

	if dErr != nil {
		log.Printf("ERROR WITH DECODER - {%+v} - METADATA HASHMAP\n", dErr)
		return dErr
	}

	return nil
}

// Function saves the node persistence hash into a Go binary (.gob) file
// since a hashmap will be deleted when the system is restarted (stored in RAM)
// we encode the hashmap and store it in a file saved on disk to be loaded when OptiFS starts
func SaveNodePersistenceHash(hashmap map[string]*NodeInfo) error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	nodeMutex.Lock()
	defer nodeMutex.Unlock()

	log.Println("SAVING NODE INFO HASHMAP")
	dest := "hashing/OptiFSNodePersistenceSave.gob"

	// create the file if it doesn't exist, truncate it if it does
	// we assume nobody will be calling this file, as it is a very unique name
	file, err := os.Create(dest)

	if err != nil {
		log.Printf("ERROR WITH FILE - {%+v} - NODE HASHMAP\n", err)
		return err
	}

	defer file.Close() // don't let the file close

	encode := gob.NewEncoder(file) // set the file that we created to the encoder
	eErr := encode.Encode(hashmap) // encode the hashmap into binary, put it in the file

	if eErr != nil {
		log.Printf("ERROR WITH ENCODER - {%+v} - NODE HASHMAP\n", eErr)
		return eErr
	}

	log.Println("Saved succesfully!")

	return nil
}

// retrieve the encoded node info hashmap from the file when the system restarts
func RetrieveNodePersistenceHash() error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	nodeMutex.Lock()
	defer nodeMutex.Unlock()

	log.Println("RETRIEVING NODE HASHMAP")
	dest := "hashing/OptiFSNodePersistenceSave.gob"

	file, err := os.Open(dest) // open where the hashmap was encoded

	if err != nil {
		return err
	}

	defer file.Close() // don't let the file close

	decode := gob.NewDecoder(file)              // set the file that we opened to the decoder
	dErr := decode.Decode(&nodePersistenceHash) // decode the file back into the hashmap

	if dErr != nil {
		log.Printf("ERROR WITH DECODER - {%+v} - NODE HASHMAP", dErr)
		return dErr
	}

	return nil
}

// save the hashmap which stores information about directories in the filesystem
// when the filesystem restarts
func SaveDirMetadataHash(hashmap map[string]*MapEntryMetadata) error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	dirMutex.Lock()
	defer dirMutex.Unlock()

	log.Println("SAVING DIR HASHMAP")
	dest := "hashing/OptiFSDirMetadataSave.gob"

	// create the file if it doesn't exist, truncate it if it does
	// we assume nobody will be calling this file, as it is a very unique name
	file, err := os.Create(dest)

	if err != nil {
		log.Printf("ERROR WITH FILE - {%+v} - DIR HASHMAP\n", err)
		return err
	}

	defer file.Close() // don't let the file close

	encode := gob.NewEncoder(file) // set the file that we created to the encoder
	eErr := encode.Encode(hashmap) // encode the hashmap into binary, put it in the file

	if eErr != nil {
		log.Printf("ERROR WITH ENCODER - {%+v} - DIR HASHMAP\n", eErr)
		return eErr
	}

	return nil

}

// retrieve the hashmap which stores information about directories in the filesystem
// when the filesystem restarts
func RetrieveDirMetadataHash() error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	dirMutex.Lock()
	defer dirMutex.Unlock()

	log.Println("RETRIEVING DIR HASHMAP")
	dest := "hashing/OptiFSDirMetadataSave.gob"

	file, err := os.Open(dest) // open where the hashmap was encoded

	if err != nil {
		return err
	}

	defer file.Close() // don't let the file close

	decode := gob.NewDecoder(file)          // set the file that we opened to the decoder
	dErr := decode.Decode(&dirMetadataHash) // decode the file back into the hashmap

	if dErr != nil {
		log.Printf("ERROR WITH DECODER - {%+v} - DIR HASHMAP\n", dErr)
		return dErr
	}

	return nil

}

// for saving the hashmaps when system is shut off
// preserves private hashmaps
func SavePersistantStorage() {
	SaveNodePersistenceHash(nodePersistenceHash)
	SaveMetadataMap(regularFileMetadataHash)
	SaveDirMetadataHash(dirMetadataHash)
}

// for actually loading the hashmaps when the system is turned on
// preserves private hashmaps
func RetrievePersistantStorage() {
	RetrieveNodePersistenceHash()
	RetrieveMetadataMap()
	RetrieveDirMetadataHash()
}

// Printing the regularFileMetadataHash for testing purposes
func PrintRegularFileMetadataHash() {
	log.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
	log.Println("PRINTING FILE METADATA HASHMAP")
	for key, value := range regularFileMetadataHash {
		log.Printf("Key: %x, Value: %v\n", key, value)
	}
	log.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
}

// Printing the dirMetadataHash for testing purposes
func PrintDirMetadataHash() {
	log.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
	log.Println("PRINTING DIR METADATA HASHMAP")
	for key, value := range dirMetadataHash {
		log.Printf("Key: %x, Value: %v\n", key, value)
	}
	log.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
}

// Printing the nodePersistenceHash for testing purposes
func PrintNodePersistenceHash() {
	log.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
	log.Println("PRINTING NODE METADATA HASHMAP")
	for key, value := range nodePersistenceHash {
		log.Printf("Key: %v, Value: %v\n", key, value)
	}
	log.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
}

// Ensure integrity
//
// Function looks through all retrieved hashmaps and ensures all their entries align with
// data in the underlying filesystem
func InsureIntegrity() {
	// Collect any miss-entries
	pathsToDelete := []struct {
		path  string
		isDir bool
        hash [64]byte
        ref uint64
	}{}

	for path, nodeInfo := range nodePersistenceHash {
		// check to see that the node exists
		var st syscall.Stat_t
		err := syscall.Stat(path, &st)
		if err != nil {
			// if there is an error, delete the entry
			log.Printf("INTEGRITY ERROR: {%v} - {%v}\n", path, err)
			pathsToDelete = append(pathsToDelete, struct {
				path  string
				isDir bool
				hash  [64]byte
				ref   uint64
			}{path, nodeInfo.IsDir, nodeInfo.ContentHash, nodeInfo.RefNum})
		}
	}

	// Cleanup
	for index := range pathsToDelete {
		path := pathsToDelete[index].path
		isDir := pathsToDelete[index].isDir
        hash := pathsToDelete[index].hash
        ref := pathsToDelete[index].ref

		// Remove from relevant metadata struct
		if isDir {
            RemoveDirEntry(path)
			log.Printf("Removed custom metadata for {%v} directory.\n", path)
		} else {
            RemoveRegularFileMetadata(hash, ref)
            log.Printf("Removed custom metadata for {%v} file.\n", path)
		}

        // Remove from persisten store
        RemoveNodeInfo(path)
        log.Printf("Removed {%v} from persistent store\n", path)
	}

    log.Println("FILESYSTEM HEALTHY")
}
