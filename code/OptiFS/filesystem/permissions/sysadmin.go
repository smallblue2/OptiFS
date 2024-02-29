package permissions

import (
	"context"
	"encoding/gob"
	"os"
	"os/user"
	"strconv"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
)

type Sysadmin struct {
	UID uint32
	GID uint32
	Set bool // flag to see if we have set the sysadmin already
}

var SysAdmin Sysadmin

// save the sysadmin details when the system shuts down
func SaveSysadmin(dest string) error {
	// create the file if it doesn't exist, truncate it if it does
	// we assume nobody will be calling this file, as it is a very unique name

	file, err := os.Create(dest + "/OptiFSSysadminSave.gob")

	if err != nil {
		return err
	}

	defer file.Close() // don't let the file close

	encode := gob.NewEncoder(file)  // set the file that we created to the encoder
	eErr := encode.Encode(SysAdmin) // encode the hashmap into binary, put it in the file

	if eErr != nil {
		return eErr
	}


	return nil
}

// retrieve the sysadmin details when the system boots up
func RetrieveSysadmin(dest string) error {


	file, err := os.Open(dest + "/OptiFSSysadminSave.gob") // open where the info was encoded

	if err != nil {
		return err
	}

	defer file.Close() // don't let file close

	decode := gob.NewDecoder(file)   // set the file that we opened to the decoder
	dErr := decode.Decode(&SysAdmin) // decode the file back into the struct

	if dErr != nil {
		return dErr
	}


	return nil
}

// get the UID and GID of the sysadmin that runs the filesystem
// this is saved (persisent), so we only need to get it once
func SetSysadmin() syscall.Errno {

	sysadmin, sErr := user.Current() // get the current user
	if sErr != nil {
		return fs.ToErrno(sErr)
	}

	u, uidConversionErr := strconv.Atoi(sysadmin.Uid) // get the UID
	if uidConversionErr != nil {
		return fs.ToErrno(uidConversionErr)
	}

	g, gidConversionErr := strconv.Atoi(sysadmin.Gid) // get the GID
	if gidConversionErr != nil {
		return fs.ToErrno(gidConversionErr)
	}

	// fill in sysadmin details
	SysAdmin.UID = uint32(u)
	SysAdmin.GID = uint32(g)
	SysAdmin.Set = true

	return fs.OK
}

// checks if the user is the sysadmin of the system, or is in the same sysadmin group
func IsUserSysadmin(ctx *context.Context) bool {

	// if we have a context to get it from
	if ctx != nil {
		ctxErr, uid, gid := GetUIDGID(*ctx)
		if ctxErr != fs.OK {
		}
		if uid == SysAdmin.UID || gid == SysAdmin.GID {
			return true
		}
	} else {
		// if there is no context (starting up system, etc.)
		user, err := user.Current() // get the current user
		if err != nil {
		}

		// extract userID
		userUID, conversionErr := strconv.Atoi(user.Uid)
		if conversionErr != nil {
            return false
		}

		// extract groupID
		userGID, gidConversionErr := strconv.Atoi(user.Gid) // get the GID
		if gidConversionErr != nil {
            return false
		}

		// if they have the same UID or are in the same group (sysadmin group)
		if uint32(userUID) == SysAdmin.UID || uint32(userGID) == SysAdmin.GID {
			return true
		}

		return false
	}

	return false
}

// checker function, if the UID is valid returns true, else false
func ValidUID(uid string) bool {
	_, err := user.LookupId(uid)
	return err == nil
}

// checker function, if the GID is valid returns true, else false
func ValidGID(gid string) bool {
	_, err := user.LookupGroupId(gid)
	return err == nil
}

// change the sysadmin UID (if specified)
// it is checked before this function is called that the person calling it is a current sysadmin
func ChangeSysadminUID(uid string) syscall.Errno {
	if !ValidUID(uid) {
		return fs.ToErrno(syscall.ENOENT)
	}

	// extract userID
	newUid, conversionErr := strconv.Atoi(uid)
	if conversionErr != nil {
		return fs.ToErrno(syscall.ENOENT)
	}
	SysAdmin.UID = uint32(newUid) // set new UID

	return fs.OK

}

// change the group of the sysadmin (if specified)
// it is checked before this function is called that the person calling it is a current sysadmin
func ChangeSysadminGID(gid string) syscall.Errno {
	if !ValidUID(gid) {
		return fs.ToErrno(syscall.ENOENT)
	}

	// extract GID
	newGid, conversionErr := strconv.Atoi(gid)
	if conversionErr != nil {
		return fs.ToErrno(syscall.ENOENT)
	}
	SysAdmin.GID = uint32(newGid) // set new GID

	return fs.OK

}
