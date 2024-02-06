package hashing

import (
	"encoding/gob"
	"errors"
	"log"
	"os"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fuse"
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
	ReferenceCount  uint32 // How many references there are to the same file content
	EntryList       map[uint64]*MapEntryMetadata
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

type NodeInfo struct {
	ContentHash [64]byte
	RefNum      uint64
}

// This is the hash that contains all custom metadata, and is indexable through file content hashed and reference numbers
var CustomMetadataHash = make(map[[64]byte]*MapEntry)

// This is the hash that contains node paths as a key and ContentHash's and RefNum. Used to keep node info persistent
var NodePersistenceHash = make(map[string]*NodeInfo)

// Stores information about a node by its path
func StoreNodeInfo(path string, contentHash [64]byte, refNum uint64) {
	NodePersistenceHash[path] = &NodeInfo{ContentHash: contentHash, RefNum: refNum}
}

// Retrieves information about a node by its path
func RetrieveNodeInfo(path string) (error, [64]byte, uint64) {
	info, ok := NodePersistenceHash[path]
	if !ok {
		return errors.New("No node info available for path"), [64]byte{}, 0
	}

	return nil, info.ContentHash, info.RefNum
}

// Function handles the removal of a node's persistent info
func RemoveNodeInfo(path string) error {
	delete(NodePersistenceHash, path)
	return nil
}

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

// Performs a lookup on the FileHashes map to tell if content is unique.
//
// Returns a bool for whether the contentHash can be found and also returns the underlying Inode
func IsUnique(contentHash [64]byte) (bool, uint32) {
	// Check to see if there's an entry for the contentHash and refNum above
	entry, exists := CustomMetadataHash[contentHash]
	// If it doesn't exist
	if !exists {
		log.Println("Content is unique!")
		return !exists, 0
	}

	// If it exists, return the underlying Inode
	log.Println("Content isn't unique!")
	return !exists, entry.UnderlyingInode
}

// Retrieves node metadata for a hash and refnum provided. Returns an error if it cannot be found
func LookupMetadataEntry(contentHash [64]byte, refNum uint64) (error, *MapEntryMetadata) {

	log.Println("Looking up a contentHash and refNum...")

	// First check for default values
	var defaultByteArray [64]byte
	if contentHash == defaultByteArray || refNum == 0 {
		log.Println("Default values detected, no MapEntryMetadata available")
		return errors.New("Default values detected"), &MapEntryMetadata{}
	}

	// Now actually query the hashmap
	if contentEntry, ok := CustomMetadataHash[contentHash]; ok {
		if nodeMetadata, ok := contentEntry.EntryList[refNum]; ok {
			return nil, nodeMetadata
		}
	}
	log.Println("contentHash and refNum didn't lead to MapEntryMetadata")
	return errors.New("Couldn't find entry!"), &MapEntryMetadata{}
}

// Removes a MapEntryMetadata object based on contenthash and refnum provided. Also handles if this potentially creates
// an empty MapEntry struct
func RemoveMetadata(contentHash [64]byte, refNum uint64) error {

	log.Printf("Removing Metadata for refNum{%v}, contentHash{%+v}\n", refNum, contentHash)

	// Check to see if an entry exists
	err, entry, _ := RetrieveMapEntryAndMetadataFromHashAndRef(contentHash, refNum)
	if err != nil {
		log.Println("Couldn't find an entry!")
		return err
	}
	log.Println("Found an entry!")

	// Delete the metadata from our entry
	delete(entry.EntryList, refNum)
	// Reflect these changes in the MapEntry
	entry.ReferenceCount--

	log.Println("Deleted metadata, checking to see if we need to delete the MapEntry")

	// Check to see if the MapEntry is empty
	if entry.ReferenceCount == 0 {
		// If it is, delete the whole entry
		delete(CustomMetadataHash, contentHash)
		log.Println("Deleted MapEntry")
	}
	log.Println("Finished removing metadata")

	return nil
}

// Retrieves the MapEntry struct from which the Metadata entry struct that the refNum and contentHash links to
func RetrieveMapEntryFromHashAndRef(contentHash [64]byte, refNum uint64) (error, *MapEntry) {

	log.Println("Looking up MapEntry from Hash and Ref")

	// First check for default values
	var defaultByteArray [64]byte
	if contentHash == defaultByteArray || refNum == 0 {
		log.Println("Default values detected, no MapEntry available")
		return errors.New("Default values detected"), &MapEntry{}
	}

	// Now actually query the hashmap
	if contentEntry, ok := CustomMetadataHash[contentHash]; ok {
		if _, ok := contentEntry.EntryList[refNum]; ok {
			log.Println("Found a MapEntry for valid hash and refnum!")
			return nil, contentEntry
		}
	}

	log.Println("Couldn't find a MapEntry for provided hash and refnum")
	return errors.New("Couldn't find entry!"), &MapEntry{}
}

// Retrieves the MapEntry and MapEntryMetadata struct from which the refNum and contentHash links to
func RetrieveMapEntryAndMetadataFromHashAndRef(contentHash [64]byte, refNum uint64) (error, *MapEntry, *MapEntryMetadata) {

	log.Println("Looking up MapEntry and MapEntryMetadata from Hash and Ref")

	// First check for default values
	var defaultByteArray [64]byte
	if contentHash == defaultByteArray || refNum == 0 {
		log.Println("Default values detected, no MapEntry or MapEntryData available")
		return errors.New("Default values detected"), &MapEntry{}, &MapEntryMetadata{}
	}

	// Now actually query the hashmap
	if contentEntry, ok := CustomMetadataHash[contentHash]; ok {
		if metadataEntry, ok := contentEntry.EntryList[refNum]; ok {
			log.Println("Found a MapEntry and MapEntryMetadata for valid hash and refnum!")
			return nil, contentEntry, metadataEntry
		}
	}

	log.Println("Couldn't find a MapEntry and MapEntryMetadata for provided hash and refnum")
	return errors.New("Couldn't find entry!"), &MapEntry{}, &MapEntryMetadata{}
}

// Updates a MapEntryMetadata object corresponding to the contentHash and refNum provided
//
// If refNum or contentHash is invalid, it returns an error
func SAFE_FullMetadataEntryUpdate(contentHash [64]byte, refNum uint64, unstableAttr *syscall.Stat_t) error {

	log.Println("Updating metadata through lookup...")
	// Ensure that contentHash and refNum is valid
	err, metadata := LookupMetadataEntry(contentHash, refNum)
	if err != nil {
		log.Println("Couldn't find the metadata struct")
		return err
	}
	log.Println("Found the metadata struct")

	log.Printf("unstableAttr: %+v\n", unstableAttr)

	// Now we can be sure the entry exists, let's update it
	(*metadata).Mode = (*unstableAttr).Mode
	(*metadata).Atim = (*unstableAttr).Atim
	(*metadata).Mtim = (*unstableAttr).Mtim
	(*metadata).Ctim = (*unstableAttr).Ctim
	(*metadata).Uid = (*unstableAttr).Uid
	(*metadata).Gid = (*unstableAttr).Gid
	(*metadata).Dev = (*unstableAttr).Dev
	(*metadata).Ino = (*unstableAttr).Ino
	(*metadata).Rdev = (*unstableAttr).Rdev
	(*metadata).Nlink = (*unstableAttr).Nlink
	(*metadata).Size = (*unstableAttr).Size
	(*metadata).Blksize = (*unstableAttr).Blksize
	(*metadata).Blocks = (*unstableAttr).Blocks
	(*metadata).X__pad0 = (*unstableAttr).X__pad0
	(*metadata).X__unused = (*unstableAttr).X__unused

	log.Printf("metadata: %+v\n", metadata)
	log.Println("Updated all custom metadata attributes through lookup")

	return nil
}

// Updates a MapEntryMetadata object corresponding to the MapEntryMetadata provided
func STRUCT_FullMetadataEntryUpdate(metadata *MapEntryMetadata, unstableAttr *syscall.Stat_t) error {

	log.Println("Updating metadata through struct...")
	log.Printf("unstableAttr: %+v\n", unstableAttr)

	(*metadata).Mode = (*unstableAttr).Mode
	(*metadata).Atim = (*unstableAttr).Atim
	(*metadata).Mtim = (*unstableAttr).Mtim
	(*metadata).Ctim = (*unstableAttr).Ctim
	(*metadata).Uid = (*unstableAttr).Uid
	(*metadata).Gid = (*unstableAttr).Gid
	(*metadata).Dev = (*unstableAttr).Dev
	(*metadata).Ino = (*unstableAttr).Ino
	(*metadata).Rdev = (*unstableAttr).Rdev
	(*metadata).Nlink = (*unstableAttr).Nlink
	(*metadata).Size = (*unstableAttr).Size
	(*metadata).Blksize = (*unstableAttr).Blksize
	(*metadata).Blocks = (*unstableAttr).Blocks
	(*metadata).X__pad0 = (*unstableAttr).X__pad0
	(*metadata).X__unused = (*unstableAttr).X__unused

	log.Printf("metadata: %+v\n", metadata)
	log.Println("Updated all custom metadata attributes through struct")

	return nil
}

// Moves old metadata to a new node being created
func MigrateMetadata(oldMeta *MapEntryMetadata, newMeta *MapEntryMetadata, unstableAttr *syscall.Stat_t) error {

    log.Println("Migrating old metadata across to new entry.")

	// Old attributes to carry across
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

// Function updates the UID and GID of a MapEntryMetadata
// Accepts pointers, doesn't set nil values
func UpdateOwner(metadata *MapEntryMetadata, uid, gid *uint32) error {
	if uid != nil {
		(*metadata).Uid = *uid
		log.Println("Updated custom UID")
	}
	if gid != nil {
		(*metadata).Gid = *gid
		log.Println("Updated custom GID")
	}
	return nil
}

// Function updates the time data of a MapEntryMetadata
// Accepts pointers, doesn't set nil values
func UpdateTime(metadata *MapEntryMetadata, atim, mtim, ctim *syscall.Timespec) error {
	if atim != nil {
		(*metadata).Atim = *atim
		log.Println("Updated custom ATime")
	}
	if mtim != nil {
		(*metadata).Mtim = *mtim
		log.Println("Updated custom MTime")
	}
	if ctim != nil {
		(*metadata).Ctim = *ctim
		log.Println("Updated custom CTime")
	}
	return nil
}

// Function updates inode and device fields of a MapEntryMetadata
// Accepts pointers, doesn't set nil values
func UpdateLocation(metadata *MapEntryMetadata, inode, dev *uint64) error {
	if inode != nil {
		(*metadata).Ino = *inode
		log.Println("Updated custom Inode")
	}
	if dev != nil {
		(*metadata).Dev = *dev
		log.Println("Updated custom Device")
	}
	return nil
}

// Function updates size field of a MapEntryMetadata
// Accepts pointers, doesn't set nil values
func UpdateSize(metadata *MapEntryMetadata, size *int64) error {
	if size != nil {
		(*metadata).Size = *size
		log.Println("Updated custom Size")
	}
	return nil
}

// Function updates link count of a MapEntryMetadata
// Accepts pointers, doesn't set nil values
func UpdateLinkCount(metadata *MapEntryMetadata, linkCount *uint64) error {
	if linkCount != nil {
		(*metadata).Nlink = *linkCount
		log.Println("Updated custom Nlink")
	}
	return nil
}

// Function updates mode of a MapEntryMetadata
// Accepts pointers, doesn't set nil values
func UpdateMode(metadata *MapEntryMetadata, mode *uint32) error {
	if mode != nil {
		(*metadata).Mode = *mode
		log.Println("Updated custom Mode")
	}
	return nil
}

// Function update C++ struct padding optimisation variables - not sure if they're used or needed
// Accepts pointers, doesn't set nil values
func UpdateWeirdCPPStuff(metadata *MapEntryMetadata, X__pad0 *int32, X__unused *[3]int64) error {
	if X__pad0 != nil {
		(*metadata).X__pad0 = *X__pad0
		log.Println("Updated custom X__pad0")
	}
	if X__unused != nil {
		(*metadata).X__unused = *X__unused
		log.Println("Updated custom X__unused")
	}
	return nil
}

// Function fills the AttrOut struct with its own information
func FillAttrOut(metadata *MapEntryMetadata, out *fuse.AttrOut) {

	log.Printf("metadata: %+v\n", metadata)

	// Fill the AttrOut with our custom attributes stored in our hash
	(*out).Attr.Size = uint64((*metadata).Size)
	(*out).Attr.Blocks = uint64((*metadata).Blocks)
	(*out).Attr.Atime = uint64((*metadata).Atim.Sec)
	(*out).Attr.Atimensec = uint32((*metadata).Atim.Nsec)
	(*out).Attr.Mtime = uint64((*metadata).Mtim.Sec)
	(*out).Attr.Mtimensec = uint32((*metadata).Mtim.Nsec)
	(*out).Attr.Ctime = uint64((*metadata).Ctim.Sec)
	(*out).Attr.Ctimensec = uint32((*metadata).Ctim.Nsec)
	(*out).Attr.Mode = (*metadata).Mode
	(*out).Attr.Nlink = uint32((*metadata).Nlink)
	(*out).Attr.Uid = uint32((*metadata).Uid)
	(*out).Attr.Gid = uint32((*metadata).Gid)
	(*out).Attr.Rdev = uint32((*metadata).Rdev)
	(*out).Attr.Blksize = uint32((*metadata).Blksize)

	log.Println("Filled AttrOut from custom metadata")
}

// Creates a new MapEntry in the main hash map when provided with a contentHash
// If the MapEntry already exists, we will simply pass back the already created MapEntry
func CreateMapEntry(contentHash [64]byte) *MapEntry {
	if entry, ok := CustomMetadataHash[contentHash]; ok {
		log.Println("MapEntry already exists, returning it")
		return entry
	}

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
	CustomMetadataHash[contentHash] = newEntry
	return newEntry
}

// Create a new createMapEntryMetadata struct (with default values) in the provided MapEntry.
// Returns the new createMapEntryMetadata along with the refNum to it.
func CreateMapEntryMetadata(entry *MapEntry) (refNum uint64, newEntry *MapEntryMetadata) {

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

// Function saves the custom metadata hashmap
// since a hashmap will be deleted when the system is restarted (stored in RAM)
// we encode the hashmap and store it in a file saved on disk to be loaded when OptiFS starts
func SaveMetadataMap(hashmap map[[64]byte]*MapEntry) error {
	log.Println("SAVING METADATA HASHMAP")
	dest := "hashing/OptiFSMetadataSave.gob"

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

// retrieve the encoded hashmap from the file when the system restarts
func RetrieveMetadataMap() error {
	log.Println("RETRIEVING METADATA HASHMAP")
	dest := "hashing/OptiFSMetadataSave.gob"

	file, err := os.Open(dest) // open where the hashmap was encoded

	if err != nil {
		return err
	}

	defer file.Close() // don't let the file close

	decode := gob.NewDecoder(file)             // set the file that we opened to the decoder
	dErr := decode.Decode(&CustomMetadataHash) // decode the file back into the hashmap

	if dErr != nil {
		log.Println("ERROR WITH DECODER - METADATA HASHMAP")
		return dErr
	}

	return nil
}

// printing hashmap for testing purposes
func PrintMetadataMap() {
	log.Println("PRINTING METADATA HASHMAP")
	for key, value := range CustomMetadataHash {
		log.Printf("Key: %x, Value: %v\n", key, value)
	}
}

// Function saves the node persistence hash into a Go binary (.gob) file
// since a hashmap will be deleted when the system is restarted (stored in RAM)
// we encode the hashmap and store it in a file saved on disk to be loaded when OptiFS starts
func SaveNodeInfoMap(hashmap map[string]*NodeInfo) error {
	log.Println("SAVING NODE INFO HASHMAP")
	dest := "hashing/OptiFSNodeSave.gob"

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
func RetrieveNodeInfoMap() error {
	log.Println("RETRIEVING NODE HASHMAP")
	dest := "hashing/OptiFSNodeSave.gob"

	file, err := os.Open(dest) // open where the hashmap was encoded

	if err != nil {
		return err
	}

	defer file.Close() // don't let the file close

	decode := gob.NewDecoder(file)              // set the file that we opened to the decoder
	dErr := decode.Decode(&NodePersistenceHash) // decode the file back into the hashmap

	if dErr != nil {
		log.Println("ERROR WITH DECODER - NODE HASHMAP")
		return dErr
	}

	return nil
}
