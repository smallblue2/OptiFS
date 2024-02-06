package node

import (
	"context"
	"filesystem/file"
	"filesystem/hashing"
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
var _ = (fs.NodeStatfser)((*OptiFSNode)(nil))      // StatFS
var _ = (fs.InodeEmbedder)((*OptiFSNode)(nil))     // Inode
var _ = (fs.NodeLookuper)((*OptiFSNode)(nil))      // lookup
var _ = (fs.NodeOpendirer)((*OptiFSNode)(nil))     // opening directories
var _ = (fs.NodeReaddirer)((*OptiFSNode)(nil))     // read directory
var _ = (fs.NodeGetattrer)((*OptiFSNode)(nil))     // get attributes of a node
var _ = (fs.NodeSetattrer)((*OptiFSNode)(nil))     // Set attributes of a node
var _ = (fs.NodeOpener)((*OptiFSNode)(nil))        // open a file
var _ = (fs.NodeGetxattrer)((*OptiFSNode)(nil))    // Get extended attributes of a node
var _ = (fs.NodeSetxattrer)((*OptiFSNode)(nil))    // Set extended attributes of a node
var _ = (fs.NodeRemovexattrer)((*OptiFSNode)(nil)) // Remove extended attributes of a node
var _ = (fs.NodeListxattrer)((*OptiFSNode)(nil))   // List extended attributes of a node
var _ = (fs.NodeMkdirer)((*OptiFSNode)(nil))       // Creates a directory
var _ = (fs.NodeCreater)((*OptiFSNode)(nil))       // creates a file
var _ = (fs.NodeUnlinker)((*OptiFSNode)(nil))      // Unlinks (deletes) a file
var _ = (fs.NodeRmdirer)((*OptiFSNode)(nil))       // Unlinks (deletes) a directory
var _ = (fs.NodeAccesser)((*OptiFSNode)(nil))      // Checks access of a node
var _ = (fs.NodeWriter)((*OptiFSNode)(nil))        // Writes to a node
var _ = (fs.NodeFlusher)((*OptiFSNode)(nil))       // Flush the node
var _ = (fs.NodeReleaser)((*OptiFSNode)(nil))      // Releases a node
var _ = (fs.NodeFsyncer)((*OptiFSNode)(nil))       // Ensures writes are actually written to disk
var _ = (fs.NodeGetlker)((*OptiFSNode)(nil))       // find conflicting locks for given lock
var _ = (fs.NodeSetlker)((*OptiFSNode)(nil))       // gets a lock on a node
var _ = (fs.NodeSetlkwer)((*OptiFSNode)(nil))      // gets a lock on a node, waits for it to be ready
var _ = (fs.NodeRenamer)((*OptiFSNode)(nil))       // Changes the directory a node is in
var _ = (fs.NodeMknoder)((*OptiFSNode)(nil))       // Similar to lookup, but creates the inode
var _ = (fs.NodeLinker)((*OptiFSNode)(nil))        // For handling hard links
var _ = (fs.NodeSymlinker)((*OptiFSNode)(nil))     // For handling hard links
var _ = (fs.NodeReadlinker)((*OptiFSNode)(nil))    // For reading symlinks

// Statfs implements statistics for the filesystem that holds this
// Inode.
func (n *OptiFSNode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	log.Println("IN STATFS")
	// As this is a loopback filesystem, we will stat the underlying filesystem.
	var s syscall.Statfs_t = syscall.Statfs_t{}
	err := syscall.Statfs(n.path(), &s)
	if err != nil {
		return fs.ToErrno(err)
	}
	out.FromStatfsT(&s)
	return fs.OK
}

// Path returns the full path to the underlying file in the underlying filesystem
func (n *OptiFSNode) path() string {
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

// Function creates a custom, unique, inode number from a file stat structure
//
// since we're using NFS, theres a chance not all Inode numbers will be unique, so we
// calculate one using bit swapping
func (n *OptiFSRoot) idFromStat(s *syscall.Stat_t) fs.StableAttr {
	// swap the higher and lower bits to generate unique no
	swapped := (uint64(s.Dev) << 32) | (uint64(s.Dev) >> 32)
	swappedRoot := (n.Dev << 32) | (n.Dev << 32)

	return fs.StableAttr{
		Mode: uint32(s.Mode),                  // Copy the underlying files perms
		Gen:  1,                               // generation number (determines lifetime of inode)
		Ino:  (swapped ^ swappedRoot) ^ s.Ino, // Unique generated inode number
	}
}

// Updates the node's metadata info, such as the contentHash, reference number, and
// the persistent info in the NodePersistenceHash
func (n *OptiFSNode) updateMetadataInfo(contentHash [64]byte, refNum uint64) {
	n.currentHash = contentHash
	n.refNum = refNum
	path := n.path()
	hashing.StoreNodeInfo(path, n.currentHash, n.refNum)
	log.Printf("Node (%v) stored currentHash (%+v) and refNum (%+v)\n", path, n.currentHash, n.refNum)
}

// get the attributes for the file hashing
func (n *OptiFSNode) GetAttr() fs.StableAttr {
	return n.StableAttr()
}

// lookup FINDS A NODE based on its name
func (n *OptiFSNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	//log.Printf("LOOKUP performed for %v from node %v\n", name, n.path())
	filePath := filepath.Join(n.path(), name) // getting the full path to the file (join name to path)
	s := syscall.Stat_t{}                     // status of a file
	err := syscall.Lstat(filePath, &s)        // gets the file attributes (also returns attrs of symbolic link)

	if err != nil {
		////log.Println("LOOKUP FAILED!")
		return nil, fs.ToErrno(err)
	}

	// Fill the output attributes from out stat struct
	out.Attr.FromStat(&s)

	// Create a new node to represent the underlying looked up file
	// or directory in our VFS
	nd := n.RootNode.newNode(n.EmbeddedInode(), name, &s)
	// Create the inode structure within FUSE, copying the underlying
	// file's attributes with an auto generated inode in idFromStat
	x := n.NewInode(ctx, nd, n.RootNode.idFromStat(&s))

	return x, fs.OK
}

// opens a directory and then closes it
func (n *OptiFSNode) Opendir(ctx context.Context) syscall.Errno {
	// Open the directory (n), 0755 is the default perms for a new directory
	dir, err := syscall.Open(n.path(), syscall.O_DIRECTORY, 0755)
	if err != nil {
		return fs.ToErrno(err)
	}
	syscall.Close(dir) // close when finished
	return fs.OK
}

// opens a stream of dir entries,
func (n *OptiFSNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	return fs.NewLoopbackDirStream(n.path())
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

	// Try and get an entry in our own custom system
	herr, metadata := hashing.LookupMetadataEntry(n.currentHash, n.refNum)
	if herr == nil {
		hashing.FillAttrOut(metadata, out)
		return fs.OK
	}

	log.Println("Statting underlying node")

	// OTHERWISE, just stat the node
	path := n.path()
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

	// If we have a file descriptor, use its setattr
	if f != nil {
		return f.(fs.FileSetattrer).Setattr(ctx, in, out)
	}

	// OTHERWISE, first try and update our own custom metadata system
	var foundEntry bool

	// Check to see if we can find an entry in our hashmap
	herr, customMetadata := hashing.LookupMetadataEntry(n.currentHash, n.refNum)
	if herr == nil {
		foundEntry = true
	}

	// If we need to - Manually change the underlying attributes ourselves
	path := n.path()

	// If the mode needs to be changed
	if mode, ok := in.GetMode(); ok {
		// Try and modify our custom metadata system first
		if foundEntry {
			hashing.UpdateMode(customMetadata, &mode)
			// Otherwise just update the underlying node's mode
		} else {
			// Change the mode to the new mode
			if err := syscall.Chmod(path, mode); err != nil {
				return fs.ToErrno(err)
			}
			log.Println("Updated underlying mode")
		}
	}

	// Try and get UID and GID
	uid, uok := in.GetUID()
	gid, gok := in.GetGID()
	// If we have a UID or GID to set
	if uok || gok {
		// Set their default values to -1
		// -1 indicates that the respective value shouldn't change
		safeUID, safeGID := -1, -1
		if uok {
			safeUID = int(uid)
		}
		if gok {
			safeGID = int(gid)
		}
		// Try and update our custom metadata system isntead
		if foundEntry {
			// As our update function works on optional pointers, convert
			// the safeguarding to work with pointers
			var saferUID *uint32
			var saferGID *uint32
			if safeUID != -1 {
				tmp := uint32(safeUID)
				saferUID = &tmp
			}
			if safeGID != -1 {
				tmp := uint32(safeGID)
				saferGID = &tmp
			}
			hashing.UpdateOwner(customMetadata, saferUID, saferGID)
			// Otherwise, just update the underlying node instead
		} else {
			// Chown these values
			err := syscall.Chown(path, safeUID, safeGID)
			if err != nil {
				return fs.ToErrno(err)
			}
			log.Println("Updated underlying UID & GID")
		}
	}

	// Same thing for modification and access times
	mtime, mok := in.GetMTime()
	atime, aok := in.GetATime()
	if mok || aok {
		// Initialize pointers to the time values
		ap := &atime
		mp := &mtime
		// Take into account if access of mod times are not both provided
		if !aok {
			ap = nil
		}
		if !mok {
			mp = nil
		}

		// Create an array to hold timespec values for syscall
		// This is a data structure that represents a time value
		// with precision up to nanoseconds
		var times [2]syscall.Timespec
		times[0] = fuse.UtimeToTimespec(ap)
		times[1] = fuse.UtimeToTimespec(mp)

		// Try and update our own custom metadata system first
		if foundEntry {
			hashing.UpdateTime(customMetadata, &times[0], &times[1], nil)
			// OTHERWISE update the underlying file
		} else {
			// Call the utimenano syscall, ensuring to convert our time array
			// into a slice, as it expects one
			if err := syscall.UtimesNano(path, times[:]); err != nil {
				return fs.ToErrno(err)
			}
			log.Println("Updated underlying ATime & MTime")
		}
	}

	// If we have a size to update, do so as well
	if size, ok := in.GetSize(); ok {
		// First try and change the custom metadata system
		if foundEntry {
			tmp := int64(size)
			hashing.UpdateSize(customMetadata, &tmp)
		} else {
			if err := syscall.Truncate(path, int64(size)); err != nil {
				return fs.ToErrno(err)
			}
			log.Println("Updated underlying size")
		}
	}

	// Now reflect these changes in the out stream
	// Use our custom datastruct if we can
	if foundEntry {
		log.Println("Reflecting custom attributes changes!")
		// Fill the AttrOut with our custom attributes stored in our hash
		hashing.FillAttrOut(customMetadata, out)

		return fs.OK
	}

	log.Println("Reflecting underlying attributes changes!")
	stat := syscall.Stat_t{}
	err := syscall.Lstat(path, &stat) // respect symlinks with lstat
	if err != nil {
		return fs.ToErrno(err)
	}
	out.FromStat(&stat)

	return fs.OK
}

// Opens a file for reading, and returns a filehandle
// flags determines how we open the file (read only, read-write, etc...)
func (n *OptiFSNode) Open(ctx context.Context, flags uint32) (f fs.FileHandle, fFlags uint32, errno syscall.Errno) {
	// Prefers an AND NOT with syscall.O_APPEND, removing it from the flags if it exists
	//log.Println("ENTERED OPEN")
	path := n.path()

    // Not sure if ACCESS is checked for opening a file
    log.Printf("\n=======================\nOpen Flags: (0x%v)\n=======================\n", strconv.FormatInt(int64(flags), 16))

    // Check custom permissions for opening the file
    // Lookup metadata entry
    herr, metadata := hashing.LookupMetadataEntry(n.currentHash, n.refNum)
    if herr == nil { // If we found custom metadata
        log.Println("Checking custom metadata for OPEN permission")
        allowed := checkOpenPermissions(ctx, metadata, flags)
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
	optiFile := file.NewOptiFSFile(fileDescriptor, n.GetAttr(), flags, n.currentHash, n.refNum)
	//log.Println("Created a new loopback file")
	return optiFile, flags, fs.OK
}

// Checks open flags against node permissions
func checkOpenPermissions(ctx context.Context, metadata *hashing.MapEntryMetadata, flags uint32) bool {

    // Extract the UID and GID from the context
    caller, check := fuse.FromContext(ctx)
    if !check {
        log.Println("No caller info available")
        return true
    }
    currentUID := uint32(caller.Uid)
    currentGID := uint32(caller.Gid)

    // Determine access writes based on the Mode
    mode := metadata.Mode
    allowed := true

    // Check to see if we're reading and/or writing
    readFlags := syscall.O_RDONLY | syscall.O_RDWR | syscall.O_SYNC
    writeFlags := syscall.O_WRONLY | syscall.O_RDWR | syscall.O_APPEND | syscall.O_DSYNC | syscall.O_FSYNC | syscall.O_DIRECT

    reading := flags&uint32(readFlags)
    writing := flags&uint32(writeFlags)

    isOwner := currentUID == metadata.Uid
    isGroup := currentGID == metadata.Gid

    // Check read permissions if necessary
    if reading != 0 {
        log.Println("Open requires reading permission")
        // Check the read permissions
        if isOwner { // IF we're the owner
            log.Println("User is the owner")
            if mode&syscall.S_IRUSR == 0 { // IF we're not allowed to read
                log.Println("Reading is not allowed")
                allowed = false
            }
            log.Println("Reading is allowed")
        } else if isGroup { // IF we're in the group
            log.Println("User is in the group")
            if mode&syscall.S_IRGRP == 0 { // IF we're not allowed to read
                log.Println("Reading is not allowed")
                allowed = false
            }
            log.Println("Reading is allowed")
        } else { // OTHERWISE we're other
            log.Println("User is other")
            if mode&syscall.S_IROTH == 0 { // IF we're not allowed to read
                log.Println("Reading is not allowed")
                allowed = false
            }
            log.Println("Reading is allowed")
        }
    }

    // Check writing permissions if necessary
    if writing != 0 {
        log.Println("Open requires writing permission")
        // Check the write permissions
        if isOwner { // IF we're the owner
            log.Println("User is the owner")
            if mode&syscall.S_IWUSR == 0 { // IF we're not allowed to write
                log.Println("Writing not allowed")
                allowed = false
            }
            log.Println("Writing allowed")
        } else if isGroup { // IF we're in the group
            log.Println("User is in the group")
            if mode&syscall.S_IWGRP == 0 { // IF we're not allowed to write
                log.Println("Writing not allowed")
                allowed = false
            }
            log.Println("Writing allowed")
        } else { // OTHERWISE we're other
            log.Println("User is other")
            if mode&syscall.S_IWOTH == 0 { // IF we're not allowed to write
                log.Println("Writing not allowed")
                allowed = false
            }
            log.Println("Writing allowed")
        }
    }

    log.Printf("Access: %v\n", allowed)
    return allowed
}

// Get EXTENDED attribute
func (n *OptiFSNode) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	//log.Println("ENTERED GETXATTR")
	// Pass it down to the filesystem below
	attributeSize, err := unix.Lgetxattr(n.path(), attr, dest)
	return uint32(attributeSize), fs.ToErrno(err)
}

// Set EXTENDED attribute
func (n *OptiFSNode) Setxattr(ctx context.Context, attr string, data []byte, flags uint32) syscall.Errno {
	//log.Println("ENTERED SETXATTR")
	// Pass it down to the filesystem below
	err := unix.Lsetxattr(n.path(), attr, data, int(flags))
	return fs.ToErrno(err)
}

// Remove EXTENDED attribute
func (n *OptiFSNode) Removexattr(ctx context.Context, attr string) syscall.Errno {
	//log.Println("ENTERED REMOVEXATTR")
	err := unix.Lremovexattr(n.path(), attr)
	return fs.ToErrno(err)
}

// List EXTENDED attributes
func (n *OptiFSNode) Listxattr(ctx context.Context, dest []byte) (uint32, syscall.Errno) {
	//log.Println("ENTERED LISTXATTR")
	// Pass it down to the filesystem below
	allAttributesSize, err := unix.Llistxattr(n.path(), dest)
	return uint32(allAttributesSize), fs.ToErrno(err)
}

// Checks access of a node
func (n *OptiFSNode) Access(ctx context.Context, mask uint32) syscall.Errno {
    log.Printf("Checking ACCESS for %v\n", n.path())
	// Prioritise custom metadata

    // Check if custom metadata exists
    err, metadata := hashing.LookupMetadataEntry(n.currentHash, n.refNum)
    if err != nil {
        // If there is no metadata, just perform a normal ACCESS on the underlying node
        log.Println("No custom metadata available, defaulting to underlying node")
        return fs.ToErrno(syscall.Access(n.path(), mask))
    }

    // Extract the UID and GID from the context
    caller, check := fuse.FromContext(ctx)
    if !check {
        log.Println("No caller info available, defaulting to underlying node")
        return fs.ToErrno(syscall.Access(n.path(), mask))
    }
    currentUID := uint32(caller.Uid)
    currentGID := uint32(caller.Gid)

    // Determine access writes based on the Mode
    mode := metadata.Mode
    var allowed bool

    switch {
    case currentUID == metadata.Uid:
        // user is the owner
        log.Println("User is the owner")
        // Don't shift the mode at all, as the bits are in the correct place already
        allowed = checkPermission(mask, mode)
        log.Printf("Owner requested %v, allowed: %v\n", mask, allowed)
    case currentGID == metadata.Gid:
        // User is in the group
        log.Println("User is in the group")
        // shift mode 3 bits to the left to line up group permission bits to be under where user bits usually are
        allowed = checkPermission(mask, mode<<3)
        log.Printf("Group member requested %v, allowed: %v\n", mask, allowed)
    default:
        // Check for others permissions
        log.Println("User is under others")
        // shift mode 6 bits to the left to line up other permission bits to be under where user bits usually are
        allowed = checkPermission(mask, mode<<6)
        log.Printf("Other member requested %v, allowed: %v\n", mask, allowed)
    }

    if !allowed {
        log.Println("NOT ALLOWED")
        return syscall.EACCES
    }

    log.Println("ALLOWED")
    return fs.OK
}

// Checks if the requested access is allowed based on the leftmost bits (user)
func checkPermission(mask, mode uint32) bool {
	// Read permission check
	// If the mask AND'd with 4 (requesting read access), AND the mode AND'd with S_IRUSR == 0 (file doesn't allow read access)
	if mask&4 > 0 && mode&syscall.S_IRUSR == 0 {
        return false
	}
    // Write permission check
    // If the mask AND'd with 2 (requesting write access), AND the mode AND'd with S_IWUSR == 0 (file doesn't allow write access)
    if mask&2 > 0 && mode&syscall.S_IWUSR == 0 {
        return false
    }
    // Execute permission check
    // If the mask AND'd with 1 (requesting exec access), AND the mode AND'd with S_IXUSR == 0 (file doesn't allow exec access)
    if mask&1 > 0 && mode&syscall.S_IXUSR == 0 {
        return false
    }
    return true
}


// Make a directory
func (n *OptiFSNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	//log.Println("ENTERED MKDIR")
	// Create the directory
	filePath := filepath.Join(n.path(), name)
	err := syscall.Mkdir(filePath, mode)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	// Now stat the new directory, ensuring it was created
	var directoryStatus syscall.Stat_t
	err = syscall.Stat(filePath, &directoryStatus)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	// Fill the output attributes from out stat struct
	out.Attr.FromStat(&directoryStatus)

	// Create a new node to represent the underlying looked up file
	// or directory in our VFS
	nd := n.RootNode.newNode(n.EmbeddedInode(), name, &directoryStatus)

	// Create the inode structure within FUSE, copying the underlying
	// file's attributes with an auto generated inode in idFromStat
	x := n.NewInode(ctx, nd, n.RootNode.idFromStat(&directoryStatus))

	return x, fs.OK
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
	filePath := filepath.Join(n.path(), name) // create the path for the new file

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

	// Create a new node to represent the underlying looked up file
	// or directory in our VFS
	nd := n.RootNode.newNode(n.EmbeddedInode(), name, &s)

	// Create the inode structure within FUSE, copying the underlying
	// file's attributes with an auto generated inode in idFromStat
	x := n.NewInode(ctx, nd, n.RootNode.idFromStat(&s))

	newFile := file.NewOptiFSFile(fdesc, n.GetAttr(), flags, n.currentHash, n.refNum) // make filehandle for file operations

	out.FromStat(&s) // fill out info

	return x, newFile, 0, fs.OK
}

// Unlinks (removes) a file
func (n *OptiFSNode) Unlink(ctx context.Context, name string) syscall.Errno {
	log.Printf("UNLINK performed on %v from node %v\n", name, n.path())

	// Flag for custom metadata existing
	var customExists bool

	// Construct the file's path since 'n' is actually the parent directory
	filePath := filepath.Join(n.path(), name)

	// Since 'n' is actually the parent directory, we need to retrieve the underlying node to search
	// for custom metadata to cleanup
	herr, contentHash, refNum := hashing.RetrieveNodeInfo(filePath)
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
		hashing.RemoveMetadata(contentHash, refNum)
		hashing.RemoveNodeInfo(filePath)
	}

	return fs.ToErrno(err)
}

// Unlinks (removes) a directory
func (n *OptiFSNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	//log.Printf("RMDIR performed on %v from node %v\n", name, n.path())
	filePath := filepath.Join(n.path(), name)
	err := syscall.Rmdir(filePath)
	return fs.ToErrno(err)
}

func (n *OptiFSNode) Write(ctx context.Context, f fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	//log.Println("ENTERED WRITE")
	if f != nil {
		written, errno := f.(fs.FileWriter).Write(ctx, data, off)
		// Update the node's metadata info with the file's current hash and refnum
		n.updateMetadataInfo(f.(*file.OptiFSFile).CurrentHash, f.(*file.OptiFSFile).RefNum)
		return written, errno
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
	// IFF this operation is to be done atomically (which is a far more delicate operation)
	if flags&unix.RENAME_EXCHANGE != 0 {
		n.renameExchange(name, newParent, newName)
	}

	// Regular rename operation if there's no RENAME_EXCHANGE flag (atomic), e.g. files between filesystems (VFS <-> Disk)
	p1 := filepath.Join(n.path(), name)
	p2 := filepath.Join(n.RootNode.Path, newParent.EmbeddedInode().Path(nil), newName)

	err := syscall.Rename(p1, p2)
	return fs.ToErrno(err)
}

// Handles the name exchange of two inodes
//
// Adapted from go-fuse/fs/loopback.go
func (n *OptiFSNode) renameExchange(name string, newparent fs.InodeEmbedder, newName string) syscall.Errno {
	// Open the directory of the current node
	currDirFd, err := syscall.Open(n.path(), syscall.O_DIRECTORY, 0)
	if err != nil {
		return fs.ToErrno(err)
	}
	defer syscall.Close(currDirFd)

	// Open the new parent directory
	newParentDirPath := filepath.Join(n.RootNode.Path, newparent.EmbeddedInode().Path(nil))
	newParentDirFd, err := syscall.Open(newParentDirPath, syscall.O_DIRECTORY, 0)
	if err != nil {
		return fs.ToErrno(err)
	}
	defer syscall.Close(currDirFd)

	// Get the directory status for data integrity checks
	var st syscall.Stat_t
	if err := syscall.Fstat(currDirFd, &st); err != nil {
		return fs.ToErrno(err)
	}

	inode := &n.Inode
	// Check to see if the user is trying to move the root directory, and that the inode number
	// is the same from the Fstat - ensuring the current directory hasn't been moved or modified.
	if inode.Root() != inode && inode.StableAttr().Ino != n.RootNode.idFromStat(&st).Ino {
		// Return EBUSY if there is something amiss - suggesting the resource is busy
		return syscall.EBUSY
	}

	// Check the status of the new parent directory
	if err := syscall.Fstat(newParentDirFd, &st); err != nil {
		return fs.ToErrno(err)
	}

	newParentDirInode := newparent.EmbeddedInode()
	// Ensure that the new directory isn't the root node, and that the inodes match up, same
	// consistency checks as above
	if newParentDirInode.Root() != newParentDirInode && newParentDirInode.StableAttr().Ino != n.RootNode.idFromStat(&st).Ino {
		return syscall.EBUSY
	}

	// Perform the actual rename operation
	// Use Renameat2, an advanced version of Rename which accepts flags, which itself is an
	// extension of the rename syscall. Use RENAME_EXCHANGE as this forces the exchange to
	// occur atomically - avoiding race conditions
	return fs.ToErrno(unix.Renameat2(currDirFd, name, newParentDirFd, newName, unix.RENAME_EXCHANGE))
}

// Creates a node that isn't a regular file/dir/node - like device nodes or pipes
func (n *OptiFSNode) Mknod(ctx context.Context, name string, mode uint32, dev uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	// Create the path of the node to be created
	nodePath := filepath.Join(n.path(), name)
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

	// Fill in the out attributes
	out.Attr.FromStat(&st)

	// Create a fuse node to represent this new node
	newNode := n.RootNode.newNode(n.EmbeddedInode(), name, &st)

	// Actually create the node within FUSE
	x := n.NewInode(ctx, newNode, n.RootNode.idFromStat(&st))

	return x, fs.OK
}

// Handles the creation of hardlinks
func (n *OptiFSNode) Link(ctx context.Context, target fs.InodeEmbedder, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
	// Construct the full paths
	sourcePath := filepath.Join(n.RootNode.Path, target.EmbeddedInode().Path(nil))
	targetPath := filepath.Join(n.path(), name)
	if err := syscall.Link(sourcePath, targetPath); err != nil {
		return nil, fs.ToErrno(err)
	}

	// Get the status of this new hard link
	st := syscall.Stat_t{}
	if err := syscall.Stat(targetPath, &st); err != nil {
		syscall.Unlink(targetPath)
		return nil, fs.ToErrno(nil)
	}

	// Fill in the out attributes
	out.Attr.FromStat(&st)

	// Actually create the fuse node to represent this link
	newNode := n.RootNode.newNode(n.EmbeddedInode(), name, &st)
	x := n.NewInode(ctx, newNode, n.RootNode.idFromStat(&st))

	return x, fs.OK
}

func (n *OptiFSNode) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
	// Construct the paths
	sourcePath := filepath.Join(n.RootNode.Path, target)
	targetPath := filepath.Join(n.path(), name)

	// Perform the hardlink in the underlying file system
	if err := syscall.Symlink(sourcePath, targetPath); err != nil {
		return nil, fs.ToErrno(err)
	}

	// Set the owner to the creator
	n.setOwner(ctx, targetPath)

	st := syscall.Stat_t{}
	if err := syscall.Lstat(targetPath, &st); err != nil {
		syscall.Unlink(targetPath)
		return nil, fs.ToErrno(err)
	}

	// Fill the attributes in out
	out.Attr.FromStat(&st)

	// Create the FUSE node to represent the symlink
	newNode := n.RootNode.newNode(n.EmbeddedInode(), name, &st)
	x := n.NewInode(ctx, newNode, n.RootNode.idFromStat(&st))

	return x, fs.OK
}

// Handles reading a symlink
func (n *OptiFSNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	linkPath := n.path()

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
