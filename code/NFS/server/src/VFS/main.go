package main

import (
	"context"
	"flag"
	"log"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Represents the root node
type myRoot struct {
	fs.Inode
}

// Ensures the root node implements OnAdd
var _ = (fs.NodeOnAdder)((*myRoot)(nil))

// Constructs a simple persistent filesystem on mount
func (r *myRoot) OnAdd(ctx context.Context) {
	log.Println("Populating filesystem...")

	var rootInode *fs.Inode = &r.Inode

	node1 := myNodeFactory(ctx, rootInode, 100, "Test file one, Hello if you're reading this!\n")
	node2 := myNodeFactory(ctx, rootInode, 101, "Second test file, how's it going?\n")
	node3 := myNodeFactory(ctx, rootInode, 102, "Hoping NFS will work with this naturally!\n")

	rootInode.AddChild("file_one", node1, true)
	rootInode.AddChild("file_two", node2, true)
	rootInode.AddChild("file_three", node3, true)
}

// Custom node structure
type myNode struct {
	fs.Inode           // Composing the regular Inode
	Attr     fuse.Attr // Holds the attributes of the file
	Data     []byte    // Holds the data of the file
}

// A factory for creating persistent myNode's
// Parameters are context, parent's Inode, the new node's inode and content
func myNodeFactory(ctx context.Context, root *fs.Inode, inode uint64, content string) *fs.Inode {
	var size uint64 = uint64(len([]byte(content)))
	var newNode *fs.Inode = root.NewPersistentInode(ctx, &myNode{Attr: fuse.Attr{Ino: inode, Size: size}, Data: []byte(content)}, fs.StableAttr{Mode: syscall.S_IFREG, Ino: inode})
	return newNode
}

// Interfaces for our custom file
var _ = (fs.NodeOpener)((*myNode)(nil))    // Open for a Regular File
var _ = (fs.NodeGetattrer)((*myNode)(nil)) // Generic get attribute
var _ = (fs.NodeReader)((*myNode)(nil))    // Read for a Regular File

// Read-only Open implementation
func (n *myNode) Open(ctx context.Context, openFlags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	// Check if it's being opened for writing
	if openFlags&(syscall.O_RDWR|syscall.O_WRONLY) != 0 { // bit-wise AND to check for write flags
		return nil, 0, syscall.EROFS // No file-handle, no flags, Read-Only File System Error
	}

	return nil, fuse.FOPEN_KEEP_CACHE, fs.OK // Cache the file content (as it's static)
}

// Attribute getter for our custom inode structure
func (n *myNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Attr = n.Attr
	return fs.OK
}

// Read implementation for our custom inode structure
func (n *myNode) Read(ctx context.Context, f fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	var end int = len(dest) + int(off) // Ensure that our destination buffer isn't asking for too much
	if end > len(n.Data) {
		end = len(n.Data)
	}
	return fuse.ReadResultData(n.Data[off:end]), fs.OK
}

func main() {
	// The directory to mount to
	var mountDir string
	// Extract the mount directory from the cmdline
	flag.StringVar(&mountDir, "m", "", "The directory to mount the virtual file system to")
    flag.Parse()
    // If there is no mountpoint specified, exit
	if mountDir == "" {
		log.Fatal("Error: No mountpoint specified.")
	}

	// Create the root node
	var root *myRoot = &myRoot{}

	// Mount the root
	server, err := fs.Mount(mountDir, root, &fs.Options{
		MountOptions: fuse.MountOptions{Debug: true}})
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Mounted on %s", mountDir)
	log.Printf("Unmount by calling 'fusermount -u %s'", mountDir)

	server.Wait()
}
