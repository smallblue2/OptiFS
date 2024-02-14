package main

import (
	"filesystem/metadata"
	"filesystem/permissions"
	"filesystem/vfs"
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"path"
	"strconv"

	"github.com/hanwen/go-fuse/v2/fs"
)

var SysadminUID, SysadminGID uint32

func main() {
	log.Println("Starting OptiFS")
	log.SetFlags(log.Lmicroseconds)
	debug := flag.Bool("debug", false, "enter debug mode")
	removePersistence := flag.Bool("rm-persistence", false, "remove persistence saving (saving of virtual node metadata)")
	disableIntegrityCheck := flag.Bool("disable-icheck", false, "disables the integrity check of the persistent data of the filesystem")

	flag.Parse() // parse arguments
	if flag.NArg() < 2 {
		fmt.Printf("usage: %s <mountpoint> <underlying filesystem>\n", path.Base(os.Args[0])) // show correct usage
		fmt.Printf("\noptions:\n")
		flag.PrintDefaults() // show what optional flags can be used
		os.Exit(2)           // exit w/ error code
	}

	under := flag.Arg(1)
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

	permissions.RetrieveSysadmin()

	// get the UID and GID of the sysadmin that runs the filesystem
	// this is saved (persisent), so we only need to get it once
	if !permissions.SysAdmin.Set {
		log.Println("No Sysadmin found, setting user as sysadmin.")

		sysadmin, sErr := user.Current() // get the current user
		if sErr != nil {
			log.Fatalf("Couldn't get sysadmin info!: %v\n", sErr)
		}

		u, uErr := strconv.Atoi(sysadmin.Uid) // get the UID
		if uErr != nil {
			log.Fatalf("Couldn't get sysadmin UID!: %v\n", uErr)
		}

		g, gErr := strconv.Atoi(sysadmin.Gid) // get the GID
		if gErr != nil {
			log.Fatalf("Couldn't get sysadmin GID!: %v\n", gErr)
		}

		// fill in sysadmin details
		permissions.SysAdmin.UID = uint32(u)
		permissions.SysAdmin.GID = uint32(g)
		permissions.SysAdmin.Set = true

	}

	// mount the filesystem
	server, err := fs.Mount(flag.Arg(0), root, options)
	if err != nil {
		log.Fatalf("Mount Failed!!: %v\n", err)
	}

	if !(*removePersistence) {
		metadata.RetrievePersistantStorage() // retrieve the hashmaps
		permissions.RetrieveSysadmin()
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
