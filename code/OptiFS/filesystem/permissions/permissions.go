package permissions

import (
	"context"
	"filesystem/metadata"
	"log"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Checks open syscall permissions
func CheckOpenPermissions(ctx context.Context, nodeMetadata *metadata.MapEntryMetadata, flags uint32) bool {

	isAllowed := true

	// Check the intent of the open flags
	readIntent, writeIntent := checkOpenIntent(flags)

	log.Printf("Write intent: {%v} || Read intent: {%v}\n", writeIntent, readIntent)

	// If the open intends to read, check it has permission
	if readIntent {
		readPerm := CheckPermissions(ctx, nodeMetadata, 0)
		log.Printf("Checked read perm -> {%v}\n", readPerm)
		if !readPerm {
			isAllowed = false
		}
	}
	// If the open intends to write, check it has permission
	if writeIntent {
		writePerm := CheckPermissions(ctx, nodeMetadata, 1)
		log.Printf("Checked write perm -> {%v}\n", writePerm)
		if !writePerm {
			isAllowed = false
		}
	}

	// sysadmin is always allowed
	if IsUserSysadmin() {
		isAllowed = true
	}

	log.Printf("Is allowed -> {%v}\n", isAllowed)
	return isAllowed
}

// Checks the permissions for the user stored in 'ctx' for the 'nodeMetadata' trying the operation 'op'.
//
// READ -> op = 0
// WRITE -> op = 1
// EXEC -> op = 2
func CheckPermissions(ctx context.Context, nodeMetadata *metadata.MapEntryMetadata, op uint8) bool {
	err1, uid, gid := GetUIDGID(ctx)
	if err1 != fs.OK {
		log.Println("Failed to get UIDGID, exiting.")
		return false
	}

	switch op {
	case 0: // Read permission check
		log.Println("Checking read permissions")
		return readCheck(uid, gid, nodeMetadata)
	case 1:
		log.Println("Checking write permissions")
		return writeCheck(uid, gid, nodeMetadata)
	case 2:
		log.Println("Checking exec permissions")
		return execCheck(uid, gid, nodeMetadata)
	default:
		log.Printf("Unknown operation permission check requested (%v)\n", op)
		return false
	}
}

// Checks the mode against the uid and gid for read permissions
func readCheck(uid uint32, gid uint32, nodeMetadata *metadata.MapEntryMetadata) bool {
	return checkMode(uid, gid, nodeMetadata, syscall.S_IRUSR, syscall.S_IRGRP, syscall.S_IROTH)
}

// Checks the mode against the uid and gid for write permissions
func writeCheck(uid uint32, gid uint32, nodeMetadata *metadata.MapEntryMetadata) bool {
	return checkMode(uid, gid, nodeMetadata, syscall.S_IWUSR, syscall.S_IWGRP, syscall.S_IWOTH)
}

// Checks the mode against the uid and gid for exec permissions
func execCheck(uid uint32, gid uint32, nodeMetadata *metadata.MapEntryMetadata) bool {
	return checkMode(uid, gid, nodeMetadata, syscall.S_IXUSR, syscall.S_IXGRP, syscall.S_IXOTH)
}

func checkMode(uid uint32, gid uint32, nodeMetadata *metadata.MapEntryMetadata, ownerFlag uint32, groupFlag uint32, otherFlag uint32) bool {
	log.Println("Extracting mode...")
	mode := nodeMetadata.Mode

	// sysadmin is always allowed
	if IsUserSysadmin() {
		return true
	}

	log.Println("Performing AND operation checks...")
	switch {
	case isOwner(uid, nodeMetadata.Uid):
		log.Println("User is the owner")
		return mode&ownerFlag != 0
	case isGroup(gid, nodeMetadata.Gid):
		log.Println("User is in the group")
		return mode&groupFlag != 0
	default:
		log.Println("User is considered other")
		return mode&otherFlag != 0
	}

}

// Checks if the caller is the owner of a node
func IsOwner(ctx context.Context, nodeMetadata *metadata.MapEntryMetadata) bool {
	err, uid, _ := GetUIDGID(ctx)
	if err != fs.OK {
		return false
	}

	return nodeMetadata.Uid == uid
}

func isOwner(uid uint32, oUid uint32) bool {
	return uid == oUid
}

func isGroup(gid uint32, oGid uint32) bool {
	return gid == oGid
}

// Checks if the requested access is allowed based on the user bits
//
// Can be used to check other or group through bitshifting
func checkPermissionBits(mask, mode uint32) bool {
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

// returns the intent of an open, what permissions will be required
func checkOpenIntent(flags uint32) (readIntent bool, writeIntent bool) {
	// syscall.O_RDONLY is 0!!
	if flags == syscall.O_RDONLY || flags&syscall.O_RDWR != 0 {
		readIntent = true
	}
	if flags&syscall.O_WRONLY != 0 || flags&syscall.O_RDWR != 0 ||
		flags&syscall.O_CREAT != 0 || flags&syscall.O_TRUNC != 0 ||
		flags&syscall.O_APPEND != 0 {
		writeIntent = true
	}
	return
}

// Gets the caller UID and GID from the context provided
func GetUIDGID(ctx context.Context) (syscall.Errno, uint32, uint32) {
	caller, check := fuse.FromContext(ctx)
	if !check {
		log.Println("No caller info available")
		return fs.ToErrno(syscall.ENODATA), 0, 0
	}
	return fs.OK, uint32(caller.Uid), uint32(caller.Gid)
}

// Checks a mask against nodeMetadata mode
func CheckMask(ctx context.Context, mask uint32, nodeMetadata *metadata.MapEntryMetadata) bool {

	// Extract the UID and GID from the context
	err1, currentUID, currentGID := GetUIDGID(ctx)
	if err1 != fs.OK {
		log.Println("Can't get user context")
		return false
	}

	// Determine access writes based on the Mode
	mode := nodeMetadata.Mode
	var allowed bool

	switch {
	case isOwner(currentUID, nodeMetadata.Uid):
		// user is the owner
		log.Println("User is the owner")
		// Don't shift the mode at all, as the bits are in the correct place already
		allowed = checkPermissionBits(mask, mode)
		log.Printf("Owner requested %v, allowed: %v\n", mask, allowed)
	case isGroup(currentGID, nodeMetadata.Gid):
		// User is in the group
		log.Println("User is in the group")
		// shift mode 3 bits to the left to line up group permission bits to be under where user bits usually are
		allowed = checkPermissionBits(mask, mode<<3)
		log.Printf("Group member requested %v, allowed: %v\n", mask, allowed)
	default:
		// Check for others permissions
		log.Println("User is under others")
		// shift mode 6 bits to the left to line up other permission bits to be under where user bits usually are
		allowed = checkPermissionBits(mask, mode<<6)
		log.Printf("Other member requested %v, allowed: %v\n", mask, allowed)
	}

	// sysadmin is always allowed
	if IsUserSysadmin() {
		allowed = true
	}

	if !allowed {
		log.Println("NOT ALLOWED")
		return false
	}

	log.Println("ALLOWED")
	return true
}
