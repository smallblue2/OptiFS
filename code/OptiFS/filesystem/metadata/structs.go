// This file contains the definitions of the structs used in the Metadata Module

package metadata

import (
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
)

// MapEntry is the entry for hashed content in our regularFileMetadataHash
//
// Contains information about how many files reference the same content,
// what the underlying inode is, etc...
//
// All metadata for each instance of the content is stored in EntryList
type MapEntry struct {
	ReferenceCount  uint32 // How many references there are to the same file content
	EntryList       map[uint64]*MapEntryMetadata
	UnderlyingInode uint32
	IndexCounter    uint64
}

// MapEntryMetadata is a struct that represents a node's custom metadata
//
// Used to represent both the metadata of regular files and directories
type MapEntryMetadata struct {
	Dev       uint64
	Ino       uint64
	Gen       uint64
	Nlink     uint64
	Mode      uint32
	Uid       uint32
	Gid       uint32
	X__pad0   int32
	Rdev      uint64
	Size      int64
	Blksize   int64
	Blocks    int64
	Atim      syscall.Timespec
	Mtim      syscall.Timespec
	Ctim      syscall.Timespec
	X__unused [3]int64
}

// NodeInfo is used to store data required to keep our nodes persistent between OptiFS instances
type NodeInfo struct {
    stableAttr fs.StableAttr
    mode uint32
    isDir bool
	contentHash [64]byte
	refNum      uint64
}
