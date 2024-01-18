package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
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
var _ = (fs.NodeGetattrer)((*OptiFSNode)(nil))     // get attributes of a file/dir
var _ = (fs.NodeOpener)((*OptiFSNode)(nil))        // open a file
var _ = (fs.NodeGetxattrer)((*OptiFSNode)(nil))    // Get extended attributes of a node
var _ = (fs.NodeSetxattrer)((*OptiFSNode)(nil))    // Set extended attributes of a node
var _ = (fs.NodeRemovexattrer)((*OptiFSNode)(nil)) // Remove extended attributes of a node
var _ = (fs.NodeListxattrer)((*OptiFSNode)(nil))   // List extended attributes of a node
var _ = (fs.NodeMkdirer)((*OptiFSNode)(nil))       // Creates a directory
var _ = (fs.NodeUnlinker)((*OptiFSNode)(nil))      // Unlinks (deletes) a file
var _ = (fs.NodeRmdirer)((*OptiFSNode)(nil))       // Unlinks (deletes) a directory
var _ = (fs.NodeWriter)((*OptiFSNode)(nil))        // writes to a file
var _ = (fs.NodeFlusher)((*OptiFSNode)(nil))       // Handles the closing of a Node
var _ = (fs.NodeReleaser)((*OptiFSNode)(nil))      // Handles releasing a Node
var _ = (fs.NodeFsyncer)((*OptiFSNode)(nil))       // Ensures that data is actually written to

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
	return fs.NewLoopbackDirStream(n.path()) // TODO: Implement ourselves maybe?
}

// get the attributes of a file/dir, either with a filehandle (if passed) or through inodes
func (n *OptiFSNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	// if we have a file handle, use it to get the attributes
	if fh != nil {
		return fh.(fs.FileGetattrer).Getattr(ctx, out)
	}

	// OTHERWISE get the node's attributes (stat the node)
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

// Opens a file for reading, and returns a filehandle
// flags determines how we open the file (read only, read-write)
func (n *OptiFSNode) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fFlags uint32, errno syscall.Errno) {
	log.Println("ENTERED OPEN")
	flags = flags &^ syscall.O_APPEND // Prefers an AND NOT with syscall.O_APPEND, removing it from the flags if it exists
	path := n.path()
	fileDescriptor, err := syscall.Open(path, int(flags), 0) // try to open the file at path
	if err != nil {
		return nil, 0, fs.ToErrno(err)
	}

	// Creates a custom filehandle from the returned file descriptor from Open
	lbFile := fs.NewLoopbackFile(fileDescriptor) // TODO: Implement with our own filehandle
	return lbFile, 0, 0

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

// takes in data to write to a file, beginning from a specified offset
func (n *OptiFSNode) Write(ctx context.Context, fh fs.FileHandle, data []byte, offset int64) (uint32, syscall.Errno) {
	log.Println("Attempting to write to file")
	// if we have a filehandle, forward the write to it
	if fh != nil {
		return fh.(fs.FileWriter).Write(ctx, data, offset) /// check that its a file first
	}

	return 0, syscall.EBADFD

	// // temp code, dont know how to get the file descriptor from open function implemented
	// file, fErr := syscall.Open(n.path(), syscall.O_WRONLY, 0666) // open the file for writing, with the default permissions for a file
	// if fErr != nil {
	// 	return 0, fs.ToErrno(fErr) // no bytes written
	// }

	// // go to "offset" many bites from the start (0) of file
	// _, sErr := syscall.Seek(file, offset, 0)
	// if sErr != nil {
	// 	return 0, fs.ToErrno(sErr) // no bytes written
	// }

	// written, wErr := syscall.Write(file, data) // write the data to the file
	// if wErr != nil {
	// 	return 0, fs.ToErrno(wErr) // no bytes written
	// }

	// return uint32(written), fs.OK

}

// Closes a node -> FORWARDS TO FILEHANDLE'S FLUSH
func (n *OptiFSNode) Flush(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	log.Println("ENTERED FLUSH")
	if fh != nil {
		return fh.(fs.FileFlusher).Flush(ctx)
	}

	return syscall.EBADFD
}

// Releases a node -> FORWARDS TO FILEHANDLE'S RELEASE
func (n *OptiFSNode) Release(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	log.Println("ENTERED RELEASE")
	if fh != nil {
		return fh.(fs.FileReleaser).Release(ctx)
	}

	return syscall.EBADFD
}

func (n *OptiFSNode) Fsync(ctx context.Context, fh fs.FileHandle, flags uint32) syscall.Errno {
	log.Println("ENTERED FSYNC")
	if fh != nil {
		return fs.ToErrno(fh.(fs.FileFsyncer).Fsync(ctx, flags))
	}

	return syscall.EBADFD
}

func main() {
	log.Println("Starting OptiFS")
	log.SetFlags(log.Lmicroseconds)
	debug := flag.Bool("debug", false, "enter debug mode")

	flag.Parse() // parse arguments
	if flag.NArg() < 2 {
		fmt.Printf("usage: %s <mountpoint> <underlying filesystem>\n", path.Base(os.Args[0])) // show correct usage
		fmt.Printf("\noptions:\n")
		flag.PrintDefaults() // show what optional flags can be used
		os.Exit(2)           // exit w/ error code
	}

	under := flag.Arg(1)
	data := &OptiFSRoot{
		Path: under,
	}

	// set the options for the filesystem:
	options := &fs.Options{}
	options.Debug = *debug                                                               // set the debug value the user chooses (T/F)
	options.MountOptions.Options = append(options.MountOptions.Options, "fsname="+under) // set the filesystem name
	options.NullPermissions = true                                                       // doesn't check the permissions for calls (good for setting up custom permissions [namespaces??])

	root := &OptiFSNode{
		RootNode: data,
	}

	// mount the filesystem
	server, err := fs.Mount(flag.Arg(0), root, options)
	if err != nil {
		log.Fatalf("Mount Failed!!: %v\n", err)
	}

	log.Println("=========================================================")
	log.Printf("Mounted %v with underlying root at %v\n", flag.Arg(0), data.Path)
	log.Printf("DEBUG: %v", options.Debug)
	log.Println("=========================================================")
	server.Wait()

}
