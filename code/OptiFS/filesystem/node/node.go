package node

import (
	"context"
	"filesystem/file"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
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

// Statfs implements statistics for the filesystem that holds this
// Inode.
func (n *OptiFSNode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
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

// get the inode number for the file hashing
func (n *OptiFSNode) GetInode() uint64 {
	attr := n.StableAttr()
	return attr.Ino
}

// lookup finds a file/directory based on its name
func (n *OptiFSNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Printf("LOOKUP performed for %v from node %v\n", name, n.path())
	fp := filepath.Join(n.path(), name) // getting the full path to the file (join name to path)
	s := syscall.Stat_t{}               // status of a file
	err := syscall.Lstat(fp, &s)        // gets the file attributes (also returns attrs of symbolic link)

	if err != nil {
		log.Println("LOOKUP FAILED!")
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
	log.Println("ENTERED GETATTR")
	// if we have a file handle, use it to get the attributes
	if f != nil {
		return f.(fs.FileGetattrer).Getattr(ctx, out)
	}

	// OTHERWISE get the node's attributes (stat the node)
	path := n.path()
	log.Printf("NO FILEHANDLE PASSED IN GETATTR, STATING %v INSTEAD\n", path)
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
	log.Println("ENTERED SETATTR")

	// If we have a file descriptor, use its setattr
	if f != nil {
		return f.(fs.FileSetattrer).Setattr(ctx, in, out)
	}

	// Manually change the attributes ourselves
	path := n.path()

	// If the mode needs to be changed
	if mode, ok := in.GetMode(); ok {
		// Change the mode to the new mode
		if err := syscall.Chmod(path, mode); err != nil {
			return fs.ToErrno(err)
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
		// Chown these values
		err := syscall.Chown(path, safeUID, safeGID)
		if err != nil {
			return fs.ToErrno(err)
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
		// Call the utimenano syscall, ensuring to convert our time array
		// into a slice, as it expects one
		if err := syscall.UtimesNano(path, times[:]); err != nil {
			return fs.ToErrno(err)
		}
	}

	// If we have a size to update, do so as well
	if size, ok := in.GetSize(); ok {
		if err := syscall.Truncate(path, int64(size)); err != nil {
			return fs.ToErrno(err)
		}
	}

	// Now reflect these changes in the out stream
	stat := syscall.Stat_t{}
	err := syscall.Lstat(path, &stat) // respect symlinks with lstat
	if err != nil {
		return fs.ToErrno(err)
	}
	out.FromStat(&stat)

	return fs.OK
}

// Opens a file for reading, and returns a filehandle
// flags determines how we open the file (read only, read-write)
func (n *OptiFSNode) Open(ctx context.Context, flags uint32) (f fs.FileHandle, fFlags uint32, errno syscall.Errno) {
	flags = flags &^ syscall.O_APPEND
	// Prefers an AND NOT with syscall.O_APPEND, removing it from the flags if it exists
	log.Println("ENTERED OPEN")
	path := n.path()
	fileDescriptor, err := syscall.Open(path, int(flags), 0666) // try to open the file at path
	if err != nil {
		return nil, 0, fs.ToErrno(err)
	}

	// Creates a custom filehandle from the returned file descriptor from Open
	optiFile := file.NewOptiFSFile(fileDescriptor)
	log.Println("Created a new loopback file")
	return optiFile, flags, fs.OK

}

// Get EXTENDED attribute
func (n *OptiFSNode) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	log.Println("ENTERED GETXATTR")
	// Pass it down to the filesystem below
	attributeSize, err := syscall.Getxattr(n.path(), attr, dest)
	return uint32(attributeSize), fs.ToErrno(err)
}

// Set EXTENDED attribute
func (n *OptiFSNode) Setxattr(ctx context.Context, attr string, data []byte, flags uint32) syscall.Errno {
	log.Println("ENTERED SETXATTR")
	// Pass it down to the filesystem below
	err := syscall.Setxattr(n.path(), attr, data, int(flags))
	return fs.ToErrno(err)
}

// Remove EXTENDED attribute
func (n *OptiFSNode) Removexattr(ctx context.Context, attr string) syscall.Errno {
	log.Println("ENTERED REMOVEXATTR")
	err := syscall.Removexattr(n.path(), attr)
	return fs.ToErrno(err)
}

// List EXTENDED attributes
func (n *OptiFSNode) Listxattr(ctx context.Context, dest []byte) (uint32, syscall.Errno) {
	log.Println("ENTERED LISTXATTR")
	// Pass it down to the filesystem below
	allAttributesSize, err := syscall.Listxattr(n.path(), dest)
	return uint32(allAttributesSize), fs.ToErrno(err)
}

// Checks access of a node
func (n *OptiFSNode) Access(ctx context.Context, mask uint32) syscall.Errno {
	log.Println("ENTERED ACCESS")
	return fs.ToErrno(syscall.Access(n.path(), mask))
}

// Make a directory
func (n *OptiFSNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Println("ENTERED MKDIR")
	// Create the directory
	fp := filepath.Join(n.path(), name)
	err := syscall.Mkdir(fp, mode)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	// Now stat the new directory, ensuring it was created
	var directoryStatus syscall.Stat_t
	err = syscall.Stat(fp, &directoryStatus)
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

// create a file that doesn't exist, also fills in the gid/uid of the user into the file attributes
func (n *OptiFSNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (inode *fs.Inode, f fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	fp := filepath.Join(n.path(), name) // create the path for the new file
	flags = flags &^ syscall.O_APPEND   // Prefers an AND NOT with syscall.O_APPEND, removing it from the flags if it exists

	// try to open the file, OR create if theres no file to open
	fdesc, err := syscall.Open(fp, int(flags)|os.O_CREATE, mode)

	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	n.setOwner(ctx, fp) // set who made the file

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

	newFile := file.NewOptiFSFile(fdesc) // make filehandle for file operations

	out.FromStat(&s) // fill out info

	return x, newFile, 0, fs.OK
}

// Unlinks (removes) a file
func (n *OptiFSNode) Unlink(ctx context.Context, name string) syscall.Errno {
	log.Printf("UNLINK performed on %v from node %v\n", name, n.path())
	fp := filepath.Join(n.path(), name)
	err := syscall.Unlink(fp)
	return fs.ToErrno(err)
}

// Unlinks (removes) a directory
func (n *OptiFSNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	log.Printf("RMDIR performed on %v from node %v\n", name, n.path())
	fp := filepath.Join(n.path(), name)
	err := syscall.Rmdir(fp)
	return fs.ToErrno(err)
}

func (n *OptiFSNode) Write(ctx context.Context, f fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	log.Println("ENTERED WRITE")
	if f != nil {
		return f.(fs.FileWriter).Write(ctx, data, off)
	}

	log.Println("WRITE - EBADFD")
	return 0, syscall.EBADFD // bad file descriptor
}

func (n *OptiFSNode) Flush(ctx context.Context, f fs.FileHandle) syscall.Errno {
	log.Println("ENTERED FLUSH")
	if f != nil {
		return f.(fs.FileFlusher).Flush(ctx)
	}
	log.Println("FLUSH - EBADFD")
	return syscall.EBADFD // bad file descriptor
}

// FUSE's version of a close
func (n *OptiFSNode) Release(ctx context.Context, f fs.FileHandle) syscall.Errno {
	log.Println("ENTERED RELEASE")
	if f != nil {
		return f.(fs.FileReleaser).Release(ctx)
	}
	log.Println("RELEASE - EBADFD")
	return syscall.EBADFD // bad file descriptor
}

func (n *OptiFSNode) Fsync(ctx context.Context, f fs.FileHandle, flags uint32) syscall.Errno {
	log.Println("ENTERED FSYNC")
	if f != nil {
		return f.(fs.FileFsyncer).Fsync(ctx, flags)
	}
	log.Println("FSYNC - EBADFD")
	return syscall.EBADFD // bad file descriptor
}

// gets the status' of locks on a node by passing it to the filehandle
func (n *OptiFSNode) Getlk(ctx context.Context, f fs.FileHandle, owner uint64, lk *fuse.FileLock, flags uint32, out *fuse.FileLock) syscall.Errno {
	if f != nil {
		return f.(fs.FileGetlker).Getlk(ctx, owner, lk, flags, out) // send it if filehandle exists
	}
	log.Println("GETLK - EBADFD")
	return syscall.EBADFD // bad file descriptor
}

// gets a lock on a node by passing it to the filehandle, if it can't get the lock it fails
func (n *OptiFSNode) Setlk(ctx context.Context, f fs.FileHandle, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	if f != nil {
		return f.(fs.FileSetlker).Setlk(ctx, owner, lk, flags) // send it if filehandle exists
	}
	log.Println("SETLK - EBADFD")
	return syscall.EBADFD // bad file descriptor
}

// gets a lock on a node by passing it to the filehandle
// if it can't get the lock then it waits for the lock to be obtainable
func (n *OptiFSNode) Setlkw(ctx context.Context, f fs.FileHandle, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	if f != nil {
		return f.(fs.FileSetlkwer).Setlkw(ctx, owner, lk, flags) // send it if filehandle exists
	}
	log.Println("SETLKW - EBADFD")
	return syscall.EBADFD // bad file descriptor
}
