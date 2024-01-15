package main

import (
	"context"
	"path/filepath"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Loopback filesystems are simply filesystems that delegate their operations to an underlying POSIX file system.

// Create a root for our loopback filesystem.
type LoopbackRoot struct {
    // The poth to the root of the underlying file system.
    Path string

    // The device on which the Path resides. This must be set if
    // the underlying filesystem crosses file systems.
    Dev uint64

    // NewNode returns a new InodeEmbedder to be used to responde to
    // a LOOKUP/CREATE/MKDIR/MKNOD opcode. If not set, use a LoopbackNode.
    NewNode func(rootData *LoopbackRoot, parent *fs.Inode, name string, st *syscall.Stat_t) fs.InodeEmbedder
}

// Currying, calls the root's custom NewNode if it exists, otherwise defaults
// to using a LoopbackNode
func (r *LoopbackRoot) newNode(parent *fs.Inode, name string, st *syscall.Stat_t) fs.InodeEmbedder {
    if r.NewNode != nil {
        return r.NewNode(r, parent, name, st)
    }
    return &LoopbackNode{
        RootData: r,
    }
}

// Create stable attributes from a file stat
func (r *LoopbackRoot) idFromStat(st *syscall.Stat_t) fs.StableAttr {
    // Create the inode number from the underlying inode, and mixing in
    // the device number.
    swapped := (uint64(st.Dev) << 32) | (uint64(st.Dev) >> 32)
    swappedRootDev := (r.Dev << 32) | (r.Dev >> 32)
    return fs.StableAttr{
        Mode: uint32(st.Mode),
        Gen: 1,
        Ino: (swapped ^ swappedRootDev) ^ st.Ino,
    }
}

// LoopbackNode is a filesystem node in a loopback file system. It is
// public so it can be used as a basis for other loopback based
// filesystems.
type LoopbackNode struct {
    fs.Inode

    // RootData points back to the root of the loopback filesystem.
    RootData *LoopbackRoot
}

var _ = (fs.NodeStatfser)((*LoopbackNode)(nil))
var _ = (fs.NodeLookuper)((*LoopbackNode)(nil))

func (n *LoopbackNode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
    s := syscall.Statfs_t{}
    err := syscall.Statfs(n.path(), &s)
    if err != nil {
        return fs.ToErrno(err)
    }
    out.FromStatfsT(&s)
    return fs.OK
}

// path returns the full path to the file in the underlying file system.
func (n *LoopbackNode) path() string {
    path := n.Path(n.Root())
    return filepath.Join(n.RootData.Path, path)
}

// Locates a child in a directory
//
// If the child exists, returns attribute data about the child in `out`
// Also returns the Inode of the child
//
// If child doesn't exist, return ENOENT (Error, no entry)
func (n LoopbackNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
    // Path of `name` file relative to node 'n'
    p := filepath.Join(n.path(), name)

    st := syscall.Stat_t{}
    err := syscall.Lstat(p, &st)
    if err != nil {
        return nil, fs.ToErrno(err)
    }

    out.Attr.FromStat(&st)
    node := n.RootData.newNode(n.EmbeddedInode(), name, &st)
    ch := n.NewInode(ctx, node, n.RootData.idFromStat(&st)
    return ch, 
}


func main() {

}
