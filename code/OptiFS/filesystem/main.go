package main

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

	"github.com/hanwen/go-fuse/v2/fs"
)

func main() {
	log.Println("Starting OptiFS")
	log.SetFlags(log.Lmicroseconds)
	debug := flag.Bool("debug", false, "enter debug mode")
	removePersistence := flag.Bool("rm-persistence", false, "remove persistence saving (saving of virtual node metadata)")
	disableIntegrityCheck := flag.Bool("disable-icheck", false, "disables the integrity check of the persistent data of the filesystem")
	changeSysadminUID := flag.String("change-sysadmin-uid", "", "changes the sysadmin (through UID) of the system")
	changeSysadminGID := flag.String("change-sysadmin-gid", "", "changes the sysadmin group of the system")

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
	options.Debug = *debug                                                               // set the debug value the user chooses (T/F)
	options.AllowOther = true                                                            // Gives users access other than the one that originally mounts it
	options.MountOptions.Options = append(options.MountOptions.Options, "fsname="+under) // set the filesystem name
	options.NullPermissions = true                                                       // doesn't check the permissions for calls (good for setting up custom permissions [namespaces??])

	root := &vfs.OptiFSNode{
		RootNode: data,
	}

	if !(*removePersistence) {
		permissions.RetrieveSysadmin()
	}

	// if there is no sysadmin, set the current user as the sysadmin
	if !permissions.SysAdmin.Set {
		permissions.SetSysadmin()
	} else if !permissions.IsUserSysadmin() {
		log.Fatal("You cannot run this OptiFS instance: not a sysadmin.")
	}

	// The user wishes to change the sysadmin UID/GID
	if *changeSysadminUID != "" {
		permissions.ChangeSysadminUID(*changeSysadminUID)
		permissions.SaveSysadmin() // save the changes
		return
	}
	if *changeSysadminGID != "" {
		permissions.ChangeSysadminGID(*changeSysadminGID)
		permissions.SaveSysadmin() // save the changes
		return
	}

	// mount the filesystem
	server, err := fs.Mount(flag.Arg(0), root, options)
	if err != nil {
		log.Fatalf("Mount Failed!!: %v\n", err)
	}

	if !(*removePersistence) {
		metadata.RetrievePersistantStorage() // retrieve the hashmaps
		permissions.RetrieveSysadmin()       // retrieve sysadmin info
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
		defer func() {
			metadata.SavePersistantStorage()
			permissions.SaveSysadmin()
			// print for debugging purposes
			metadata.PrintRegularFileMetadataHash()
			metadata.PrintDirMetadataHash()
			metadata.PrintNodePersistenceHash()
		}()
	}

	server.Wait()
}
