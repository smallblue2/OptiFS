// This file contains all relevant code for maintaining persistence between sessions of the OptiFSFile system

package metadata

import (
	"encoding/gob"
	"log"
	"os"
	"sync"
	"syscall"
	"time"

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
		store.IsDir = *isDir
	}
	if stableAttr != nil {
		store.StableIno = stableAttr.Ino
		store.StableGen = stableAttr.Gen
		store.StableMode = stableAttr.Mode
	}
	if mode != nil {
		store.Mode = *mode
	}
	if contentHash != nil {
		store.ContentHash = *contentHash
	}
	if refNum != nil {
		store.RefNum = *refNum
	}
}

// Retrieves the content hash and reference number of a node in the nodePersistenceHash
func RetrieveNodeInfo(path string) (syscall.Errno, uint64, uint32, uint64, uint32, bool, [64]byte, uint64) {
	// needs a read lock as data is not being modified, only read, so multiple
	// operations can read at the same time (concurrently)
	nodeMutex.RLock()
	defer nodeMutex.RUnlock()

	info, ok := nodePersistenceHash[path]
	if !ok {
		log.Println("Failed to retrieve node!")
		return fs.ToErrno(syscall.ENODATA), 0, 0, 0, 0, false, [64]byte{}, 0
	}

	log.Printf("Retrieved persistent data for {%v}.\n", path)

	return fs.OK, info.StableIno, info.StableMode, info.StableGen, info.Mode, info.IsDir, info.ContentHash, info.RefNum
}

// Removes an entry from the nodePersistenceHash
func RemoveNodeInfo(path string) error {
	// needs a write lock as we are modifying the hashmap
	nodeMutex.Lock()
	defer nodeMutex.Unlock()

	delete(nodePersistenceHash, path)

    log.Printf("Removed persistent data for {%v}.\n", path)

	return nil
}

// Function saves the regularFileMetadataHash
// Since a hashmap will be deleted when the system is restarted (stored in RAM),
// we encode the hashmap and store it in a file saved on disk to be loaded when OptiFS starts
func SaveMetadataMap(hashmap map[[64]byte]*MapEntry, dest string) error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	metadataMutex.Lock()
	defer metadataMutex.Unlock()

    log.Println("Saving regular file metadata hash map")

	// create the file if it doesn't exist, truncate it if it does
	// we assume nobody will be calling this file, as it is a very unique name
	file, err := os.Create(dest + "/OptiFSRegularFileMetadataSave.gob")

	if err != nil {
        log.Println("Failed to create save.")
		return err
	}

	defer file.Close() // don't let the file close

	encode := gob.NewEncoder(file) // set the file that we created to the encoder
	eErr := encode.Encode(hashmap) // encode the hashmap into binary, put it in the file

	if eErr != nil {
        log.Println("Failed to encode.")
		return eErr
	}

	log.Println("Saved succesfully.")

	return nil
}

// Retrieve the encoded hashmap from the file when the system restarts to maintain
// persistence between OptiFS instances
func RetrieveMetadataMap(dest string) error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	metadataMutex.Lock()
	defer metadataMutex.Unlock()

    log.Println("Retrieving regular file metadata hash map")

	file, err := os.Open(dest + "/OptiFSRegularFileMetadataSave.gob") // open where the hashmap was encoded

	if err != nil {
        log.Println("Doesn't exist.")
		return err
	}

	defer file.Close() // don't let the file close

	decode := gob.NewDecoder(file)                  // set the file that we opened to the decoder
	dErr := decode.Decode(&regularFileMetadataHash) // decode the file back into the hashmap

	if dErr != nil {
        log.Println("Failed to decode.")
		return dErr
	}

	log.Println("Retrieved succesfully.")

	return nil
}

// Function saves the node persistence hash into a Go binary (.gob) file
// since a hashmap will be deleted when the system is restarted (stored in RAM)
// we encode the hashmap and store it in a file saved on disk to be loaded when OptiFS starts
func SaveNodePersistenceHash(hashmap map[string]*NodeInfo, dest string) error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	nodeMutex.Lock()
	defer nodeMutex.Unlock()

    log.Println("Saving persistence metadata hash map")

	// create the file if it doesn't exist, truncate it if it does
	// we assume nobody will be calling this file, as it is a very unique name
	file, err := os.Create(dest + "/OptiFSNodePersistenceSave.gob")

	if err != nil {
        log.Println("Doesn't exist.")
		return err
	}

	defer file.Close() // don't let the file close

	encode := gob.NewEncoder(file) // set the file that we created to the encoder
	eErr := encode.Encode(hashmap) // encode the hashmap into binary, put it in the file

	if eErr != nil {
		log.Println("Failed to encode.")
		return eErr
	}

	log.Println("Saved succesfully.")

	return nil
}

// retrieve the encoded node info hashmap from the file when the system restarts
func RetrieveNodePersistenceHash(dest string) error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	nodeMutex.Lock()
	defer nodeMutex.Unlock()

	log.Println("Retrieving persistence metadata hash map")

	file, err := os.Open(dest + "/OptiFSNodePersistenceSave.gob") // open where the hashmap was encoded

	if err != nil {
        log.Println("Doesn't exist.")
		return err
	}

	defer file.Close() // don't let the file close

	decode := gob.NewDecoder(file)              // set the file that we opened to the decoder
	dErr := decode.Decode(&nodePersistenceHash) // decode the file back into the hashmap

	if dErr != nil {
		log.Println("Failed to decode.")
		return dErr
	}

    log.Println("Retrieved succesfully.")

	return nil
}

// save the hashmap which stores information about directories in the filesystem
// when the filesystem restarts
func SaveDirMetadataHash(hashmap map[string]*MapEntryMetadata, dest string) error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	dirMutex.Lock()
	defer dirMutex.Unlock()

	log.Println("Saving directory metadata hash map")

	// create the file if it doesn't exist, truncate it if it does
	// we assume nobody will be calling this file, as it is a very unique name
	file, err := os.Create(dest + "/OptiFSDirMetadataSave.gob")

	if err != nil {
        log.Println("Doesn't exist.")
		return err
	}

	defer file.Close() // don't let the file close

	encode := gob.NewEncoder(file) // set the file that we created to the encoder
	eErr := encode.Encode(hashmap) // encode the hashmap into binary, put it in the file

	if eErr != nil {
        log.Println("Failed to encode.")
		return eErr
	}

    log.Println("Saved succesfully.")

	return nil

}

// retrieve the hashmap which stores information about directories in the filesystem
// when the filesystem restarts
func RetrieveDirMetadataHash(dest string) error {
	// lock the operation, and make sure it doesnt unlock until function is exited
	// unlocks when function is exited
	dirMutex.Lock()
	defer dirMutex.Unlock()

	log.Println("Retrieving directory metadata hash map")

	file, err := os.Open(dest + "/OptiFSDirMetadataSave.gob") // open where the hashmap was encoded

	if err != nil {
        log.Println("Doesn't exist.")
		return err
	}

	defer file.Close() // don't let the file close

	decode := gob.NewDecoder(file)          // set the file that we opened to the decoder
	dErr := decode.Decode(&dirMetadataHash) // decode the file back into the hashmap

	if dErr != nil {
        log.Println("Failed to decode.")
		return dErr
	}

    log.Println("Retrieved succesfully.")

	return nil

}

// for saving the hashmaps when system is shut off
// preserves private hashmaps
func SavePersistantStorage(dest string) {
	log.Println("Taking snapshot of file system...")
	SaveNodePersistenceHash(nodePersistenceHash, dest)
	SaveMetadataMap(regularFileMetadataHash, dest)
	SaveDirMetadataHash(dirMetadataHash, dest)
}

// for actually loading the hashmaps when the system is turned on
// preserves private hashmaps
func RetrievePersistantStorage(dest string) {
	RetrieveNodePersistenceHash(dest)
	RetrieveMetadataMap(dest)
	RetrieveDirMetadataHash(dest)
}

// Printing the regularFileMetadataHash for testing purposes
func PrintRegularFileMetadataHash() {
	log.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
	log.Println("PRINTING FILE METADATA HASHMAP")
	for key, value := range regularFileMetadataHash {
		log.Printf("Key: %+v, Value: %+v\n", key, value)
	}
	log.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
}

// Printing the dirMetadataHash for testing purposes
func PrintDirMetadataHash() {
	log.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
	log.Println("PRINTING DIR METADATA HASHMAP")
	for key, value := range dirMetadataHash {
		log.Printf("Key: %+v, Value: %+v\n", key, value)
	}
	log.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
}

// Printing the nodePersistenceHash for testing purposes
func PrintNodePersistenceHash() {
	log.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
	log.Println("PRINTING NODE METADATA HASHMAP")
	for key, value := range nodePersistenceHash {
		log.Printf("Key: %+v, Value: %+v\n", key, value)
	}
	log.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
}

// Ensure integrity
//
// Function looks through all retrieved hashmaps and ensures all their entries align with
// data in the underlying filesystem
func InsureIntegrity() {

    log.Println("Checking integrity of metadata against underlying filesystem.")

	// Collect any miss-entries
	pathsToDelete := []struct {
		path  string
		isDir bool
		hash  [64]byte
		ref   uint64
	}{}

    // Finds all discrepencies and stores them to be deleted
    // Cannot delete them during iteration - messes up the loop
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

	// Deletes all incorrect metadata
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

// allows us to constantly save each hashmap for data integrity
// saves every 30s by default
func SaveStorageRegularly(dest string, interval int) {
	for range time.Tick(time.Second * time.Duration(interval)) {
		SavePersistantStorage(dest)
	}
}
