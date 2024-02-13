package main

import (
	"filesystem/metadata"
	"filesystem/vfs"
	"flag"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/hanwen/go-fuse/v2/fs"
)

func main() {
	log.Println("Starting OptiFS")
	log.SetFlags(log.Lmicroseconds)
	debug := flag.Bool("debug", false, "enter debug mode")
    rmpersist := flag.Bool("rm-persistence", false, "remove persistence saving (saving of virtual node metadata)")

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

	// mount the filesystem
	server, err := fs.Mount(flag.Arg(0), root, options)
	if err != nil {
		log.Fatalf("Mount Failed!!: %v\n", err)
	}

    if !(*rmpersist) {
        metadata.RetrievePersistantStorage() // retrieve the hashmaps
        // print for debugging purposes
        metadata.PrintRegularFileMetadataHash()
        metadata.PrintDirMetadataHash()
        metadata.PrintNodePersistenceHash()
    }

	log.Println("=========================================================")
	log.Printf("Mounted %v with underlying root at %v\n", flag.Arg(0), data.Path)
	log.Printf("DEBUG: %v", options.Debug)
	log.Printf("RMPERSIST: %v", *rmpersist)
	log.Println("=========================================================")

	// when we are shutting down the filesystem, save the hashmaps
    if !(*rmpersist) {
        defer func() {
            metadata.SavePersistantStorage()
            // print for debugging purposes
            metadata.PrintRegularFileMetadataHash()
            metadata.PrintDirMetadataHash()
            metadata.PrintNodePersistenceHash()
        }()
    }

	server.Wait()
}
