// This file contains the hashmap definitions for the metadata module

package metadata

// Contains all metadata for regular files in our custom metadata system
//
// The key is the content of a regular file hashed using BLAKE3
//
// The value is a MapEntry object, which contains high-level information about a 
// content, and further contains a list of MapEntryMetadata struct, containing unique
// metadata about each instance of that content.
var regularFileMetadataHash = make(map[[64]byte]*MapEntry)

// Contains all metadata for directories in our custom metadata system
//
// The key is the inode of a directory node
//
// The value is the custom metadata information for a directory
var dirMetadataHash = make(map[uint64]*MapEntryMetadata)

// Contains information in order to maintain persistence between OptiFS instances
//
// The key is the path of a node relative to the root
//
// The value is a NodeInfo struct, which contains content hashes and reference numbers
var nodePersistenceHash = make(map[string]*NodeInfo)
