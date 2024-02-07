package permissions

import (
	"context"
	"errors"
	"filesystem/metadata"
	"log"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fuse"
)

// Gets the caller UID and GID from the context provided
func getUIDGID(ctx context.Context) (error, uint32, uint32) {
    caller, check := fuse.FromContext(ctx)
    if !check {
        log.Println("No caller info available")
        return errors.New("No caller info available"), 0, 0
    }
    return nil, uint32(caller.Uid), uint32(caller.Gid)
}

// Checks a user's access to a node
func CheckAccess(ctx context.Context, mask uint32, nodeMetadata *metadata.MapEntryMetadata) bool {
    // Extract the UID and GID from the context
    err1, currentUID, currentGID := getUIDGID(ctx)
    if err1 != nil {
        log.Println("Can't get user context")
        return false
    }

    // Determine access writes based on the Mode
    mode := nodeMetadata.Mode
    var allowed bool

    switch {
    case currentUID == nodeMetadata.Uid:
        // user is the owner
        log.Println("User is the owner")
        // Don't shift the mode at all, as the bits are in the correct place already
        allowed = CheckPermissionBits(mask, mode)
        log.Printf("Owner requested %v, allowed: %v\n", mask, allowed)
    case currentGID == nodeMetadata.Gid:
        // User is in the group
        log.Println("User is in the group")
        // shift mode 3 bits to the left to line up group permission bits to be under where user bits usually are
        allowed = CheckPermissionBits(mask, mode<<3)
        log.Printf("Group member requested %v, allowed: %v\n", mask, allowed)
    default:
        // Check for others permissions
        log.Println("User is under others")
        // shift mode 6 bits to the left to line up other permission bits to be under where user bits usually are
        allowed = CheckPermissionBits(mask, mode<<6)
        log.Printf("Other member requested %v, allowed: %v\n", mask, allowed)
    }

    if !allowed {
        log.Println("NOT ALLOWED")
        return false
    }

    log.Println("ALLOWED")
    return true
}

// Checks open flags against MapEntryMetadata mode, uid & gid
func CheckOpenPermissions(ctx context.Context, nodeMetadata *metadata.MapEntryMetadata, flags uint32) bool {

    // Extract the UID and GID from the context
    err1, currentUID, currentGID := getUIDGID(ctx)
    if err1 != nil {
        return false
    }

    // Determine access writes based on the Mode
    mode := nodeMetadata.Mode
    allowed := true

    // Check to see if we're reading and/or writing
    readFlags := syscall.O_RDONLY | syscall.O_RDWR | syscall.O_SYNC
    writeFlags := syscall.O_WRONLY | syscall.O_RDWR | syscall.O_APPEND | syscall.O_DSYNC | syscall.O_FSYNC | syscall.O_DIRECT

    reading := flags&uint32(readFlags)
    writing := flags&uint32(writeFlags)

    isOwner := currentUID == nodeMetadata.Uid
    isGroup := currentGID == nodeMetadata.Gid

    // Check read permissions if necessary
    if reading != 0 {
        log.Println("Open requires reading permission")
        // Check the read permissions
        if isOwner { // IF we're the owner
            log.Println("User is the owner")
            if mode&syscall.S_IRUSR == 0 { // IF we're not allowed to read
                log.Println("Reading is not allowed")
                allowed = false
            }
            log.Println("Reading is allowed")
        } else if isGroup { // IF we're in the group
            log.Println("User is in the group")
            if mode&syscall.S_IRGRP == 0 { // IF we're not allowed to read
                log.Println("Reading is not allowed")
                allowed = false
            }
            log.Println("Reading is allowed")
        } else { // OTHERWISE we're other
            log.Println("User is other")
            if mode&syscall.S_IROTH == 0 { // IF we're not allowed to read
                log.Println("Reading is not allowed")
                allowed = false
            }
            log.Println("Reading is allowed")
        }
    }

    // Check writing permissions if necessary
    if writing != 0 {
        log.Println("Open requires writing permission")
        // Check the write permissions
        if isOwner { // IF we're the owner
            log.Println("User is the owner")
            if mode&syscall.S_IWUSR == 0 { // IF we're not allowed to write
                log.Println("Writing not allowed")
                allowed = false
            }
            log.Println("Writing allowed")
        } else if isGroup { // IF we're in the group
            log.Println("User is in the group")
            if mode&syscall.S_IWGRP == 0 { // IF we're not allowed to write
                log.Println("Writing not allowed")
                allowed = false
            }
            log.Println("Writing allowed")
        } else { // OTHERWISE we're other
            log.Println("User is other")
            if mode&syscall.S_IWOTH == 0 { // IF we're not allowed to write
                log.Println("Writing not allowed")
                allowed = false
            }
            log.Println("Writing allowed")
        }
    }

    log.Printf("Access: %v\n", allowed)
    return allowed
}

// Function checks open dir permissions
func CheckOpenDirPermissions(ctx context.Context, dirMetadata *metadata.MapEntryMetadata) bool {
    err, currentUID, currentGID := getUIDGID(ctx)
    if err != nil {
        log.Println(err)
        return false
    }

    mode := dirMetadata.Mode
    isOwner := currentUID == dirMetadata.Uid
    isGroup := currentGID == dirMetadata.Gid

    // Check execute permissions
    allowed := false
    if isOwner { // If the user is the owner of the directory
        allowed = mode&syscall.S_IXUSR != 0
    } else if isGroup {
        allowed = mode&syscall.S_IXGRP != 0
    } else {
        allowed = mode&syscall.S_IXOTH != 0
    }

    return allowed
}

// Function checks if the user has read permissions on the MapEntryMetadata
func CheckReadPermissions(ctx context.Context, metadata *metadata.MapEntryMetadata) bool {
    // Extract the UID and GID from the context
    err1, currentUID, currentGID := getUIDGID(ctx)
    if err1 != nil {
        return false
    }

    // Check read permissions
    mode := metadata.Mode
    switch {
    case currentUID == metadata.Uid:
        log.Println("User is the owner")
        return mode&syscall.S_IRUSR != 0
    case currentGID == metadata.Gid:
        log.Println("User is in the group")
        return mode&syscall.S_IRGRP != 0
    default:
        log.Println("User is considered other")
        return mode&syscall.S_IROTH != 0
    }
}

// Checks if the requested access is allowed based on the user bits
//
// Can be used to check other or group through bitshifting
func CheckPermissionBits(mask, mode uint32) bool {
	// Read permission check
	// If the mask AND'd with 4 (requesting read access), AND the mode AND'd with S_IRUSR == 0 (file doesn't allow read access)
	if mask&4 > 0 && mode&syscall.S_IRUSR == 0 {
        return false
	}
    // Write permission check
    // If the mask AND'd with 2 (requesting write access), AND the mode AND'd with S_IWUSR == 0 (file doesn't allow write access)
    if mask&2 > 0 && mode&syscall.S_IWUSR == 0 {
        return false
    }
    // Execute permission check
    // If the mask AND'd with 1 (requesting exec access), AND the mode AND'd with S_IXUSR == 0 (file doesn't allow exec access)
    if mask&1 > 0 && mode&syscall.S_IXUSR == 0 {
        return false
    }
    return true
}
