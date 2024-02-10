package vfs

import (
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
// var _ = (fs.NodeRenamer)((*OptiFSNode)(nil))    // Changes the directory a node is in
// var _ = (fs.NodeMknoder)((*OptiFSNode)(nil))    // Similar to lookup, but creates the inode
// var _ = (fs.NodeLinker)((*OptiFSNode)(nil))     // For handling hard links
// var _ = (fs.NodeSymlinker)((*OptiFSNode)(nil))  // For handling hard links
var _ = (fs.NodeReadlinker)((*OptiFSNode)(nil)) // For reading symlinks

// Statfs implements statistics for the filesystem that holds this
// Inode.
func (n *OptiFSNode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	log.Println("IN STATFS")
	// As this is a loopback filesystem, we will stat the underlying filesystem.
	var s syscall.Statfs_t = syscall.Statfs_t{}
	err := syscall.Statfs(n.RPath(), &s)
	if err != nil {
		return fs.ToErrno(err)
	}
	out.FromStatfsT(&s)
	return fs.OK
}

// Path returns the full RPath to the underlying file in the underlying filesystem
func (n *OptiFSNode) RPath() string {
	// Get 'n's node's path relative to OptiFS's root
	var path string = n.Path(n.Root())
	return filepath.Join(n.RootNode.Path, path)
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
	if err1 == nil {
		log.Println("Checking directory custom permissions")
		isAllowed := permissions.CheckPermissions(ctx, dirMetadata, 2) // Check exec permissions
		if !isAllowed {
			log.Println("Not allowed!")
			return nil, fs.ToErrno(syscall.EACCES)
		}
		log.Println("Not allowed!")
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

	path := n.RPath()
	log.Printf("Opening directory '%v'\n", path)

	// Check exec permissions if there is custom metadata
	err1, dirMetadata := metadata.LookupDirMetadata(path)
	if err1 == nil {
		log.Println("Checking directory custom permissions")
		isAllowed := permissions.CheckPermissions(ctx, dirMetadata, 2) // Check exec permissions
		if !isAllowed {
			log.Println("Not allowed!")
			return fs.ToErrno(syscall.EACCES)
		}
		log.Println("Not allowed!")
	}

	// Open the directory (n), 0755 is the default perms for a new directory
	dir, err2 := syscall.Open(n.RPath(), syscall.O_DIRECTORY, 0755)
	if err2 != nil {
		return fs.ToErrno(err2)
	}
	syscall.Close(dir) // close when finished
	return fs.OK
}

// opens a stream of dir entries,
func (n *OptiFSNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {

	path := n.RPath()

	// Check read permissions if available
	err1, dirMetadata := metadata.LookupDirMetadata(path)
	if err1 == nil {
		isAllowed := permissions.CheckPermissions(ctx, dirMetadata, 0)
		if !isAllowed {
			return nil, fs.ToErrno(syscall.EACCES)
		}
		log.Println("Not allowed!")
	}

	return fs.NewLoopbackDirStream(path)
}

// get the attributes of a file/dir, either with a filehandle (if passed) or through inodes
func (n *OptiFSNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	log.Println("NODE || entered GETATTR")
	// if we have a file handle, use it to get the attributes
	if f != nil {
		log.Println("Filehandle isn't nil!")
		return f.(fs.FileGetattrer).Getattr(ctx, out)
	}

	log.Println("Filehandle is nil, using node!")

	path := n.RPath()

	// Try and get an entry in our own custom system
	err1, fileMetadata := metadata.LookupRegularFileMetadata(n.currentHash, n.refNum)
	if err1 == nil { // If it exists
		metadata.FillAttrOut(fileMetadata, out)
		return fs.OK
	}
	err2, dirMetadata := metadata.LookupDirMetadata(path)
	if err2 == nil {
		metadata.FillAttrOut(dirMetadata, out)
		return fs.OK
	}

	log.Println("Statting underlying node")

	// OTHERWISE, just stat the node
	var err error
	s := syscall.Stat_t{}
	// IF we're dealing with the root, stat it directly as opposed to handling symlinks
	if &n.Inode == n.Root() {
		err = syscall.Stat(path, &s) // if we are looking for the root of FS
	} else {
		// Otherwise, use Lstat to handle symlinks as well as normal files/directories
		err = syscall.Lstat(path, &s) // if it's just a normal file/dir
	}

	if err != nil {
		return fs.ToErrno(err)
	}

	out.FromStat(&s) // no error getting attrs, fill them in out
	return fs.OK
}

// Sets attributes of a node
func (n *OptiFSNode) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {

	log.Println("NODE || entered SETATTR")

	// Check to see if we can find an entry in our node hashmap
	err1, fileMetadata := metadata.LookupRegularFileMetadata(n.currentHash, n.refNum)
	if err1 == nil {
		log.Println("Setting attributes for custom regular file metadata.")
		if f != nil {
			return fs.ToErrno(SetAttributes(ctx, fileMetadata, in, n, f.(*OptiFSFile), out))
		}
		return fs.ToErrno(SetAttributes(ctx, fileMetadata, in, n, nil, out))
	}
	// Also check to see if we can find an entry in our directory hashmap
	err2, dirMetadata := metadata.LookupDirMetadata(n.RPath())
	if err2 == nil {
		log.Println("Setting attributes for custom directory metadata.")
		if f != nil {
			return fs.ToErrno(SetAttributes(ctx, dirMetadata, in, n, f.(*OptiFSFile), out))
		}
		return fs.ToErrno(SetAttributes(ctx, dirMetadata, in, n, nil, out))
	}

	// Otherwise, neither exists; just do underlying node
	log.Println("Setting attributes for underlying node.")
	return fs.ToErrno(SetAttributes(ctx, nil, in, n, nil, out))
}

// Opens a file for reading, and returns a filehandle
// flags determines how we open the file (read only, read-write, etc...)
func (n *OptiFSNode) Open(ctx context.Context, flags uint32) (f fs.FileHandle, fFlags uint32, errno syscall.Errno) {

	log.Println("ENTERED OPEN")

	path := n.RPath()

	// Not sure if ACCESS is checked for opening a file
	log.Printf("\n=======================\nOpen Flags: (0x%v)\n=======================\n", strconv.FormatInt(int64(flags), 16))

	// Check custom permissions for opening the file
	// Lookup metadata entry
	herr, fileMetadata := metadata.LookupRegularFileMetadata(n.currentHash, n.refNum)
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
	optiFile := NewOptiFSFile(fileDescriptor, n.GetAttr(), flags, n.currentHash, n.refNum)
	//log.Println("Created a new loopback file")
	return optiFile, flags, fs.OK
}

// Get EXTENDED attribute
func (n *OptiFSNode) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	//log.Println("ENTERED GETXATTR")

	// Check if the user has read access
	err1, nodeMetadata := metadata.LookupRegularFileMetadata(n.currentHash, n.refNum)
	if err1 == nil {
		hasRead := permissions.CheckPermissions(ctx, nodeMetadata, 0)
		if !hasRead {
			return 0, syscall.EACCES
		}
	}
	path := n.RPath()
	err2, dirMetadata := metadata.LookupDirMetadata(path)
	if err2 == nil {
		hasRead := permissions.CheckPermissions(ctx, dirMetadata, 0)
		if !hasRead {
			return 0, syscall.EACCES
		}
	}

	// Pass it down to the filesystem below
	attributeSize, err3 := unix.Lgetxattr(path, attr, dest)
	return uint32(attributeSize), fs.ToErrno(err3)
}

// Set EXTENDED attribute
func (n *OptiFSNode) Setxattr(ctx context.Context, attr string, data []byte, flags uint32) syscall.Errno {
	//log.Println("ENTERED SETXATTR")

	// Check if the user has write access
	err1, nodeMetadata := metadata.LookupRegularFileMetadata(n.currentHash, n.refNum)
	if err1 == nil {
		hasWrite := permissions.CheckPermissions(ctx, nodeMetadata, 1)
		if !hasWrite {
			return syscall.EACCES
		}
	}
	path := n.RPath()
	err2, dirMetadata := metadata.LookupDirMetadata(path)
	if err2 == nil {
		hasWrite := permissions.CheckPermissions(ctx, dirMetadata, 1)
		if !hasWrite {
			return syscall.EACCES
		}
	}

	// Pass it down to the filesystem below
	err3 := unix.Lsetxattr(path, attr, data, int(flags))
	return fs.ToErrno(err3)
}

// Remove EXTENDED attribute
func (n *OptiFSNode) Removexattr(ctx context.Context, attr string) syscall.Errno {
	//log.Println("ENTERED REMOVEXATTR")

	// Check if the user has write access
	err1, nodeMetadata := metadata.LookupRegularFileMetadata(n.currentHash, n.refNum)
	if err1 == nil {
		hasWrite := permissions.CheckPermissions(ctx, nodeMetadata, 1)
		if !hasWrite {
			return syscall.EACCES
		}
	}
	path := n.RPath()
	err2, dirMetadata := metadata.LookupDirMetadata(path)
	if err2 == nil {
		hasWrite := permissions.CheckPermissions(ctx, dirMetadata, 1)
		if !hasWrite {
			return syscall.EACCES
		}
	}

	err3 := unix.Lremovexattr(path, attr)
	return fs.ToErrno(err3)
}

// List EXTENDED attributes
func (n *OptiFSNode) Listxattr(ctx context.Context, dest []byte) (uint32, syscall.Errno) {
	//log.Println("ENTERED LISTXATTR")

	// Check if the user has read access
	err1, nodeMetadata := metadata.LookupRegularFileMetadata(n.currentHash, n.refNum)
	if err1 == nil {
		hasRead := permissions.CheckPermissions(ctx, nodeMetadata, 0)
		if !hasRead {
			return 0, syscall.EACCES
		}
	}
	path := n.RPath()
	err2, dirMetadata := metadata.LookupDirMetadata(path)
	if err2 == nil {
		hasRead := permissions.CheckPermissions(ctx, dirMetadata, 0)
		if !hasRead {
			return 0, syscall.EACCES
		}
	}

	// Pass it down to the filesystem below
	allAttributesSize, err3 := unix.Llistxattr(path, dest)
	return uint32(allAttributesSize), fs.ToErrno(err3)
}

// Checks access of a node
func (n *OptiFSNode) Access(ctx context.Context, mask uint32) syscall.Errno {
	log.Printf("Checking ACCESS for %v\n", n.RPath())
	// Prioritise custom metadata

	// Check if custom metadata exists for a regular file
	if err, fileMetadata := metadata.LookupRegularFileMetadata(n.currentHash, n.refNum); err == nil {
		// If there is no metadata, just perform a normal ACCESS on the underlying node
		log.Println("Found custom regular file metadata, checking...")
		isAllowed := permissions.CheckMask(ctx, mask, fileMetadata)
		if !isAllowed {
			return fs.ToErrno(syscall.EACCES)
		}
	}

	// Check if custom metadata exists for a directory
	path := n.RPath()
	if err, dirMetadata := metadata.LookupDirMetadata(path); err == nil {
		log.Println("Found custom directory metadata, checking...")
		isAllowed := permissions.CheckMask(ctx, mask, dirMetadata)
		if !isAllowed {
			return fs.ToErrno(syscall.EACCES)
		}
	}

	// Otherwise, default the access to underlying filesystem
	return fs.ToErrno(syscall.Access(n.RPath(), mask))
}

// Make a directory
func (n *OptiFSNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Println("ENTERED MKDIR")

	filePath := filepath.Join(n.RPath(), name)

	// Check write and execute permissions on the parent directory
	err1, dirMetadata := metadata.LookupDirMetadata(n.RPath())
	if err1 == nil {
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
	if os.Getuid() != 0 {
		return nil
	}

	person, check := fuse.FromContext(ctx) // get person's info
	// if we werent able to get the info of the person who performed the operation
	if !check {
		return nil
	}

	// change the ownership of the file/dir to the UID and GID of the person
	return syscall.Lchown(path, int(person.Uid), int(person.Gid))

}

// create a REGULAR FILE that doesn't exist, also fills in the gid/uid of the user into the file attributes
func (n *OptiFSNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (inode *fs.Inode, f fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {

	// Check write and exec permissions on the parent directory
	err1, dirMetadata := metadata.LookupDirMetadata(n.RPath())
	if err1 == nil {
		log.Println("Checking directory custom permissions")
		writePerm := permissions.CheckPermissions(ctx, dirMetadata, 1) // Check for write permissions
		if !writePerm {
			log.Println("Not allowed!")
			return nil, nil, 0, fs.ToErrno(syscall.EACCES)
		}
		execPerm := permissions.CheckPermissions(ctx, dirMetadata, 2) // Check exec permissions
		if !execPerm {
			log.Println("Not allowed!")
			return nil, nil, 0, fs.ToErrno(syscall.EACCES)
		}
	}

	filePath := filepath.Join(n.RPath(), name) // create the path for the new file

	// try to open the file, OR create if theres no file to open
	fdesc, err := syscall.Open(filePath, int(flags)|os.O_CREATE, mode)

	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
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
	log.Printf("UNLINK performed on %v from node %v\n", name, n.RPath())

	// Check write and exec permissions on the parent directory
	err1, dirMetadata := metadata.LookupDirMetadata(n.RPath())
	if err1 == nil {
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
	herr, _, _, _, contentHash, refNum := metadata.RetrieveNodeInfo(filePath)
	if herr == nil {
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

	// Check exec and write permissions on the parent directory
	// Don't need to check the directory being removed, as it must be empty to be removed
	err1, dirMetadata := metadata.LookupDirMetadata(n.RPath())
	if err1 == nil {
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

	err := syscall.Rmdir(filePath)
	return fs.ToErrno(err)
}

func (n *OptiFSNode) Write(ctx context.Context, f fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	//log.Println("ENTERED WRITE")
	if f != nil {
		// Check if we have permission to write to the file
		err, fileMetadata := metadata.LookupRegularFileMetadata(n.currentHash, n.refNum)
		if err == nil { // if it exists
			writePerm := permissions.CheckPermissions(ctx, fileMetadata, 1) // check write perm
			if !writePerm {
				return 0, syscall.EACCES
			}
		}

		// Asset that the file is an OptiFSFile
		OptiF := f.(*OptiFSFile)

		OptiF.mu.Lock()
		defer OptiF.mu.Unlock()

		// Keep the old metadata if it exists
		err1, oldMetadata := metadata.LookupRegularFileMetadata(OptiF.currentHash, OptiF.refNum)

		// Hash the current contents
		OptiF.currentHash = hashing.HashContents(data, OptiF.flags)
		// Check to see if it's unique
		isUnique, _ := metadata.IsContentHashUnique(OptiF.currentHash)

		// TODO: I think it should only be created in Create syscall, not write - or maybe not idk, need to think

		// If it's unique - CREATE a new MapEntry
		if isUnique {
			metadata.CreateRegularFileMapEntry(OptiF.currentHash)
			// If it already exists, simply retrieve it
		}

		err2, entry := metadata.LookupRegularFileEntry(OptiF.currentHash)
		if err2 != nil {
			return 0, fs.ToErrno(syscall.EAGAIN) // return EAGAIN if we error here, not sure what is appropriate...
		}

		// Create a new MapEntryMetadata instance
		refNum, fileMetadata := metadata.CreateRegularFileMetadata(entry)
		// Set the file handle's refnum to the entry
		OptiF.refNum = refNum

		// Update our persistence hash
		metadata.UpdateNodeInfo(n.RPath(), nil, nil, nil, &OptiF.currentHash, &OptiF.refNum)

		// Perform the write

		// TODO: Set up links if non-unique NEEDS TO BE ATOMIC ( gon' be hard :( )
		numOfBytesWritten, werr := syscall.Pwrite(OptiF.fdesc, data, off)

		// Fill in the MapEntryMetadata object
		var st syscall.Stat_t
		serr := syscall.Fstat(OptiF.fdesc, &st)
		if serr != nil {
			return 0, fs.ToErrno(serr)
		}

		if err1 == nil { // If we had old metadata, keep aspects of it
			metadata.MigrateRegularFileMetadata(oldMetadata, fileMetadata, &st)
		} else { // If we don't have old metadata, do a full copy of the underlying node's metadata
			stableAttr := n.StableAttr()
			metadata.FullMapEntryMetadataUpdate(fileMetadata, &st, &stableAttr)
		}

		return uint32(numOfBytesWritten), fs.ToErrno(werr)
	}

	//log.Println("WRITE - EBADFD")
	return 0, syscall.EBADFD // bad file descriptor
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
	//log.Println("ENTERED RELEASE")
	if f != nil {
		return f.(fs.FileReleaser).Release(ctx)
	}
	//log.Println("RELEASE - EBADFD")
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
//func (n *OptiFSNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
//	// Check write and exec permissions of source and target directories
//	path := n.RPath()
//	err1, dir1Metadata := metadata.LookupDirMetadata(path)
//	err2, dir2Metadata := metadata.LookupDirMetadata(newParent.EmbeddedInode().Path(nil))
//	if err1 == nil {
//		hasWrite := permissions.CheckPermissions(ctx, dir1Metadata, 1)
//		if !hasWrite {
//			return fs.ToErrno(syscall.EACCES)
//		}
//		hasExec := permissions.CheckPermissions(ctx, dir1Metadata, 2)
//		if !hasExec {
//			return fs.ToErrno(syscall.EACCES)
//		}
//	}
//	if err2 == nil {
//		hasWrite := permissions.CheckPermissions(ctx, dir2Metadata, 1)
//		if !hasWrite {
//			return fs.ToErrno(syscall.EACCES)
//		}
//		hasExec := permissions.CheckPermissions(ctx, dir2Metadata, 2)
//		if !hasExec {
//			return fs.ToErrno(syscall.EACCES)
//		}
//	}
//
//	// IFF this operation is to be done atomically (which is a far more delicate operation)
//	if flags&unix.RENAME_EXCHANGE != 0 {
//		n.renameExchange(name, newParent, newName)
//	}
//
//	// Regular rename operation if there's no RENAME_EXCHANGE flag (atomic), e.g. files between filesystems (VFS <-> Disk)
//	p1 := filepath.Join(n.RPath(), name)
//	p2 := filepath.Join(n.RootNode.Path, newParent.EmbeddedInode().Path(nil), newName)
//
//	err3 := syscall.Rename(p1, p2)
//	return fs.ToErrno(err3)
//}

// Handles the name exchange of two inodes
//
// Adapted from go-fuse/fs/loopback.go
//func (n *OptiFSNode) renameExchange(name string, newparent fs.InodeEmbedder, newName string) syscall.Errno {
//	// Open the directory of the current node
//
//    path := n.RPath()
//
//	currDirFd, err := syscall.Open(path, syscall.O_DIRECTORY, 0)
//	if err != nil {
//		return fs.ToErrno(err)
//	}
//	defer syscall.Close(currDirFd)
//
//	// Open the new parent directory
//	newParentDirPath := filepath.Join(n.RootNode.Path, newparent.EmbeddedInode().Path(nil))
//	newParentDirFd, err := syscall.Open(newParentDirPath, syscall.O_DIRECTORY, 0)
//	if err != nil {
//		return fs.ToErrno(err)
//	}
//	defer syscall.Close(currDirFd)
//
//	// Get the directory status for data integrity checks
//	var st syscall.Stat_t
//	if err := syscall.Fstat(currDirFd, &st); err != nil {
//		return fs.ToErrno(err)
//	}
//
//	inode := &n.Inode
//	// Check to see if the user is trying to move the root directory, and that the inode number
//	// is the same from the Fstat - ensuring the current directory hasn't been moved or modified.
//	if inode.Root() != inode && inode.StableAttr().Ino != n.RootNode.getStableAttr(&st, &path).Ino {
//		// Return EBUSY if there is something amiss - suggesting the resource is busy
//		return syscall.EBUSY
//	}
//
//	// Check the status of the new parent directory
//	if err := syscall.Fstat(newParentDirFd, &st); err != nil {
//		return fs.ToErrno(err)
//	}
//
//	newParentDirInode := newparent.EmbeddedInode()
//	// Ensure that the new directory isn't the root node, and that the inodes match up, same
//	// consistency checks as above
//	if newParentDirInode.Root() != newParentDirInode && newParentDirInode.StableAttr().Ino != n.RootNode.getStableAttr(&st, &newParentDirPath).Ino {
//		return syscall.EBUSY
//	}
//
//	// Perform the actual rename operation
//	// Use Renameat2, an advanced version of Rename which accepts flags, which itself is an
//	// extension of the rename syscall. Use RENAME_EXCHANGE as this forces the exchange to
//	// occur atomically - avoiding race conditions
//	return fs.ToErrno(unix.Renameat2(currDirFd, name, newParentDirFd, newName, unix.RENAME_EXCHANGE))
//}

// Creates a node that isn't a regular file/dir/node - like device nodes or pipes
//func (n *OptiFSNode) Mknod(ctx context.Context, name string, mode uint32, dev uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
//
//	path := n.RPath()
//
//	// Check the write and execute permissions of the parent directory
//	err1, dirMetadata := metadata.LookupDirMetadata(path)
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
//	// Create the path of the node to be created
//	nodePath := filepath.Join(path, name)
//	// Create the node
//	if err := syscall.Mknod(nodePath, mode, int(dev)); err != nil {
//		return nil, fs.ToErrno(err)
//	}
//
//	// Keep the owner
//	n.setOwner(ctx, nodePath)
//
//	st := syscall.Stat_t{}
//	if err := syscall.Lstat(nodePath, &st); err != nil {
//		// Kill the node if we can't Lstat it - something went wrong
//		syscall.Unlink(nodePath)
//		return nil, fs.ToErrno(err)
//	}
//
//	// Fill in the out attributes
//	out.Attr.FromStat(&st)
//
//	// Create a fuse node to represent this new node
//	newNode := n.RootNode.newNode(n.EmbeddedInode(), name, &st)
//
//	// Actually create the node within FUSE
//	x := n.NewInode(ctx, newNode, n.RootNode.getStableAttr(&st, &nodePath))
//
//	return x, fs.OK
//}
//
//// Handles the creation of hardlinks
//func (n *OptiFSNode) Link(ctx context.Context, target fs.InodeEmbedder, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
//	// Check write and execute permissions on the source directory
//	err1, dirMetadata := metadata.LookupDirMetadata(target.EmbeddedInode().Path(nil))
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
//	// Construct the full paths
//	sourcePath := filepath.Join(n.RootNode.Path, target.EmbeddedInode().Path(nil))
//	targetPath := filepath.Join(n.RPath(), name)
//	if err := syscall.Link(sourcePath, targetPath); err != nil {
//		return nil, fs.ToErrno(err)
//	}
//
//	// Get the status of this new hard link
//	st := syscall.Stat_t{}
//	if err := syscall.Stat(targetPath, &st); err != nil {
//		syscall.Unlink(targetPath)
//		return nil, fs.ToErrno(nil)
//	}
//
//	// Fill in the out attributes
//	out.Attr.FromStat(&st)
//
//	// Actually create the fuse node to represent this link
//	newNode := n.RootNode.newNode(n.EmbeddedInode(), name, &st)
//	x := n.NewInode(ctx, newNode, n.RootNode.getStableAttr(&st, &targetPath))
//
//	return x, fs.OK
//}
//
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
//	// Fill the attributes in out
//	out.Attr.FromStat(&st)
//
//	// Create the FUSE node to represent the symlink
//	newNode := n.RootNode.newNode(n.EmbeddedInode(), name, &st)
//	x := n.NewInode(ctx, newNode, n.RootNode.getStableAttr(&st, &targetPath))
//
//	return x, fs.OK
//}

// Handles reading a symlink
func (n *OptiFSNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	linkPath := n.RPath()

	// Keep trying to read the link, doubling our buffler size each time
	// 256 is just an arbitrary number that isn't necessarily too large,
	// or too small.
	for l := 256; ; l *= 2 {
		// Create a buffer to read the link into
		buffer := make([]byte, l)
		sz, err := syscall.Readlink(linkPath, buffer)
		if err != nil {
			return nil, fs.ToErrno(err)
		}

		// If we fit the data into the buffer, return it
		if sz < len(buffer) {
			return buffer[:sz], fs.OK
		}
	}
}
