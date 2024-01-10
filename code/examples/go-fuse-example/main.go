package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// files contains the files we will expose as a file system
var files = map[string]string{
	"file":              "content",
	"subdir/other-file": "other-content",
}

// inMemoryFS is the root of the tree
type inMemoryFS struct {
	fs.Inode
}

// Type assertion, asserting that the inMemoryFS pointer satisfies the NodeOnAdder interface
var _ = (fs.NodeOnAdder)((*inMemoryFS)(nil))

// OnAdd is called on mounting the file system
// We use it to populate the file system tree.
func (root *inMemoryFS) OnAdd(ctx context.Context) {
	for name, content := range files {
		dir, base := filepath.Split(name)

		// p is a pointer to the root's Inode
		p := &root.Inode

		// Add directories leading up to the file
		for _, component := range strings.Split(dir, "/") {
			if len(component) == 0 {
				continue
			}
			// Check if the child already exists
			ch := p.GetChild(component)
			if ch == nil {
				// Create a directory
				ch = p.NewPersistentInode(ctx, &fs.Inode{}, fs.StableAttr{Mode: syscall.S_IFDIR})
				// Add it
				succ := p.AddChild(component, ch, true)
				if !succ {
					log.Fatalf("Error, failed to addchild %v to %v", component, p)
				}
			}

			p = ch
		}

		// Make a file out of the content bytes.
		// This type provides the open/read/flush methods.
		embedder := &fs.MemRegularFile{
			Data: []byte(content),
		}

		// Create the file.
		// The Inode must be persistent, because its life-time is not under control of the kernal.
		child := p.NewPersistentInode(ctx, embedder, fs.StableAttr{})

		// Add the file.
		p.AddChild(base, child, true)
	}
}


// How to build a file system in memory.
// The read/write logic for the file is provided by the MemRegularFile type.
func main() {
	// Where we'll mount the FS
	mntDir, _ := os.MkdirTemp("", "mount")
    defer os.Remove(mntDir)

	root := &inMemoryFS{}
	server, err := fs.Mount(mntDir, root, &fs.Options{
		MountOptions: fuse.MountOptions{Debug: true},
	})
	if err != nil {
		log.Panic(err)
	}
	defer server.Unmount()

	log.Printf("Mounted on %s", mntDir)
	log.Printf("Unmount by calling 'fusermount -u %s'", mntDir)

	// Wait until unmount before exiting
	server.Wait()
}
