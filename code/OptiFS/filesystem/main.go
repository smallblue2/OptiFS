package main

import (
	"context"
	"log"
	"path/filepath"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fs"
)

// Root for OptiFS
type OptiFSRoot struct {
    // The path to the root of the underlying file system (OptiFS is
    // a loopback filesystem)
    Path string
}

// General Node for OptiFS
type OptiFSNode struct {
    fs.Inode

    // The root node of the filesystem
    RootNode *OptiFSRoot
}

// Interfaces/contracts to abide by
var _ = (fs.NodeStatfser)((*OptiFSNode)(nil)) // StatFS

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

func main() {
    log.Println("Starting OptiFS")
}
