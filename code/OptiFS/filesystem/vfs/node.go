package vfs

import (
	"bytes"
	"context"
	"encoding/binary"
	"filesystem/hashing"
	"filesystem/metadata"
	"filesystem/permissions"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"golang.org/x/sys/unix"
)

// Root for OptiFS
type OptiFSRoot struct {
	// The path to the root of the underlying file system (OptiFS is
	// a loopback filesystem)
	Path string

	// device the path is on (for NFS purposes)
	Dev uint64

	// returns new Inode for lookups/creation, allowing for custom behaviour (handling certain files)
	// if not set use an OptiFSRoot
	NewNode func(data *OptiFSRoot, parent *fs.Inode, name string, s *syscall.Stat_t) fs.InodeEmbedder
}

// General Node for OptiFS
type OptiFSNode struct {
	fs.Inode

	// The root node of the filesystem
	RootNode *OptiFSRoot

	currentHash [64]byte
	refNum      uint64
}

// Interfaces/contracts to abide by

// Filesystem and Node Operations
var _ = (fs.NodeStatfser)((*OptiFSNode)(nil))  // StatFS
var _ = (fs.InodeEmbedder)((*OptiFSNode)(nil)) // Inode

// Directory Operations
var _ = (fs.NodeLookuper)((*OptiFSNode)(nil))  // lookup
var _ = (fs.NodeOpendirer)((*OptiFSNode)(nil)) // opening directories
var _ = (fs.NodeReaddirer)((*OptiFSNode)(nil)) // read directory
var _ = (fs.NodeMkdirer)((*OptiFSNode)(nil))   // Creates a directory
var _ = (fs.NodeRmdirer)((*OptiFSNode)(nil))   // Unlinks (deletes) a directory
var _ = (fs.NodeAccesser)((*OptiFSNode)(nil))  // Checks access of a node

// Regular File Operations
var _ = (fs.NodeOpener)((*OptiFSNode)(nil))   // open a file
var _ = (fs.NodeCreater)((*OptiFSNode)(nil))  // creates a file
var _ = (fs.NodeUnlinker)((*OptiFSNode)(nil)) // Unlinks (deletes) a file
var _ = (fs.NodeWriter)((*OptiFSNode)(nil))   // Writes to a node
var _ = (fs.NodeFlusher)((*OptiFSNode)(nil))  // Flush the node
var _ = (fs.NodeReleaser)((*OptiFSNode)(nil)) // Releases a node
var _ = (fs.NodeFsyncer)((*OptiFSNode)(nil))  // Ensures writes are actually written to disk

// Attribute Operations
var _ = (fs.NodeGetattrer)((*OptiFSNode)(nil))     // get attributes of a node
var _ = (fs.NodeSetattrer)((*OptiFSNode)(nil))     // Set attributes of a node
var _ = (fs.NodeGetxattrer)((*OptiFSNode)(nil))    // Get extended attributes of a node
var _ = (fs.NodeSetxattrer)((*OptiFSNode)(nil))    // Set extended attributes of a node
var _ = (fs.NodeRemovexattrer)((*OptiFSNode)(nil)) // Remove extended attributes of a node
var _ = (fs.NodeListxattrer)((*OptiFSNode)(nil))   // List extended attributes of a node

// Locking and Linking Operations
var _ = (fs.NodeGetlker)((*OptiFSNode)(nil))  // find conflicting locks for given lock
var _ = (fs.NodeSetlker)((*OptiFSNode)(nil))  // gets a lock on a node
var _ = (fs.NodeSetlkwer)((*OptiFSNode)(nil)) // gets a lock on a node, waits for it to be ready
var _ = (fs.NodeRenamer)((*OptiFSNode)(nil))  // Changes the directory a node is in
var _ = (fs.NodeMknoder)((*OptiFSNode)(nil))  // Similar to lookup, but creates the inode
var _ = (fs.NodeLinker)((*OptiFSNode)(nil))   // For handling hard links
// var _ = (fs.NodeSymlinker)((*OptiFSNode)(nil))  // For handling hard links
// var _ = (fs.NodeReadlinker)((*OptiFSNode)(nil)) // For reading symlinks

// Statfs implements statistics for the filesystem that holds this
// Inode.
func (n *OptiFSNode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	log.Println("IN STATFS")
	// As this is a loopback filesystem, we will stat the underlying filesystem.
	var s syscall.Statfs_t = syscall.Statfs_t{}
	err := syscall.Statfs(n.RPath(), &s)
	if err != nil {
		log.Println("Failed to stat underlying!")
		return fs.ToErrno(err)
	}
	log.Printf("%v\n", s)
	out.FromStatfsT(&s)
	return fs.OK
}

// Path returns the full RPath to the underlying file in the underlying filesystem
func (n *OptiFSNode) RPath() string {
	// Get 'n's node's path relative to OptiFS's root
	var path string = n.Path(n.Root())
	return filepath.Join(n.RootNode.Path, path)
}

// checker if we are in the root of the filesystem
// used for sysadmin purposes
func (n *OptiFSNode) IsRoot() bool {
	return n.Inode.Root() == &n.Inode
}

// checker to see if we are allowed to perform operations at the current location
func (n *OptiFSNode) IsAllowed(ctx context.Context) error {
	// if we're in the root directory
	if n.IsRoot() {
		log.Println(">>> WE ARE IN ROOT!!!!!")
		err, userID, groupID := permissions.GetUIDGID(ctx) // get the UID/GID of the person doing the syscall
		if err != nil {
			log.Println("ERROR GETTING UID/GID")
			return fs.ToErrno(err)
		}

		// if they are not the sysadmin, they can't operate in root!
		if userID != permissions.SysAdmin.UID || groupID != permissions.SysAdmin.GID {
			log.Println("Only the syadmin can do operations in root :(")
			return fs.ToErrno(syscall.EACCES)
		}
	} else {
		log.Println(">>> WE ARE NOT IN ROOT!")
	}

	return nil
}

// checker to see if we are allowed to perform operations in the source and destination specified
func (n *OptiFSNode) IsAllowedTwoLocations(ctx context.Context, newParent fs.InodeEmbedder) error {

	dest := newParent.EmbeddedInode() // get the inode of the destination

	// if the source or the destination is in the root directory
	if n.IsRoot() || dest.IsRoot() {
		log.Println(">>> SRC OR DEST IS IN ROOT!!!!!")
		err, userID, groupID := permissions.GetUIDGID(ctx) // get the UID/GID of the person doing the syscall
		if err != nil {
			log.Println("ERROR GETTING UID/GID")
			return fs.ToErrno(err)
		}

		// if they are not the sysadmin, they can't operate in root!
		if userID != permissions.SysAdmin.UID || groupID != permissions.SysAdmin.GID {
			log.Println("Only the syadmin can do operations in root :(")
			return fs.ToErrno(syscall.EACCES)
		}
	} else {
		log.Println(">>> SRC OR DEST IS NOT IN ROOT!")
	}

	return nil
}

// create a new node in the system
func (n *OptiFSRoot) newNode(parent *fs.Inode, name string, s *syscall.Stat_t) fs.InodeEmbedder {
	// If the NewNode function has a custom definition, use it
	if n.NewNode != nil {
		return n.NewNode(n, parent, name, s)
	}

	// Otherwise, create an OptiFSNode and return it's address
	return &OptiFSNode{
		RootNode: n, // Set the root (so we can keep track of it easily)
	}
}

// Create an existing node in the system
func (n *OptiFSRoot) existingNode(existingHash [64]byte, existingRef uint64) fs.InodeEmbedder {
	return &OptiFSNode{
		RootNode:    n,
		currentHash: existingHash,
		refNum:      existingRef,
	}
}

// Function creates a custom, unique, inode number from a file stat structure
//
// since we're using NFS, theres a chance not all Inode numbers will be unique, so we
// calculate one using bit swapping
func (n *OptiFSRoot) getNewStableAttr(s *syscall.Stat_t, path *string) fs.StableAttr {
	// Otherwise, generate a new one
	inodeInfoString := fmt.Sprintf("%s%d%d", *path, s.Dev, s.Ino)
	inodeInfo := []byte(inodeInfoString)
	hash := hashing.HashContents(inodeInfo, 0)

	vIno := binary.BigEndian.Uint64(hash[:8])

	return fs.StableAttr{
		Mode: s.Mode, // Copy the underlying files perms
		Gen:  1,      // generation number (determines lifetime of inode)
		Ino:  vIno,   // Unique generated inode number
	}

}

// get the attributes for the file hashing
func (n *OptiFSNode) GetAttr() fs.StableAttr {
	return n.StableAttr()
}

// lookup FINDS A NODE based on its name
func (n *OptiFSNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	path := n.RPath()

	log.Printf("LOOKUP performed for {%v} from node {%v}\n", name, path)

	// Check execute permissions on the parent directory
	err1, dirMetadata := metadata.LookupDirMetadata(path)
	if err1 == 0 {
		log.Println("Checking directory custom permissions")
		isAllowed := permissions.CheckPermissions(ctx, dirMetadata, 2) // Check exec permissions
		if !isAllowed {
			log.Println("Not allowed!")
			return nil, fs.ToErrno(syscall.EACCES)
		}
		log.Println("Allowed!")
	}

	filePath := filepath.Join(n.RPath(), name) // getting the full path to the file (join name to path)
	s := syscall.Stat_t{}                      // status of a file
	err := syscall.Lstat(filePath, &s)         // gets the file attributes (also returns attrs of symbolic link)

	if err != nil {
		log.Println("LOOKUP FAILED!")
		return nil, fs.ToErrno(err)
	}

	oErr, oNode, _ := HandleNodeInstantiation(ctx, n, filePath, name, &s, out, nil, nil)

	return oNode, oErr
}

// opens a directory and then closes it
func (n *OptiFSNode) Opendir(ctx context.Context) syscall.Errno {

	log.Println("In OPENDIR!")

	path := n.RPath()
	log.Printf("Opening directory '%v'\n", path)

	// Check exec permissions if there is custom metadata
	err1, dirMetadata := metadata.LookupDirMetadata(path)
	if err1 == 0 {
		log.Println("Checking directory custom permissions")
		isAllowed := permissions.CheckPermissions(ctx, dirMetadata, 2) // Check exec permissions
		if !isAllowed {
			log.Println("Not allowed!")
			return fs.ToErrno(syscall.EACCES)
		}
	}

	log.Printf("Attempting to open dir - {%v}\n", path)

	// Open the directory (n), 0755 is the default perms for a new directory
	dir, err2 := syscall.Open(n.RPath(), syscall.O_DIRECTORY, 0755)
	if err2 != nil {
		log.Printf("Error opening dir - {%v}\n", err2)
		return fs.ToErrno(err2)
	}
	syscall.Close(dir) // close when finished
	log.Println("Succesfully opened directory!")
	return fs.OK
}

// opens a stream of dir entries,
func (n *OptiFSNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {

	log.Println("Entered Readdir!")

	path := n.RPath()

	// Check read permissions if available
	err1, dirMetadata := metadata.LookupDirMetadata(path)
	if err1 == 0 {
		isAllowed := permissions.CheckPermissions(ctx, dirMetadata, 0)
		if !isAllowed {
			log.Println("Not allowed!")
			return nil, fs.ToErrno(syscall.EACCES)
		}
		log.Println("Allowed to readdir!")
	}

	return fs.NewLoopbackDirStream(path)
}

// get the attributes of a file/dir, either with a filehandle (if passed) or through inodes
func (n *OptiFSNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	log.Println("NODE || entered GETATTR")

	path := n.RPath()

	// Not sure if attributes carry over node Lookups, check persistent storage to be sure
	var existingHash [64]byte
	var existingRef uint64
	if err, _, _, _, _, isDir, hash, ref := metadata.RetrieveNodeInfo(path); err == fs.OK && !isDir {
		existingHash = hash
		existingRef = ref
	}

	// Try and get an entry in our own custom system
	err1, fileMetadata := metadata.LookupRegularFileMetadata(existingHash, existingRef)
	if err1 == nil { // If it exists
		metadata.FillAttrOut(fileMetadata, out)
		return fs.OK
	}
	err2, dirMetadata := metadata.LookupDirMetadata(path)
	if err2 == 0 {
		metadata.FillAttrOut(dirMetadata, out)
		return fs.OK
	}

	log.Println("No custom metadata available - Statting underlying node")

	// OTHERWISE, just stat the node
	var err error
	s := syscall.Stat_t{}
	if f == nil {
		// IF we're dealing with the root, stat it directly as opposed to handling symlinks
		if &n.Inode == n.Root() {
			log.Printf("Trying to stat the root - %v\n", path)
			err = syscall.Stat(path, &s) // if we are looking for the root of FS
		} else {
			// Otherwise, use Lstat to handle symlinks as well as normal files/directories
			log.Println("Statting regular filesystem node")
			err = syscall.Lstat(path, &s) // if it's just a normal file/dir
		}

		if err != nil {
			log.Printf("We got an error - %v\n", err)
			return fs.ToErrno(err)
		}
		log.Printf("Succesfully statted {%v} - {%v}\n", path, s)
	} else {
		log.Println("Statting through file descriptor")
		serr := syscall.Fstat(f.(*OptiFSFile).fdesc, &s) // stat the file descriptor to get the attrs (no path needed)

		if serr != nil {
			return fs.ToErrno(serr)
		}

		log.Printf("Stat - %v\n", s)

	}

	out.FromStat(&s) // fill the attr into struct if no errors

	return fs.OK
}

// Sets attributes of a node
func (n *OptiFSNode) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {

	log.Println("NODE || entered SETATTR")

	// Retrieve persisten data as 'n' may have empty attributes as lookups are performed as *fs.Inode's, as opposed to
	// *OptiFSNode, so it seems like attributes cannot always carry across operations

	var existingHash [64]byte
	var existingRef uint64
	var isDir bool

	// Search for persistent hash or ref's
	log.Println("Searching for persistently stored hash and ref")
	if err, _, _, _, _, isDir, hash, ref := metadata.RetrieveNodeInfo(n.RPath()); err == fs.OK && !isDir {
		existingHash = hash
		existingRef = ref
		log.Printf("Found - {%v} - {%+v}\n", existingHash, existingRef)
	} else {
		log.Println("No persistent regfile data found!")
		isDir = true
	}

	// Check to see if we can find an entry in our node hashmap
	err1, fileMetadata := metadata.LookupRegularFileMetadata(existingHash, existingRef)
	if err1 == nil {
		log.Println("Setting attributes for custom regular file metadata.")
		if f != nil {
			return fs.ToErrno(SetAttributes(ctx, fileMetadata, in, n, f.(*OptiFSFile), out, isDir))
		}
		return fs.ToErrno(SetAttributes(ctx, fileMetadata, in, n, nil, out, isDir))
	}
	// Also check to see if we can find an entry in our directory hashmap
	err2, dirMetadata := metadata.LookupDirMetadata(n.RPath())
	if err2 == 0 {
		log.Println("Setting attributes for custom directory metadata.")
		if f != nil {
			return fs.ToErrno(SetAttributes(ctx, dirMetadata, in, n, f.(*OptiFSFile), out, isDir))
		}
		return fs.ToErrno(SetAttributes(ctx, dirMetadata, in, n, nil, out, isDir))
	}

	// Otherwise, neither exists; just do underlying node
	log.Println("Setting attributes for underlying node.")
	return fs.ToErrno(SetAttributes(ctx, nil, in, n, nil, out, isDir))
}

// Opens a file for reading, and returns a filehandle
// flags determines how we open the file (read only, read-write, etc...)
func (n *OptiFSNode) Open(ctx context.Context, flags uint32) (f fs.FileHandle, fFlags uint32, errno syscall.Errno) {

	log.Println("ENTERED OPEN")

	path := n.RPath()

	// Not sure if ACCESS is checked for opening a file
	log.Printf("\n=======================\nOpen Flags: (0x%v)\n=======================\n", strconv.FormatInt(int64(flags), 16))

	// Not sure file attributes are persisten between lookups, better to retrieve from persisten store
	// instead of node
	var existingHash [64]byte
	var existingRef uint64
	if err, _, _, _, _, _, hash, ref := metadata.RetrieveNodeInfo(path); err == fs.OK {
		log.Println("Persisten hash and ref exists for file")
		existingHash = hash
		existingRef = ref
	}

	// Check custom permissions for opening the file
	// Lookup metadata entry
	herr, fileMetadata := metadata.LookupRegularFileMetadata(existingHash, existingRef)
	if herr == nil { // If we found custom metadata
		log.Println("Checking custom metadata for OPEN permission")
		allowed := permissions.CheckOpenPermissions(ctx, fileMetadata, flags)
		if !allowed {
			log.Println("Not allowed!")
			return nil, 0, syscall.EACCES
		}
	}

	fileDescriptor, err := syscall.Open(path, int(flags), 0666) // try to open the file at path
	if err != nil {
		return nil, 0, fs.ToErrno(err)
	}

	// Creates a custom filehandle from the returned file descriptor from Open
	optiFile := NewOptiFSFile(fileDescriptor, n.GetAttr(), flags, existingHash, existingRef)
	//log.Println("Created a new loopback file")
	return optiFile, flags, fs.OK
}

// Get EXTENDED attribute
func (n *OptiFSNode) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	log.Println("ENTERED GETXATTR")

	// If we're dealing with the root, just return FS.OK
	if &n.Inode == n.Root() {
		return 0, fs.OK
	}

	// Retrieve hash and refnums from persistent node
	_, _, _, _, _, _, hash, ref := metadata.RetrieveNodeInfo(n.RPath())

	// Check if the user has read access
	var customMetadata *metadata.MapEntryMetadata
	var isDir bool
	err1, nodeMetadata := metadata.LookupRegularFileMetadata(hash, ref)
	if err1 == nil {
		hasRead := permissions.CheckPermissions(ctx, nodeMetadata, 0)
		if !hasRead {
			return 0, syscall.EACCES
		}
		customMetadata = nodeMetadata
		log.Println("Set metadata to regfile metadata")
	}
	path := n.RPath()
	err2, dirMetadata := metadata.LookupDirMetadata(path)
	if err2 == 0 {
		hasRead := permissions.CheckPermissions(ctx, dirMetadata, 0)
		if !hasRead {
			return 0, syscall.EACCES
		}
		customMetadata = dirMetadata
		isDir = true
		log.Println("Set metadata to dir metadata")
	}

	attributeSize, err3 := metadata.GetCustomXAttr(customMetadata, attr, &dest, isDir)
	return uint32(attributeSize), fs.ToErrno(err3)
}

// Set EXTENDED attribute
func (n *OptiFSNode) Setxattr(ctx context.Context, attr string, data []byte, flags uint32) syscall.Errno {
	log.Println("ENTERED SETXATTR")

	// If we're dealing with the root, just return FS.OK
	if &n.Inode == n.Root() {
		return fs.OK
	}

	// Retrieve hash and refnums from persistent node
	_, _, _, _, _, _, hash, ref := metadata.RetrieveNodeInfo(n.RPath())

	// Check if the user has write access
	var customMetadata *metadata.MapEntryMetadata
	var isDir bool
	err1, nodeMetadata := metadata.LookupRegularFileMetadata(hash, ref)
	if err1 == nil {
		hasWrite := permissions.CheckPermissions(ctx, nodeMetadata, 1)
		if !hasWrite {
			return syscall.EACCES
		}
		customMetadata = nodeMetadata
		log.Println("Set metadata to regfile metadata")
	}
	path := n.RPath()
	err2, dirMetadata := metadata.LookupDirMetadata(path)
	if err2 == 0 {
		hasWrite := permissions.CheckPermissions(ctx, dirMetadata, 1)
		if !hasWrite {
			return syscall.EACCES
		}
		customMetadata = dirMetadata
		isDir = true
		log.Println("Set metadata to dir metadata")
	}

	return metadata.SetCustomXAttr(customMetadata, attr, data, flags, isDir)

}

// Remove EXTENDED attribute
func (n *OptiFSNode) Removexattr(ctx context.Context, attr string) syscall.Errno {

	log.Println("ENTERED REMOVEXATTR")

	// If we're dealing with the root, just return FS.OK
	if &n.Inode == n.Root() {
		return fs.OK
	}

	// Retrieve hash and refnums from persistent node
	_, _, _, _, _, _, hash, ref := metadata.RetrieveNodeInfo(n.RPath())

	// Custom metadata and flag to tell if we're dealing with directories or not
	var customMetadata *metadata.MapEntryMetadata
	var isDir bool

	// Check if the user has write access
	err1, nodeMetadata := metadata.LookupRegularFileMetadata(hash, ref)
	if err1 == nil {
		hasWrite := permissions.CheckPermissions(ctx, nodeMetadata, 1)
		if !hasWrite {
			return syscall.EACCES
		}
		customMetadata = nodeMetadata
	}
	path := n.RPath()
	err2, dirMetadata := metadata.LookupDirMetadata(path)
	if err2 == 0 {
		hasWrite := permissions.CheckPermissions(ctx, dirMetadata, 1)
		if !hasWrite {
			return syscall.EACCES
		}
		customMetadata = dirMetadata
		isDir = true
	}

	return metadata.RemoveCustomXAttr(customMetadata, attr, isDir)
}

// List EXTENDED attributes
func (n *OptiFSNode) Listxattr(ctx context.Context, dest []byte) (uint32, syscall.Errno) {
	//log.Println("ENTERED LISTXATTR")

	// If we're dealing with the root, just return FS.OK
	if &n.Inode == n.Root() {
		return 0, fs.OK
	}

	// Retrieve hash and refnums from persistent node
	_, _, _, _, _, _, hash, ref := metadata.RetrieveNodeInfo(n.RPath())

	// Custom metadata and flag to tell if we're dealing with directories or not
	var customMetadata *metadata.MapEntryMetadata
	var isDir bool

	// Check if the user has read access
	err1, nodeMetadata := metadata.LookupRegularFileMetadata(hash, ref)
	if err1 == nil {
		hasRead := permissions.CheckPermissions(ctx, nodeMetadata, 0)
		if !hasRead {
			return 0, syscall.EACCES
		}
		customMetadata = nodeMetadata
	}
	path := n.RPath()
	err2, dirMetadata := metadata.LookupDirMetadata(path)
	if err2 == 0 {
		hasRead := permissions.CheckPermissions(ctx, dirMetadata, 0)
		if !hasRead {
			return 0, syscall.EACCES
		}
		customMetadata = dirMetadata
		isDir = true
	}

	// Pass it down to the filesystem below
	allAttributesSize, err3 := metadata.ListCustomXAttr(customMetadata, &dest, isDir)
	log.Println("Finished listing xattr")
	return uint32(allAttributesSize), fs.ToErrno(err3)
}

// Checks access of a node
func (n *OptiFSNode) Access(ctx context.Context, mask uint32) syscall.Errno {
	log.Printf("Checking ACCESS for %v\n", n.RPath())

	// Prioritise custom metadata

	// Need to get the currentHash and refNum from the persistent store as node attributes may
	// not carry across lookups
	path := n.RPath()
	_, _, _, _, _, _, hash, ref := metadata.RetrieveNodeInfo(path)

	// Check if custom metadata exists for a regular file
	if err, fileMetadata := metadata.LookupRegularFileMetadata(hash, ref); err == nil {
		// If there is no metadata, just perform a normal ACCESS on the underlying node
		log.Println("Found custom regular file metadata, checking...")
		isAllowed := permissions.CheckMask(ctx, mask, fileMetadata)
		if !isAllowed {
			return fs.ToErrno(syscall.EACCES)
		}
	}

	// Check if custom metadata exists for a directory
	if err, dirMetadata := metadata.LookupDirMetadata(path); err == 0 {
		log.Println("Found custom directory metadata, checking...")
		isAllowed := permissions.CheckMask(ctx, mask, dirMetadata)
		if !isAllowed {
			return fs.ToErrno(syscall.EACCES)
		}
	}

	// Otherwise, default the access to underlying filesystem
	return fs.ToErrno(syscall.Access(path, mask))
}

// Make a directory
func (n *OptiFSNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Println("ENTERED MKDIR")

	// check if the user is allowed to make a directory here
	// i.e if we are in root, are they the sysadmin?
	err := n.IsAllowed(ctx)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	filePath := filepath.Join(n.RPath(), name)

	// Check write and execute permissions on the parent directory
	err1, dirMetadata := metadata.LookupDirMetadata(n.RPath())
	if err1 == 0 {
		log.Println("Checking directory custom permissions")
		writePerm := permissions.CheckPermissions(ctx, dirMetadata, 1) // Check for write permissions
		if !writePerm {
			log.Println("Not allowed!")
			return nil, fs.ToErrno(syscall.EACCES)
		}
		execPerm := permissions.CheckPermissions(ctx, dirMetadata, 2) // Check exec permissions
		if !execPerm {
			log.Println("Not allowed!")
			return nil, fs.ToErrno(syscall.EACCES)
		}
	}

	// Create the directory
	err2 := syscall.Mkdir(filePath, mode)
	if err2 != nil {
		return nil, fs.ToErrno(err2)
	}
	log.Println("Created the directory!")

	log.Printf("Mode used: 0x%X\n", mode)

	// Now stat the new directory, ensuring it was created
	var directoryStatus syscall.Stat_t
	err2 = syscall.Stat(filePath, &directoryStatus)
	if err2 != nil {
		return nil, fs.ToErrno(err2)
	}
	log.Println("Statted the directory!")

	log.Printf("Mode statted: 0x%X\n", directoryStatus.Mode)

	// Handle the node instantiation
	oErr, oInode, _ := HandleNodeInstantiation(ctx, n, filePath, name, &directoryStatus, out, nil, nil)

	// Update our custom metadata system

	return oInode, oErr
}

func (n *OptiFSNode) setOwner(ctx context.Context, path string) error {
	// make sure we are running as root user (root user id is 0)
	// if os.Getuid() != 0 {
	// 	return nil
	// }

	person, check := fuse.FromContext(ctx) // get person's info
	// if we werent able to get the info of the person who performed the operation
	if !check {
		return nil
	}

	log.Printf("OWNER HAS BEEN SET TO {%v}\n", person)

	// change the ownership of the file/dir to the UID and GID of the person
	return syscall.Lchown(path, int(person.Uid), int(person.Gid))

}

// create a REGULAR FILE that doesn't exist, also fills in the gid/uid of the user into the file attributes
func (n *OptiFSNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (inode *fs.Inode, f fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	// check if the user is allowed to make a file here
	// i.e if we are in root, are they the sysadmin?
	err := n.IsAllowed(ctx)
	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	// Check write and exec permissions on the parent directory
	err1, dirMetadata := metadata.LookupDirMetadata(n.RPath())
	if err1 == 0 {
		log.Println("Checking directory custom permissions")
		writePerm := permissions.CheckPermissions(ctx, dirMetadata, 1) // Check for write permissions
		if !writePerm {
			log.Println("Not allowed - Write!")
			return nil, nil, 0, fs.ToErrno(syscall.EACCES)
		}
		execPerm := permissions.CheckPermissions(ctx, dirMetadata, 2) // Check exec permissions
		if !execPerm {
			log.Println("Not allowed - Exec!")
			return nil, nil, 0, fs.ToErrno(syscall.EACCES)
		}
	}

	filePath := filepath.Join(n.RPath(), name) // create the path for the new file

	// try to open the file, OR create if theres no file to open
	fdesc, err := syscall.Open(filePath, int(flags)|os.O_CREATE, mode)

	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	_, ok := hashHashMap[filePath]
	if !ok {
		person, check := fuse.FromContext(ctx)
		if check {
			hashHashMap[filePath] = &writeStore{uid: person.Uid, gid: person.Gid}
		}
	}
	n.setOwner(ctx, filePath) // set who made the file

	// stat the new file, making sure it was created
	s := syscall.Stat_t{}
	fErr := syscall.Fstat(fdesc, &s)
	if fErr != nil {
		syscall.Close(fdesc) // close the file descr
		return nil, nil, 0, fs.ToErrno(err)
	}

	oErr, oNode, oFile := HandleNodeInstantiation(ctx, n, filePath, name, &s, out, &fdesc, &flags)

	return oNode, oFile, 0, oErr
}

// Unlinks (removes) a file
func (n *OptiFSNode) Unlink(ctx context.Context, name string) syscall.Errno {
	// check if the user is allowed to remove a file here
	// i.e if we are in root, are they the sysadmin?
	aErr := n.IsAllowed(ctx)
	if aErr != nil {
		return fs.ToErrno(aErr)
	}

	log.Printf("UNLINK performed on %v from node %v\n", name, n.RPath())

	// Check write and exec permissions on the parent directory
	err1, dirMetadata := metadata.LookupDirMetadata(n.RPath())
	if err1 == 0 {
		log.Println("Checking directory custom permissions")
		writePerm := permissions.CheckPermissions(ctx, dirMetadata, 1) // Check for write permissions
		if !writePerm {
			log.Println("Not allowed!")
			return fs.ToErrno(syscall.EACCES)
		}
		execPerm := permissions.CheckPermissions(ctx, dirMetadata, 2) // Check exec permissions
		if !execPerm {
			log.Println("Not allowed!")
			return fs.ToErrno(syscall.EACCES)
		}
	}

	// Flag for custom metadata existing
	var customExists bool

	// Construct the file's path since 'n' is actually the parent directory
	filePath := filepath.Join(n.RPath(), name)

	// Since 'n' is actually the parent directory, we need to retrieve the underlying node to search
	// for custom metadata to cleanup
	herr, _, _, _, _, _, contentHash, refNum := metadata.RetrieveNodeInfo(filePath)
	if herr == fs.OK {
		// Mark if it exists
		customExists = true
	}

	err := syscall.Unlink(filePath)
	if err != nil {
		return fs.ToErrno(err)
	}

	// Cleanup the custom metadata side of things ONLY if the unlink operations suceeded
	if customExists {
		metadata.RemoveRegularFileMetadata(contentHash, refNum)
		metadata.RemoveNodeInfo(filePath)
	}

	return fs.ToErrno(err)
}

// Unlinks (removes) a directory
func (n *OptiFSNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	// check if the user is allowed to remove a directory here
	// i.e if we are in root, are they the sysadmin?
	err := n.IsAllowed(ctx)
	if err != nil {
		return fs.ToErrno(err)
	}

	// Check exec and write permissions on the parent directory
	// Don't need to check the directory being removed, as it must be empty to be removed
	err1, dirMetadata := metadata.LookupDirMetadata(n.RPath())
	if err1 == 0 {
		log.Println("Checking directory custom permissions")
		writePerm := permissions.CheckPermissions(ctx, dirMetadata, 1) // Check for write permissions
		if !writePerm {
			log.Println("Not allowed!")
			return fs.ToErrno(syscall.EACCES)
		}
		execPerm := permissions.CheckPermissions(ctx, dirMetadata, 2) // Check exec permissions
		if !execPerm {
			log.Println("Not allowed!")
			return fs.ToErrno(syscall.EACCES)
		}
	}

	//log.Printf("RMDIR performed on %v from node %v\n", name, n.path())
	filePath := filepath.Join(n.RPath(), name)

	// See if we can remove the dir from our custom dir map first
	metadata.RemoveDirEntry(filePath)
	metadata.RemoveNodeInfo(filePath)

	rErr := syscall.Rmdir(filePath)
	return fs.ToErrno(rErr)
}

// For storing hashes for each block write under a file
type writeStore struct {
	buffer *bytes.Buffer
	uid    uint32
	gid    uint32
}

var hashHashMap = make(map[string]*writeStore) // A map keyed by a filepath and has a slice of 64byte arrays as a value
var hashHashMapLock sync.RWMutex

func (n *OptiFSNode) Write(ctx context.Context, f fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	//log.Println("Entered WRITE")
	if f != nil {
		// Lock the file
		f.(*OptiFSFile).mu.Lock()
		defer f.(*OptiFSFile).mu.Unlock()

		nodePath := n.RPath()

		// Check if n's attributes are default
		var defaultByteArray [64]byte
		var hash [64]byte
		var ref uint64
		if n.currentHash == defaultByteArray || n.refNum == 0 {
			// Retrieve from the persisten store
			_, _, _, _, _, _, tmpHash, tmpRef := metadata.RetrieveNodeInfo(nodePath)
			hash = tmpHash
			ref = tmpRef
		} else {
			hash = n.currentHash
			ref = n.refNum
		}

		// Ensure we have permission to write to the file
		// Check if we have permission to write to the file
		err, fileMetadata := metadata.LookupRegularFileMetadata(hash, ref)
		if err == nil { // if it exists
			writePerm := permissions.CheckPermissions(ctx, fileMetadata, 1) // check write perm
			if !writePerm {
				return 0, syscall.EACCES
			}
		}

		//log.Println("We have permission!")

		// Hash the content
		newHash := hashing.HashContents(data, f.(*OptiFSFile).flags)
		// Store the hash in the hashHashMap
		// See if an entry exists
		log.Println("Requesting hashHashMapLock")
		hashHashMapLock.Lock()
		entry, ok := hashHashMap[nodePath]
		if ok {
			// Write to the buffer, ensuring it exists
			if (*entry).buffer == nil {
				(*entry).buffer = bytes.NewBuffer(newHash[:])
			} else {
				(*entry).buffer.Write(newHash[:])
			}
		} else {
			// Buffer doesn't exist, create a new one
			// Extract UID and GID (doesn't work in RELEASE for some reason, so we store caller info here)
			log.Println("Setting UID/GID in hash buffer")
			caller, check := fuse.FromContext(ctx)
			var entry *writeStore
			if !check {
				entry = &writeStore{buffer: bytes.NewBuffer(newHash[:]), uid: 65534, gid: 65534}
			} else {
				log.Println("Filling in")
				entry = &writeStore{buffer: bytes.NewBuffer(newHash[:]), uid: uint32(caller.Uid), gid: uint32(caller.Gid)}
			}
			hashHashMap[nodePath] = entry
		}
		hashHashMapLock.Unlock()
		log.Println("Released hashHashMapLock")

		// Now write to the underlying file
		//log.Println("Performing normal write")
		numOfBytesWritten, werr := syscall.Pwrite(f.(*OptiFSFile).fdesc, data, off)
		if werr != nil {
			//log.Println("Error performing normal write!")
			return 0, fs.ToErrno(werr)
		}
		//log.Println("Wrote to file succesfully")
		return uint32(numOfBytesWritten), fs.OK
	}

	log.Println("No file descriptor - exiting!")

	return 0, fs.ToErrno(syscall.EBADFD)
}

func (n *OptiFSNode) Flush(ctx context.Context, f fs.FileHandle) syscall.Errno {
	//log.Println("ENTERED FLUSH")
	if f != nil {
		return f.(fs.FileFlusher).Flush(ctx)
	}
	//log.Println("FLUSH - EBADFD")
	return syscall.EBADFD // bad file descriptor
}

// FUSE's version of a close
func (n *OptiFSNode) Release(ctx context.Context, f fs.FileHandle) syscall.Errno {
	log.Printf("ENTERED RELEASE - {%v}\n", n.RPath())

	if f != nil {

		flags := f.(*OptiFSFile).flags

		hashHashMapLock.Lock()
		// Big check here to REALLY make sure we want to perform deduplication steps
		// Flags have to have write intend AND the bytebuffer for the file can't be empty
		if !(flags&syscall.O_WRONLY == syscall.O_WRONLY || flags&syscall.O_RDWR == syscall.O_RDWR ||
			flags&syscall.O_CREAT == syscall.O_CREAT || flags&syscall.O_TRUNC == syscall.O_TRUNC ||
			flags&syscall.O_APPEND == syscall.O_APPEND) {
			hashHashMapLock.Unlock()
			log.Println("No writing intent, simply releasing file")
			return f.(*OptiFSFile).Release(ctx)
		}
		hashHashMapLock.Unlock()
		log.Println("Writing intent, continuing to perform de-duplication steps")

		// These will be defined from writeStore below, to tell who originally performed the write
		var callerUid uint32
		var callerGid uint32

		// Calculate the final hash
		log.Println("Requesting hashHashMapLock")
		hashHashMapLock.Lock()
		var newHash [64]byte
		if hashHashMap != nil {
			writeStore, ok := hashHashMap[n.RPath()]
			if ok {
				if writeStore.buffer != nil {
					newHash = hashing.HashContents(writeStore.buffer.Bytes(), 0)
				}
				callerUid = writeStore.uid
				callerGid = writeStore.gid
				log.Printf("Final hash computed for file: {%x}\n", newHash)
				// Get rid of hashmap entry
				delete(hashHashMap, n.RPath())
			} else {
				log.Println("No available hash, file must be empty!")
			}
		} else {
			log.Println("No hashHashMapLock, how did this happen?")
		}
		hashHashMapLock.Unlock()
		log.Println("Released hashHashMapLock")

		// Save the path
		nodePath := n.RPath()

		// Check if n's attributes are default
		var defaultByteArray [64]byte
		var hash [64]byte
		var ref uint64
		if n.currentHash == defaultByteArray || n.refNum == 0 {
			// Retrieve from the persisten store
			_, _, _, _, _, _, tmpHash, tmpRef := metadata.RetrieveNodeInfo(nodePath)
			hash = tmpHash
			ref = tmpRef
		} else {
			hash = n.currentHash
			ref = n.refNum
		}

		// Keep the old metadata if it exists
		err1, oldMetadata := metadata.LookupRegularFileMetadata(hash, ref)
		log.Printf("Scanned for old metadata - %v\n", err1)

		// Check to see if it's unique
		isUnique := metadata.IsContentHashUnique(newHash)
		log.Printf("Is unique: {%v}\n", isUnique)

		// If it's unique - CREATE a new MapEntry
		if isUnique {
			metadata.CreateRegularFileMapEntry(newHash)
			log.Println("Created a regular file MapEntry")
			// If it already exists, simply retrieve it
		}

		err2, entry := metadata.LookupRegularFileEntry(newHash)
		if err2 != nil {
			log.Println("MapEntry doesn't exist!")
			return fs.ToErrno(syscall.ENODATA) // return EAGAIN if we error here, not sure what is appropriate...
		}
		log.Println("Confirmed regular file MapEntry exists")

		log.Printf("Entry index num: {%v}\n", entry.IndexCounter)

		// Find an instance of a duplicate file incase we need to do de-duplication
		// This needs to be performed before the creation of a new entry, as this gets the
		// most recent entry
		rec := metadata.RetrieveRecent(entry)
		log.Printf("Searched for previous entry - {%v}\n", rec)

		// Create a new MapEntryMetadata instance
		newRef, fileMetadata := metadata.CreateRegularFileMetadata(entry)
		log.Printf("Created a new MapEntryMetadata object at refnum {%v}\n", newRef)
		// Set the file handle's refnum to the entry

		// Update our persistence hash
		metadata.UpdateNodeInfo(nodePath, nil, nil, nil, &newHash, &newRef)
		log.Println("Updated node info")

		// Perform the deduplication
		if !isUnique {

			// If it's not unique, close the file - we're getting rid of it
			f.(fs.FileReleaser).Release(ctx)

			if rec == nil {
				// Somehow we confirmed that it's not unique, but can't find the original content
				log.Println("Cannot find the original file of duplicate content!")
				metadata.RemoveRegularFileMetadata(newHash, newRef)
				return fs.ToErrno(syscall.ENOENT)
			}

			log.Println("Performing atomic linking - file isn't unique!")

			// Create a tmpFilename to perform all operations on
			tmpFilePath := nodePath + "~(TMP)"
			// Create a hardlink to tmpFileName
			linkErr := syscall.Link(rec.Path, tmpFilePath)
			if linkErr != nil {
				log.Printf("Failed to make temporary link - {%v} - exiting", linkErr)
				metadata.RemoveRegularFileMetadata(newHash, newRef)
				return fs.ToErrno(syscall.ENOLINK)
			}
			log.Printf("Created temporary link at {%v}\n", tmpFilePath)

			var st syscall.Stat_t
			statErr := syscall.Lstat(tmpFilePath, &st)
			if statErr != nil {
				// Cleanup first
				syscall.Unlink(tmpFilePath)
				metadata.RemoveRegularFileMetadata(newHash, newRef)
				log.Printf("Failed to stat link - {%v} - removing and exiting!\n", statErr)
				return fs.ToErrno(syscall.ENOENT)
			}
			log.Println("Statted the temporary link!")

			// Now rename the link onto the original file
			renErr := syscall.Rename(tmpFilePath, nodePath)
			if renErr != nil {
				// Cleanup first
				syscall.Unlink(tmpFilePath)
				metadata.RemoveRegularFileMetadata(newHash, newRef)
				log.Printf("Failed to overwrite the original file with link - {%v} - removing and exiting!\n", renErr)
				return fs.ToErrno(syscall.EIO)
			}
			log.Println("Successfuly overwrote original file with temporary link file!")

			// Update custom metadata
			if err1 == nil {
				metadata.MigrateDuplicateFileMetadata(oldMetadata, fileMetadata, &st)
				metadata.RemoveRegularFileMetadata(newHash, newRef)
				log.Println("Updated metadata through migrating old metadata")
			} else {
				log.Println("No previous metadata available, creating a new file to simulate the new file metadata")
				// Otherwise very cheeky operation - create a new file and copy the metadata to simulate the metadata
				// of a new file
				// Ensure we can create a file to copy the metadata from
				spareTmpFilePath := nodePath + "~(SPARE)"
				spareFd, spareErr := syscall.Open(spareTmpFilePath, syscall.O_CREAT|syscall.O_RDONLY, 0644)
				if spareErr != nil {
					log.Println("Failed to create new file - UH OH!")
					// TODO: figure out how to atomically revert from here or implement some kind of metadata
					return fs.ToErrno(spareErr)
				}
				syscall.Close(spareFd)
				log.Printf("Created new file at {%v}\n", spareTmpFilePath)

				// Stat the file
				var spareSt syscall.Stat_t
				spareStatErr := syscall.Stat(spareTmpFilePath, &spareSt)
				if spareStatErr != nil {
					// Clean up
					log.Println("Failed to stat new file - UH OH")
					syscall.Unlink(spareTmpFilePath)
					// TODO: figure out how to atomically revert from here or implement some kind of metadata
					return fs.ToErrno(spareStatErr)
				}
				log.Println("Performed stat on spare node")
				log.Printf("STAT -> {%+v}\n", spareSt)
				syscall.Unlink(spareTmpFilePath)
				log.Printf("Closed and removed {%v}\n", spareTmpFilePath)

				// Now update the metadata using the newly created file's metadata
				iErr := metadata.InitialiseNewDuplicateFileMetadata(fileMetadata, &spareSt, &st, nodePath, callerUid, callerGid)
				if iErr != nil {
					log.Println("Failed to apply new custom metadata - UH OH")
					// TODO: figure out how to atomically revert from here or implement some kind of metadata
				}
				log.Println("Succesfully updated custom metadata!")
			}

			log.Println("Finished de-duplicating file!")
			return fs.OK
		} else {
			log.Println("Simply updating metadata, file is unique")

			// Fill in the MapEntryMetadata object
			var st syscall.Stat_t
			serr := syscall.Fstat(f.(*OptiFSFile).fdesc, &st)
			if serr != nil {
				log.Println("Failed to stat the file")
				return fs.ToErrno(serr)
			}
			log.Println("Sucessfully statted the file!")

			if err1 == nil { // If we had old metadata, keep aspects of it
				metadata.MigrateRegularFileMetadata(oldMetadata, fileMetadata, &st)
				log.Println("Migrated old existing metadata over!")
			} else { // If we don't have old metadata, do a full copy of the underlying node's metadata

				stableAttr := n.StableAttr()
				metadata.FullMapEntryMetadataUpdate(fileMetadata, &st, &stableAttr, nodePath)
				// Double check to force the owner to be correct
				log.Printf("Setting custom owner for empty file, {%v} - {%v}\n", callerUid, callerGid)
				metadata.UpdateOwner(fileMetadata, &callerUid, &callerGid, false)
				log.Println("Performed full MapEntryMetadata update as previous metadata didn't exist!")
			}

			log.Println("Closing the file now!")

			return f.(fs.FileReleaser).Release(ctx)
		}
	}

	log.Println("RELEASE - EBADFD")

	return syscall.EBADFD // bad file descriptor
}

func (n *OptiFSNode) Fsync(ctx context.Context, f fs.FileHandle, flags uint32) syscall.Errno {
	//log.Println("ENTERED FSYNC")
	if f != nil {
		// Check write permission
		err, fileMetadata := metadata.LookupRegularFileMetadata(n.currentHash, n.refNum)
		if err == nil {
			writePerm := permissions.CheckPermissions(ctx, fileMetadata, 1)
			if !writePerm {
				return fs.ToErrno(syscall.EACCES)
			}
		}

		return f.(fs.FileFsyncer).Fsync(ctx, flags)
	}
	//log.Println("FSYNC - EBADFD")
	return syscall.EBADFD // bad file descriptor
}

// gets the status' of locks on a node by passing it to the filehandle
func (n *OptiFSNode) Getlk(ctx context.Context, f fs.FileHandle, owner uint64, lk *fuse.FileLock, flags uint32, out *fuse.FileLock) syscall.Errno {
	if f != nil {
		return f.(fs.FileGetlker).Getlk(ctx, owner, lk, flags, out) // send it if filehandle exists
	}
	//log.Println("GETLK - EBADFD")
	return syscall.EBADFD // bad file descriptor
}

// gets a lock on a node by passing it to the filehandle, if it can't get the lock it fails
func (n *OptiFSNode) Setlk(ctx context.Context, f fs.FileHandle, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	if f != nil {
		return f.(fs.FileSetlker).Setlk(ctx, owner, lk, flags) // send it if filehandle exists
	}
	//log.Println("SETLK - EBADFD")
	return syscall.EBADFD // bad file descriptor
}

// gets a lock on a node by passing it to the filehandle
// if it can't get the lock then it waits for the lock to be obtainable
func (n *OptiFSNode) Setlkw(ctx context.Context, f fs.FileHandle, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	if f != nil {
		return f.(fs.FileSetlkwer).Setlkw(ctx, owner, lk, flags) // send it if filehandle exists
	}
	//log.Println("SETLKW - EBADFD")
	return syscall.EBADFD // bad file descriptor
}

// Moves a node to a different directory. Change is only reflected in the filetree IFF returns fs.OK
// From go-fuse/fs/loopback.go
func (n *OptiFSNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {

	log.Println("Entered RENAME")

	// check if the user is allowed to rename a directory here (source and dest)
	// i.e if either src/dest are in root, are they the sysadmin?
	err := n.IsAllowedTwoLocations(ctx, newParent)
	if err != nil {
		return fs.ToErrno(err)
	}

	// Check write and exec permissions of source and target directories
	path := n.RPath()
	err1, dir1Metadata := metadata.LookupDirMetadata(path)
	err2, dir2Metadata := metadata.LookupDirMetadata(newParent.EmbeddedInode().Path(nil))
	if err1 == 0 {
		hasWrite := permissions.CheckPermissions(ctx, dir1Metadata, 1)
		if !hasWrite {
			return fs.ToErrno(syscall.EACCES)
		}
		hasExec := permissions.CheckPermissions(ctx, dir1Metadata, 2)
		if !hasExec {
			return fs.ToErrno(syscall.EACCES)
		}
	}
	if err2 == 0 {
		hasWrite := permissions.CheckPermissions(ctx, dir2Metadata, 1)
		if !hasWrite {
			return fs.ToErrno(syscall.EACCES)
		}
		hasExec := permissions.CheckPermissions(ctx, dir2Metadata, 2)
		if !hasExec {
			return fs.ToErrno(syscall.EACCES)
		}
	}

	log.Println("Have permission to rename in source and target directories!")

	// Before moving, check whether it's a directory or not
	originalPath := filepath.Join(n.RPath(), name)
	newPath := filepath.Join(n.RootNode.Path, newParent.EmbeddedInode().Path(nil), newName)
	lErr, lSIno, lSMode, lSGen, lMode, lIsDir, lHash, lRef := metadata.RetrieveNodeInfo(originalPath)
	if lErr != fs.OK {
		log.Println("Entry doesn't exist in our persistent store - big error, why doesn't it??")
		return fs.ToErrno(syscall.ENOENT)
	}

	stable := &fs.StableAttr{Ino: lSIno, Mode: lSMode, Gen: lSGen}

	var returnErr syscall.Errno
	// IFF this operation is to be done atomically (which is a more delicate operation)
	if flags&unix.RENAME_EXCHANGE != 0 {
		returnErr = n.renameExchange(name, newParent, newName)
	} else {
		// Regular rename operation if there's no RENAME_EXCHANGE flag (atomic), e.g. files between filesystems (VFS <-> Disk)
		tmp := syscall.Rename(originalPath, newPath)
		returnErr = fs.ToErrno(tmp)
		log.Println("Performed normal rename")
	}

	// Update our storages IFF we got fs.OK
	if returnErr == fs.OK {
		log.Println("Rename suceeded!")
		// Remove the old entry
		metadata.RemoveNodeInfo(originalPath)
		log.Println("Removed old persistent entry")
		if lIsDir { // If it's a directory
			metadata.StoreDirInfo(newPath, stable, lMode)
			// Copy the old metadata over since directory custom metadata is
			// indexed by path
			tmpErr, tmpMetadata := metadata.LookupDirMetadata(originalPath)
			if tmpErr != 0 {
				log.Println("PANIC AHHHHH")
				return returnErr
			}
			metadataPointer := metadata.CreateDirEntry(newPath)
			(*metadataPointer) = *tmpMetadata
			// Delete old entry
			metadata.RemoveDirEntry(originalPath)
			log.Println("Updated new dir entry")
		} else { // If it's a regfile
			metadata.StoreRegFileInfo(newPath, stable, lMode, lHash, lRef)
			log.Println("Updated new regfile entry")
		}
	}

	return returnErr
}

// Handles the name exchange of two inodes
//
// Adapted from go-fuse/fs/loopback.go
func (n *OptiFSNode) renameExchange(name string, newparent fs.InodeEmbedder, newName string) syscall.Errno {
	// Open the directory of the current node

	log.Println("in renameExchange, sensitive rename")

	path := n.RPath()

	currDirFd, err := syscall.Open(path, syscall.O_DIRECTORY, 0)
	if err != nil {
		log.Printf("Error1 - %v\n", err)
		return fs.ToErrno(err)
	}
	defer syscall.Close(currDirFd)

	// Open the new parent directory
	newParentDirPath := filepath.Join(n.RootNode.Path, newparent.EmbeddedInode().Path(nil))
	newParentDirFd, err := syscall.Open(newParentDirPath, syscall.O_DIRECTORY, 0)
	if err != nil {
		log.Printf("Error2 - %v\n", err)
		return fs.ToErrno(err)
	}
	defer syscall.Close(currDirFd)
	log.Println("Opened newParentDir")

	// Get the directory status for data integrity checks
	//var st syscall.Stat_t
	//if err := syscall.Fstat(currDirFd, &st); err != nil {
	//    log.Printf("Error3 - %v\n", err)
	//	return fs.ToErrno(err)
	//}
	// As the additional check below was removed we don't need this, best to keep it irregardless though

	inode := &n.Inode
	// Check to see if the user is trying to move the root directory, and that the inode number
	// is the same from the Fstat - ensuring the current directory hasn't been moved or modified.
	//
	// REMOVED: 'inode.StableAttr().Ino != n.RootNode.getStableAttr(&st, &path).Ino' in this check due to our
	//          custom attribute storage making it inacurrate - Do we need to replace this?
	if inode.Root() != inode {
		// Return EBUSY if there is something amiss - suggesting the resource is busy
		log.Println("Check 1 failed")
		return syscall.EBUSY
	}
	log.Println("Check 1 suceeded")

	// Check the status of the new parent directory
	//if err := syscall.Fstat(newParentDirFd, &st); err != nil {
	//	return fs.ToErrno(err)
	//}
	// As the additional check below was removed we don't need this, best to keep it irregardless though

	newParentDirInode := newparent.EmbeddedInode()
	// Ensure that the new directory isn't the root node, and that the inodes match up, same
	// consistency checks as above
	//
	// REMOVED: 'newParentDirInode.StableAttr().Ino != n.RootNode.getStableAttr(&st, &newParentDirPath).Ino' in this check
	//          due to our custom attribute storage making it inaccurate - Do we need to replace this?
	if newParentDirInode.Root() != newParentDirInode {
		log.Println("Check 2 failed")
		return syscall.EBUSY
	}
	log.Println("Check 2 suceeded")

	// Perform the actual rename operation
	// Use Renameat2, an advanced version of Rename which accepts flags, which itself is an
	// extension of the rename syscall. Use RENAME_EXCHANGE as this forces the exchange to
	// occur atomically - avoiding race conditions
	result := fs.ToErrno(unix.Renameat2(currDirFd, name, newParentDirFd, newName, unix.RENAME_EXCHANGE))
	log.Println("Performed renameat with RENAME_EXCHANGE flag")

	return result
}

// Creates a node that isn't a regular file/dir/node - like device nodes or pipes
func (n *OptiFSNode) Mknod(ctx context.Context, name string, mode uint32, dev uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	// check if the user is allowed to make a node here
	// i.e if we are in root, are they the sysadmin?
	err := n.IsAllowed(ctx)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	path := n.RPath()

	// Check the write and execute permissions of the parent directory
	err1, dirMetadata := metadata.LookupDirMetadata(path)
	if err1 == 0 {
		hasWrite := permissions.CheckPermissions(ctx, dirMetadata, 1)
		if !hasWrite {
			return nil, fs.ToErrno(syscall.EACCES)
		}
		hasExec := permissions.CheckPermissions(ctx, dirMetadata, 2)
		if !hasExec {
			return nil, fs.ToErrno(syscall.EACCES)
		}
	}

	// Create the path of the node to be created
	nodePath := filepath.Join(path, name)
	// Create the node
	if err := syscall.Mknod(nodePath, mode, int(dev)); err != nil {
		return nil, fs.ToErrno(err)
	}

	// Keep the owner
	n.setOwner(ctx, nodePath)

	st := syscall.Stat_t{}
	if err := syscall.Lstat(nodePath, &st); err != nil {
		// Kill the node if we can't Lstat it - something went wrong
		syscall.Unlink(nodePath)
		return nil, fs.ToErrno(err)
	}

	// Handle the creation of a new node
	oErr, oNode, _ := HandleNodeInstantiation(ctx, n, nodePath, name, &st, out, nil, nil)

	return oNode, oErr
}

// // Handles the creation of hardlinks
func (n *OptiFSNode) Link(ctx context.Context, target fs.InodeEmbedder, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
	// check if the user is allowed to link nodes here
	// i.e if we are in root, are they the sysadmin?
	err := n.IsAllowedTwoLocations(ctx, target)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	// Check write and execute permissions on the source directory
	err1, dirMetadata := metadata.LookupDirMetadata(target.EmbeddedInode().Path(nil))
	if err1 == 0 {
		hasWrite := permissions.CheckPermissions(ctx, dirMetadata, 1)
		if !hasWrite {
			return nil, fs.ToErrno(syscall.EACCES)
		}
		hasExec := permissions.CheckPermissions(ctx, dirMetadata, 2)
		if !hasExec {
			return nil, fs.ToErrno(syscall.EACCES)
		}
	}

	// Construct the full paths
	sourcePath := filepath.Join(n.RootNode.Path, target.EmbeddedInode().Path(nil))
	targetPath := filepath.Join(n.RPath(), name)
	if err := syscall.Link(sourcePath, targetPath); err != nil {
		return nil, fs.ToErrno(err)
	}

	// Get the status of this new hard link
	st := syscall.Stat_t{}
	if err := syscall.Stat(targetPath, &st); err != nil {
		syscall.Unlink(targetPath)
		return nil, fs.ToErrno(nil)
	}

	// Reflect this in persistent store - we don't want this hardlink to have unique metadata, it's intended to be a hardlink
	oErr, oNode := HandleHardlinkInstantiation(ctx, n, targetPath, sourcePath, name, &st, out)
	if oErr != fs.OK {
		syscall.Unlink(targetPath)
		return nil, oErr
	}

	return oNode, oErr
}

//func (n *OptiFSNode) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
//
//	// Check write and execute permissions on the target directory
//	err1, dirMetadata := metadata.LookupDirMetadata(target)
//	if err1 == nil {
//		hasWrite := permissions.CheckPermissions(ctx, dirMetadata, 1)
//		if !hasWrite {
//			return nil, fs.ToErrno(syscall.EACCES)
//		}
//		hasExec := permissions.CheckPermissions(ctx, dirMetadata, 2)
//		if !hasExec {
//			return nil, fs.ToErrno(syscall.EACCES)
//		}
//	}
//
//	// Construct the paths
//	sourcePath := filepath.Join(n.RootNode.Path, target)
//	targetPath := filepath.Join(n.RPath(), name)
//
//	// Perform the hardlink in the underlying file system
//	if err := syscall.Symlink(sourcePath, targetPath); err != nil {
//		return nil, fs.ToErrno(err)
//	}
//
//	// Set the owner to the creator
//	n.setOwner(ctx, targetPath)
//
//	st := syscall.Stat_t{}
//	if err := syscall.Lstat(targetPath, &st); err != nil {
//		syscall.Unlink(targetPath)
//		return nil, fs.ToErrno(err)
//	}
//
//    oErr, oNode, _ := HandleNodeInstantiation(ctx, n, targetPath, name, &st, out, nil, nil)
//
//    return oNode, oErr
//}
//
//// Handles reading a symlink
//func (n *OptiFSNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
//    log.Println("Entered READLINK")
//	linkPath := n.RPath()
//
//    log.Printf("link path: %v\n", linkPath)
//
//	// Keep trying to read the link, doubling our buffler size each time
//	// 256 is just an arbitrary number that isn't necessarily too large,
//	// or too small.
//	for l := 256; ; l *= 2 {
//		// Create a buffer to read the link into
//		buffer := make([]byte, l)
//		sz, err := syscall.Readlink(linkPath, buffer)
//		if err != nil {
//            log.Printf("Readlink failed! - %v\n", err)
//			return nil, fs.ToErrno(err)
//		}
//        log.Printf("Read succesfully, sz: {%v}\n", sz)
//
//		// If we fit the data into the buffer, return it
//		if sz < len(buffer) {
//            log.Printf("Returning, buffer {%x}\n", buffer[:sz])
//			return buffer[:sz], fs.OK
//		}
//	}
//}
