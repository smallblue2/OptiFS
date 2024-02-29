package permissions

import (
	"context"
	"encoding/gob"
	"log"
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
	log.Println("Saving sysadmin info")

	file, err := os.Create(dest + "/OptiFSSysadminSave.gob")

	if err != nil {
		log.Println("Couldn't create save file.")
		return err
	}

	defer file.Close() // don't let the file close

	encode := gob.NewEncoder(file)  // set the file that we created to the encoder
	eErr := encode.Encode(SysAdmin) // encode the hashmap into binary, put it in the file

	if eErr != nil {
		log.Println("Couldn't encode sysadmin info.")
		return eErr
	}

	log.Println("Succesfully saved sysadmin info.")

	return nil
}

// retrieve the sysadmin details when the system boots up
func RetrieveSysadmin(dest string) error {

	log.Println("Retrieving sysadmin info")

	file, err := os.Open(dest + "/OptiFSSysadminSave.gob") // open where the info was encoded

	if err != nil {
		log.Println("Couldn't open save file.")
		return err
	}

	defer file.Close() // don't let file close

	decode := gob.NewDecoder(file)   // set the file that we opened to the decoder
	dErr := decode.Decode(&SysAdmin) // decode the file back into the struct

	if dErr != nil {
		log.Println("Couldn't decode sysadmin info.")
		return dErr
	}

	log.Println("Succesfully retrieved sysadmin info.")

	return nil
}

// print the current sysadmin
func PrintSysadminInfo() {
	log.Printf("Current SysAdmin: %+v\n", SysAdmin)
}

// get the UID and GID of the sysadmin that runs the filesystem
// this is saved (persisent), so we only need to get it once
func SetSysadmin() syscall.Errno {
	log.Println("NO SYSADMIN SET - Setting user as sysadmin.")

	sysadmin, sErr := user.Current() // get the current user
	if sErr != nil {
		log.Printf("Couldn't get sysadmin info!: %v\n", sErr)
		return fs.ToErrno(sErr)
	}

	u, uidConversionErr := strconv.Atoi(sysadmin.Uid) // get the UID
	if uidConversionErr != nil {
		log.Printf("Couldn't get sysadmin UID: %v\n", uidConversionErr)
		return fs.ToErrno(uidConversionErr)
	}

	g, gidConversionErr := strconv.Atoi(sysadmin.Gid) // get the GID
	if gidConversionErr != nil {
		log.Printf("Couldn't get sysadmin GID: %v\n", gidConversionErr)
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
			log.Fatalf("Couldn't get sysadmin UID from context!: %v\n", ctxErr)
		}
		if uid == SysAdmin.UID || gid == SysAdmin.GID {
			return true
		}
	} else {
		// if there is no context (starting up system, etc.)
		user, err := user.Current() // get the current user
		if err != nil {
			log.Fatalf("Couldn't get UID of user: %v\n", err)
		}

		// extract userID
		userUID, conversionErr := strconv.Atoi(user.Uid)
		if conversionErr != nil {
			log.Fatalf("Couldn't get sysadmin UID!: %v\n", conversionErr)
		}

		// extract groupID
		userGID, gidConversionErr := strconv.Atoi(user.Gid) // get the GID
		if gidConversionErr != nil {
			log.Fatalf("Couldn't get sysadmin GID!: %v\n", gidConversionErr)
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
        log.Printf("UID does not exist on system: {%v}\n", uid)
		return fs.ToErrno(syscall.ENOENT)
	}

	// extract userID
	newUid, conversionErr := strconv.Atoi(uid)
	if conversionErr != nil {
		log.Printf("Invalid UID provided: {%v}\n", conversionErr)
		return fs.ToErrno(syscall.ENOENT)
	}
	SysAdmin.UID = uint32(newUid) // set new UID

	return fs.OK

}

// change the group of the sysadmin (if specified)
// it is checked before this function is called that the person calling it is a current sysadmin
func ChangeSysadminGID(gid string) syscall.Errno {
	if !ValidUID(gid) {
		log.Printf("GID does not exist on system: {%v}\n", gid)
		return fs.ToErrno(syscall.ENOENT)
	}

	// extract GID
	newGid, conversionErr := strconv.Atoi(gid)
	if conversionErr != nil {
		log.Printf("Invalid GID provided: {%v}\n", conversionErr)
		return fs.ToErrno(syscall.ENOENT)
	}
	SysAdmin.GID = uint32(newGid) // set new GID

	return fs.OK

}
