package filesystem

import (
	"filesystem/metadata"
	"filesystem/permissions"
	"filesystem/vfs"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
)

func main() {
	log.Println("Starting OptiFS")
	log.SetFlags(log.Lmicroseconds)
	debug := flag.Bool("debug", false, "enter debug mode")
	removePersistence := flag.Bool("rm-persistence", false, "remove persistence saving (saving of virtual node metadata)")
	saveLocation := flag.String("save", "", "choose the location of saved hashmaps and sysadmin info")
	disableIntegrityCheck := flag.Bool("disable-icheck", false, "disables the integrity check of the persistent data of the filesystem")
	changeSysadminUID := flag.String("change-sysadmin-uid", "", "changes the sysadmin (through UID) of the system")
	changeSysadminGID := flag.String("change-sysadmin-gid", "", "changes the sysadmin group of the system")
	interval := flag.Int("interval", 30, "defines an amount of time that the system will regularly save persistent stores")

	flag.Parse() // parse arguments
	if flag.NArg() < 2 {
		fmt.Printf("usage: %s <mountpoint> <underlying filesystem>\n", path.Base(os.Args[0])) // show correct usage
		fmt.Printf("\noptions:\n")
		flag.PrintDefaults() // show what optional flags can be used
		os.Exit(2)           // exit w/ error code
	}

	under, err := filepath.Abs(flag.Arg(1))
	if err != nil {
		log.Println("Couldn't get absolute path for underlying filesystem!")
		return
	}
	data := &vfs.OptiFSRoot{
		Path: under,
	}

	// set the options for the filesystem:
	options := &fs.Options{}
	options.Debug = *debug    // set the debug value the user chooses (T/F)
	options.AllowOther = true // Gives users access other than the one that originally mounts it
	sec := time.Duration(0)   // Attempting to prevent caching
	options.EntryTimeout = &sec
	options.AttrTimeout = &sec
	options.MountOptions.Options = append(options.MountOptions.Options, "fsname="+under) // set the filesystem name
	options.NullPermissions = true                                                       // doesn't check the permissions for calls (good for setting up custom permissions [namespaces??])

	root := &vfs.OptiFSNode{
		RootNode: data,
	}

	var dest string
	// if they have chosen a location to save hashmaps and sysadmin info
	if *saveLocation != "" {
		dest = *saveLocation
	} else {
		dest = under + "/../save"
		os.MkdirAll(dest, 0700) // make the directory "save" if it doesn't exist
	}

	if !(*removePersistence) {
		permissions.RetrieveSysadmin(dest)
	}

	// if there is no sysadmin, set the current user as the sysadmin
	if !permissions.SysAdmin.Set {
		permissions.SetSysadmin()
	} else if !permissions.IsUserSysadmin(nil) {
		log.Fatal("You cannot run this OptiFS instance: not a sysadmin.")
	}

	// The user wishes to change the sysadmin UID/GID
	if *changeSysadminUID != "" {
		if permissions.IsUserSysadmin(nil) {
			permissions.ChangeSysadminUID(*changeSysadminUID)
			permissions.SaveSysadmin(dest) // save the changes
			return
		}
	}
	if *changeSysadminGID != "" {
		if permissions.IsUserSysadmin(nil) {
			permissions.ChangeSysadminGID(*changeSysadminGID)
			permissions.SaveSysadmin(dest) // save the changes
			return
		}
	}

	// mount the filesystem
	server, err := fs.Mount(flag.Arg(0), root, options)
	if err != nil {
		log.Fatalf("Mount Failed!!: %v\n", err)
	}

	if !(*removePersistence) {
		metadata.RetrievePersistantStorage(dest) // retrieve the hashmaps
		permissions.RetrieveSysadmin(dest)       // retrieve sysadmin info
		// print for debugging purposes
		metadata.PrintRegularFileMetadataHash()
		metadata.PrintDirMetadataHash()
		metadata.PrintNodePersistenceHash()
		permissions.PrintSysadminInfo()
	}

	if !(*disableIntegrityCheck) {
		metadata.InsureIntegrity()
	}

	log.Println("=========================================================")
	log.Printf("Mounted %v with underlying root at %v\n", flag.Arg(0), data.Path)
	log.Printf("DEBUG: %v", options.Debug)
	log.Printf("RMPERSIST: %v", *removePersistence)
	log.Printf("DISABLEICHECK: %v", *disableIntegrityCheck)
	log.Println("=========================================================")
	// when we are shutting down the filesystem, save the hashmaps

	if !(*removePersistence) {
		go metadata.SaveStorageRegularly(dest, *interval)

		defer func() {
			metadata.SavePersistantStorage(dest)
			permissions.SaveSysadmin(dest)
			// print for debugging purposes
			metadata.PrintRegularFileMetadataHash()
			metadata.PrintDirMetadataHash()
			metadata.PrintNodePersistenceHash()
		}()
	}

	server.Wait()
}
