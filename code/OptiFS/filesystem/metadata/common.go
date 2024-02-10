// This file contains internal code to the metadata module

package metadata

import (
	"log"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
)

// Function updates all MapEntryMetadata attributes from the given unstable attributes
func updateAllFromStat(metadata *MapEntryMetadata, unstableAttr *syscall.Stat_t, stableAttr *fs.StableAttr) {

    log.Printf("New Mode: 0x%X\n", (*stableAttr).Mode)
    // Take these from our stable attributes
    (*metadata).Ino = (*stableAttr).Ino
    (*metadata).Gen = (*stableAttr).Gen

    // Take these from the underlying node's stat
    (*metadata).Mode = (*unstableAttr).Mode
	(*metadata).Atim = (*unstableAttr).Atim
	(*metadata).Mtim = (*unstableAttr).Mtim
	(*metadata).Ctim = (*unstableAttr).Ctim
	(*metadata).Uid = (*unstableAttr).Uid
	(*metadata).Gid = (*unstableAttr).Gid
	(*metadata).Dev = (*unstableAttr).Dev
	(*metadata).Rdev = (*unstableAttr).Rdev
	(*metadata).Nlink = (*unstableAttr).Nlink
	(*metadata).Size = (*unstableAttr).Size
	(*metadata).Blksize = (*unstableAttr).Blksize
	(*metadata).Blocks = (*unstableAttr).Blocks
	(*metadata).X__pad0 = (*unstableAttr).X__pad0
	(*metadata).X__unused = (*unstableAttr).X__unused
}

