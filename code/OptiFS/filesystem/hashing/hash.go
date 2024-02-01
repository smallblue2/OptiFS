package hashing

import (
	"encoding/gob"
	"log"
	"os"
	"syscall"
    "errors"

	"github.com/hanwen/go-fuse/v2/fs"
	"lukechampine.com/blake3"
)

// hash in the release function
// take the data of the file
// pass it through the hashing function
// function to see if it is unique
// if hash's are the same, don't save the write, update reference count
// if they are different, proceed as normal

// MapEntry is a new content entry in our hashmap
type MapEntry struct {
	ReferenceCount  uint32                      // How many references there are to the same file content
	EntryList       map[uint64]MapEntryMetadata
	UnderlyingInode uint32
	IndexCounter    uint64
}

// MapEntryMetadata represents an instance of a file's hashed content and directly
// represents the node's info
type MapEntryMetadata struct {
	Dev       uint64
	Ino       uint64
	Gen       uint64
	Nlink     uint64
	Mode      uint32
	Uid       uint32
	Gid       uint32
	X__pad0   int32
	Rdev      uint64
	Size      int64
	Blksize   int64
	Blocks    int64
	Atim      syscall.Timespec
	Mtim      syscall.Timespec
	Ctim      syscall.Timespec
	X__unused [3]int64
}

// key = inode number
// value = hash
var FileHashes = make(map[[64]byte]MapEntry)

func HashContents(data []byte, flags uint32) [64]byte {

	// Check to see if we're appending
	if flags&syscall.O_APPEND != 0 {
		log.Println("APPENDING")
	}
	if flags&syscall.O_RDWR != 0 {
		log.Println("READWRITING")
	}
	if flags&syscall.O_WRONLY != 0 {
		log.Println("WRITINGONLY")
	}
	if flags&syscall.O_TRUNC != 0 {
		log.Println("TRUNCATING")
	}
	if flags&syscall.O_CREAT != 0 {
		log.Println("CREATING")
	}

	// TODO: Implement appending behaviour

	return blake3.Sum512(data)
}

// Performs a lookup on the FileHashes map to tell if a file is unique.
//
// Returns a bool for whether the contentHash can be found
func IsUnique(contentHash [64]byte) (bool, uint32) {
	// Check to see if there's an entry for the contentHash and refNum above
    entry, exists := FileHashes[contentHash]
    // If it doesn't exist
    if !exists {
        return !exists, 0
    }

    // If it exists, return the underlying Inode
	return !exists, entry.UnderlyingInode
}

// Retrieves node metadata for a hash and refnum provided. Returns an error if it cannot be found
func LookupEntry (contentHash [64]byte, refNum uint64) (error, MapEntryMetadata) {
    if contentEntry, ok := FileHashes[contentHash]; ok {
        if nodeMetadata, ok := contentEntry.EntryList[refNum]; ok {
            return nil, nodeMetadata
        }
    }
    return errors.New("Couldn't find entry!"), MapEntryMetadata{}
}

// Updates a MapEntryMetadata object corresponding to the contentHash and refNum provided
//
// If refNum is 0, it assumes the creation of a new entry and returns the refNum to it
// If there is no entry for the contentHash provided, it creates a new MapEntry object
func UpdateEntry(contentHash [64]byte, refNum uint64, stableAttr fs.StableAttr, unstableAttr syscall.Stat_t, UID, GID uint32) uint64 {

    log.Printf("Updating entry for %v, %v\n", refNum, contentHash)
	// Ensure there is an entry for this hash
	if _, ok := FileHashes[contentHash]; !ok {
        log.Println("File is unique, creating new MapEntry")
		// Create the entry - it doesn't exist
		newEntry := MapEntry{
			ReferenceCount:  0,
			EntryList:       make(map[uint64]MapEntryMetadata),
            IndexCounter: 0,
			UnderlyingInode: 0,
		}
		// TODO: Get the underlying inode
		FileHashes[contentHash] = newEntry
	}

    log.Println("Confirming creation of MapEntry...")
	// Get the contentEntry, now assuming it must exist
	contentEntry, ok := FileHashes[contentHash]
	// If it still doesn't exist - something's very wrong!
	if !ok {
        log.Println("It somehow still doesn't exist - EXITING")
		return 0
	}

	// Construct our attributes to update it with
    log.Println("It exists, checking the refNum now")

	// Get the current entry
	// If we don't have a reference entry - Create one
	if refNum == 0 {
        log.Println("refNum doesn't exist, constructing new MapEntryMetadata")
		// Brand new entry
		newEntry := MapEntryMetadata{
			Ino:       stableAttr.Ino,
			Mode:      stableAttr.Mode,
			Uid:       UID, // Change these to reflect the caller BECAUSE ITS NEW
			Gid:       GID, // Change these to reflect the caller BECAUSE ITS NEW
			Dev:       unstableAttr.Dev,
			Gen:       stableAttr.Gen,
			Nlink:     1, // 1 BECAUSE IT'S NEW
			Size:      unstableAttr.Size,
			Atim:      unstableAttr.Atim,
			Mtim:      unstableAttr.Mtim,
			Ctim:      unstableAttr.Ctim,
			Blksize:   unstableAttr.Blksize,
			Blocks:    unstableAttr.Blocks,
			Rdev:      unstableAttr.Rdev,
			X__pad0:   unstableAttr.X__pad0,
			X__unused: unstableAttr.X__unused,
		}

        log.Println("Applying new MapEntryMetadata struct into hashmap")

		// Add it to the entry list, indexing by the reference count
		// TODO: Maybe don't index by reference count? Is there a better system?
		contentEntry.EntryList[contentEntry.IndexCounter + 1] = newEntry
        // Update our counters
        contentEntry.IndexCounter = contentEntry.IndexCounter + 1
		contentEntry.ReferenceCount = contentEntry.ReferenceCount + 1

        log.Println("Finished! Returning new refNum")

        // Return the reference number
        return contentEntry.IndexCounter
	} else {
        log.Println("Refnum exists, updating the entry")
		// Otherwise assuming we're updating one that already exists
		currEntry, ok := contentEntry.EntryList[refNum]
		if !ok {
			return 0 // It doesn't exist, must have been handed the wrong refNum
		}

		// Update the entry
		currEntry.Uid = unstableAttr.Uid
		currEntry.Gid = unstableAttr.Gid
		currEntry.Dev = unstableAttr.Dev
		currEntry.Size = unstableAttr.Size
		currEntry.Atim = unstableAttr.Atim
		currEntry.Mtim = unstableAttr.Mtim
		currEntry.Ctim = unstableAttr.Ctim
		currEntry.Blksize = unstableAttr.Blksize
		currEntry.Blocks = unstableAttr.Blocks
		currEntry.Rdev = unstableAttr.Rdev
		currEntry.X__pad0 = unstableAttr.X__pad0
		currEntry.X__unused = unstableAttr.X__unused
		// The only values emitted from being updated above are the stableAttr - we'll see if this works
        log.Println("Entry updated succesfully!")

        // Return the reference number
        return refNum
	}
}

// since a hashmap will be deleted when the system is restarted (stored in RAM)
// we encode the hashmap and store it in a file saved on disk to be loaded when OptiFS starts
func SaveMap(hashmap map[[64]byte]MapEntry) error {
	log.Println("SAVING HASHMAP")
	dest := "hashing/OptiFSHashSave.gob"

	// create the file if it doesn't exist, truncate it if it does
	// we assume nobody will be calling this file, as it is a very unique name
	file, err := os.Create(dest)

	if err != nil {
		log.Println("ERROR WITH FILE - HASHMAP")
		return err
	}

	defer file.Close() // don't let the file close

	encode := gob.NewEncoder(file) // set the file that we created to the encoder
	eErr := encode.Encode(hashmap) // encode the hashmap into binary, put it in the file

	if eErr != nil {
		log.Println("ERROR WITH ENCODER - HASHMAP")
		return eErr
	}

	return nil
}

// retrieve the encoded hashmap from the file when the system restarts
func RetrieveMap() error {
	log.Println("RETRIEVING HASHMAP")
	dest := "hashing/OptiFSHashSave.gob"

	file, err := os.Open(dest) // open where the hashmap was encoded

	if err != nil {
		return err
	}

	defer file.Close() // don't let the file close

	decode := gob.NewDecoder(file)     // set the file that we opened to the decoder
	dErr := decode.Decode(&FileHashes) // decode the file back into the hashmap

	if dErr != nil {
		log.Println("ERROR WITH DECODER - HASHMAP")
		return dErr
	}

	return nil
}

// printing hashmap for testing purposes
func PrintMap() {
	log.Println("PRINTING HASHMAP")
	for key, value := range FileHashes {
		log.Printf("Key: %x, Value: %v\n", key, value)
	}
}
