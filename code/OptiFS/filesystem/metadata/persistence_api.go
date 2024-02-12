// This file contains all relevant code for maintaining persistence between sessions of the OptiFSFile system

package metadata

import (
	"encoding/gob"
	"errors"
	"log"
	"os"
	"sync"

	"github.com/hanwen/go-fuse/v2/fs"
)

var nodeMutex sync.Mutex // lock for hashmap saving node info

// Stores the content hash and reference number for keeping a node persistent between OptiFS instances
func StoreRegFileInfo(path string, stableAttr *fs.StableAttr, mode uint32, contentHash [64]byte, refNum uint64) {
	nodePersistenceHash[path] = &NodeInfo{stableAttr: *stableAttr, mode: mode, isDir: false, contentHash: contentHash, refNum: refNum}
}

// Specifically stores a directory into the persistence hash
func StoreDirInfo(path string, stableAttr *fs.StableAttr, mode uint32) {
	nodePersistenceHash[path] = &NodeInfo{stableAttr: *stableAttr, mode: mode, isDir: true}
}

// Updates node info in the persistence hash, all values except the path are optional and won't be updated if nil
func UpdateNodeInfo(path string, isDir *bool, stableAttr *fs.StableAttr, mode *uint32, contentHash *[64]byte, refNum *uint64) {
	log.Printf("Updating {%v}'s Persistent Data", path)
	store, ok := nodePersistenceHash[path]
	if !ok {
		log.Println("DOESNT EXIST!")
		return
	}
	if isDir != nil {
		log.Println("Setting IsDir")
		store.isDir = *isDir
	}
	if stableAttr != nil {
		log.Println("Setting stableAttr")
		store.stableAttr = *stableAttr
	}
	if mode != nil {
		log.Println("Setting mode")
		store.mode = *mode
	}
	if contentHash != nil {
		log.Println("Setting contentHash")
		store.contentHash = *contentHash
	}
	if refNum != nil {
		log.Println("Setting refNum")
		store.refNum = *refNum
	}
}

// Retrieves the content hash and reference number of a node in the nodePersistenceHash
func RetrieveNodeInfo(path string) (error, *fs.StableAttr, uint32, bool, [64]byte, uint64) {
	info, ok := nodePersistenceHash[path]
	if !ok {
		return errors.New("No node info available for path"), &fs.StableAttr{}, 0, false, [64]byte{}, 0
	}

	return nil, &info.stableAttr, info.mode, info.isDir, info.contentHash, info.refNum
}

// Removes an entry from the nodePersistenceHash
func RemoveNodeInfo(path string) error {
	delete(nodePersistenceHash, path)
	return nil
}

// Function saves the regularFileMetadataHash
// Since a hashmap will be deleted when the system is restarted (stored in RAM),
// we encode the hashmap and store it in a file saved on disk to be loaded when OptiFS starts
func SaveMetadataMap(hashmap map[[64]byte]*MapEntry) error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	nodeMutex.Lock()
	defer nodeMutex.Unlock()

	log.Println("SAVING METADATA HASHMAP")
	dest := "hashing/OptiFSRegularFileMetadataSave.gob"

	// create the file if it doesn't exist, truncate it if it does
	// we assume nobody will be calling this file, as it is a very unique name
	file, err := os.Create(dest)

	if err != nil {
		log.Println("ERROR WITH FILE - METADATA HASHMAP")
		return err
	}

	defer file.Close() // don't let the file close

	encode := gob.NewEncoder(file) // set the file that we created to the encoder
	eErr := encode.Encode(hashmap) // encode the hashmap into binary, put it in the file

	if eErr != nil {
		log.Println("ERROR WITH ENCODER - METADATA HASHMAP")
		return eErr
	}

	return nil
}

// Retrieve the encoded hashmap from the file when the system restarts to maintain
// persistence between OptiFS instances
func RetrieveMetadataMap() error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	nodeMutex.Lock()
	defer nodeMutex.Unlock()

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
		log.Println("ERROR WITH DECODER - METADATA HASHMAP")
		return dErr
	}

	return nil
}

// Printing the regularFileMetadataHash for testing purposes
func PrintRegularFileMetadataHash() {
	log.Println("PRINTING METADATA HASHMAP")
	for key, value := range regularFileMetadataHash {
		log.Printf("Key: %x, Value: %v\n", key, value)
	}
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
		log.Println("ERROR WITH FILE - NODE HASHMAP")
		return err
	}

	defer file.Close() // don't let the file close

	encode := gob.NewEncoder(file) // set the file that we created to the encoder
	eErr := encode.Encode(hashmap) // encode the hashmap into binary, put it in the file

	if eErr != nil {
		log.Println("ERROR WITH ENCODER - NODE HASHMAP")
		return eErr
	}

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
		log.Println("ERROR WITH DECODER - NODE HASHMAP")
		return dErr
	}

	return nil
}

// save the hashmap which stores information about directories in the filesystem
// when the filesystem restarts
func SaveDirMetadataHash(hashmap map[string]*MapEntryMetadata) error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	nodeMutex.Lock()
	defer nodeMutex.Unlock()

	log.Println("SAVING DIR HASHMAP")
	dest := "hashing/OptiFSDirMetadataSave.gob"

	// create the file if it doesn't exist, truncate it if it does
	// we assume nobody will be calling this file, as it is a very unique name
	file, err := os.Create(dest)

	if err != nil {
		log.Println("ERROR WITH FILE - DIR HASHMAP")
		return err
	}

	defer file.Close() // don't let the file close

	encode := gob.NewEncoder(file) // set the file that we created to the encoder
	eErr := encode.Encode(hashmap) // encode the hashmap into binary, put it in the file

	if eErr != nil {
		log.Println("ERROR WITH ENCODER - DIR HASHMAP")
		return eErr
	}

	return nil

}

// retrieve the hashmap which stores information about directories in the filesystem
// when the filesystem restarts
func RetrieveDirMetadataHash() error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	nodeMutex.Lock()
	defer nodeMutex.Unlock()

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
		log.Println("ERROR WITH DECODER - DIR HASHMAP")
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
