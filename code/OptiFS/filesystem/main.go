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
var _ = (fs.NodeStatfser)((*OptiFSNode)(nil))  // StatFS
var _ = (fs.InodeEmbedder)((*OptiFSNode)(nil)) // Inode
var _ = (fs.NodeLookuper)((*OptiFSNode)(nil))  // lookup
var _ = (fs.NodeOpendirer)((*OptiFSNode)(nil)) // opening directories
var _ = (fs.NodeReaddirer)((*OptiFSNode)(nil)) // read directory
var _ = (fs.NodeGetattrer)((*OptiFSNode)(nil)) // get attributes of a file/dir
var _ = (fs.NodeOpener)((*OptiFSNode)(nil))    // open a file

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
	// if the new node was already defined
	if n.NewNode != nil {
		return n.NewNode(n, parent, name, s)
	}

	// if there's no custom node, do the default creation
	return &OptiFSNode{
		RootNode: n,
	}
}

// since we're using NFS, theres a chance not all Inode numbers will be unique, so we
// calculate one using bit swapping
func (n *OptiFSRoot) idFromStat(s *syscall.Stat_t) fs.StableAttr {
	// swap the higher and lower bits to generate unique no
	swapped := (uint64(s.Dev) << 32) | (uint64(s.Dev) >> 32)
	swappedRoot := (n.Dev << 32) | (n.Dev << 32)

	return fs.StableAttr{
		Mode: uint32(s.Mode),                  // perms
		Gen:  1,                               // generation number (determines lifetime of inode)
		Ino:  (swapped ^ swappedRoot) ^ s.Ino, // the inode number
	}
}

// lookup finds a file/directory based on its name
func (n *OptiFSNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Println("lookup performed")
	fp := filepath.Join(n.path(), name) // getting the full path to the file (join name to path)
	s := syscall.Stat_t{}               // status of a file
	err := syscall.Lstat(fp, &s)        // gets the file attributes (also returns attrs of symbolic link)

	if err != nil {
		return nil, fs.ToErrno(err)
	}

	out.Attr.FromStat(&s)                                 // fill attrs to struct
	nd := n.RootNode.newNode(n.EmbeddedInode(), name, &s) // create new node to rep file
	x := n.NewInode(ctx, nd, n.RootNode.idFromStat(&s))

	return x, fs.OK
}

// opens a directory and then closes it
func (n *OptiFSNode) Opendir(ctx context.Context) syscall.Errno {
	dir, err := syscall.Open(n.path(), syscall.O_DIRECTORY, 0755)
	if err != nil {
		return fs.ToErrno(err)
	}
	syscall.Close(dir) // close when done
	return fs.OK
}

// opens a stream of dir entries,
func (n *OptiFSNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	return fs.NewLoopbackDirStream(n.path())
}

// get the attributes of a file/dir, either with a filehandle (if passed) or through inodes
func (n *OptiFSNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	// if we have a file handle, use it to get the attributes
	if fh != nil {
		return fh.(fs.FileGetattrer).Getattr(ctx, out)
	}

	path := n.path()
	var err error
	s := syscall.Stat_t{}
	if &n.Inode == n.Root() {
		err = syscall.Stat(path, &s) // if we are looking for the root of FS
	} else {
		err = syscall.Lstat(path, &s) // if it's just a normal file/dir
	}

	if err != nil {
		return fs.ToErrno(err)
	}

	out.FromStat(&s) // no error getting attrs, fill them in out
	return fs.OK
}

// opens a file for reading, and returns a filehandle
// flags determines how we open the file (read only, read-write)
func (n *OptiFSNode) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fFlags uint32, errno syscall.Errno) {
	flags = flags &^ syscall.O_APPEND // clears all bits in flags (1 to 0), for appending to end of file (&^ = AND NOT)
	path := n.path()
	file, err := syscall.Open(path, int(flags), 0) // try to open the file at path
	if err != nil {
		return nil, 0, fs.ToErrno(err)
	}

	lbFile := fs.NewLoopbackFile(file) // makes a filehandle
	return lbFile, 0, 0

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

	log.Printf("Mounted %v with underlying root at %v\n", flag.Arg(0), data.Path)
	server.Wait()

}
