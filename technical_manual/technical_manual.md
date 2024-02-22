# OptiFS Technical Specification

## 1. Introduction

### 1.1 Overview
OptiFS is an intermediary virtual filesystem to be used over NFS with large groups of machines and users simultaneously. It is a loopback filesystem, placed on top of the existing EXT4 filesystem on linux based machines. OptiFS strategically intercepts system calls from the user and implements custom behaviour on files or directories, optimising storage. OptiFS also employs deduplication, by using content based hashing to determine which files are unique, and significantly reducing storage redundancy by using hard links to link duplicate files to a single “main” file, all while simulating uniqueness through our own custom metadata.

The system is mounted over NFS, where users can connect to the same server and, in real time, synchronously view, edit, create, delete, move and rename files and directories seamlessly. Although the system is designed to run seamlessly with NFS, it can also run as a standalone filesystem.

### 1.2 Glossary
**NFS** - protocol for defining a network file system, stores files over a network.

**Loopback filesystem** - mounting a virtual filesystem on top of an existing filesystem.

**FUSE** - a filesystem in userspace which allows users to create their own filesystem, without writing in the kernel.

**VFS** - the Linux virtual file system, which provides a filesystem interface to programs like FUSE, a layer in the kernel.

**BLAKE3** - an extremely fast and secure cryptographic hash function. 

**Sysadmin** - the general maintainer of OptiFS (system administrator)

**UID** - user ID of a user.

**GID** - group ID that the user is in.

## 2. System Architecture

Figure 1: Package diagram of First Party packages.

### 2.1 First-Party Packages
First-Party packages are custom packages developed by the development team solely for use within this project.

#### 2.1.1 Filesystem

##### 2.1.1.1 Description
The filesystem package is the overall system as a whole. It is the highest-level package and contents the project in its entirety. The filesystem package contains all first-party packages underneath, and most importantly the main.go file - which is the entrypoint for the program.
##### 2.1.1.2 Responsibilities
Its responsibilities consist of all high-level operations in the filesystem program;

- Interpreting program flags and adjusting behaviour accordingly.
- Loading persistent data in memory
  - The current sysadmin, the user who is permitted to run the program.
  - Important information for nodes to maintain between program instances in order to ensure correct operation of custom filesystem permissions and node metadata.
- Connecting the program to the underlying filesystem.
- Triggering data integrity checks, making sure that persistent data matches the underlying filesystem.
- Mounting the FUSE virtual filesystem to the specified mountpoint.
- Saving important data from memory to hard disk on filesystem close.

#### 2.1.2 Hashing

##### 2.1.2.1 Description

The hashing package hashes the contents of a file using the BLAKE3 hashing algorithm. This algorithm was chosen due to its lightning speeds and low collision probabilities - crucial for our project. Although small, this is a vital component of the system, as it is essential for detecting duplicate content efficiently.

##### 2.1.2.2 Responsibilities
It is responsible for the hashing of file content.
- Hashing data of variable length and returning it as a 64-byte hash. 

#### 2.1.3 VFS

##### 2.1.3.1 Description
VFS (Virtual FileSystem) is a substantial and large package in our system that represents the filesystem and FUSE functionality of the program. Primary files consist of a node.go which represents a node and all node operations in our FUSE filesystem, and file.go which represents a file handle (file descriptor) in our FUSE filesystem and all file handle operations.

##### 2.1.3.2 Responsibilities
This package is responsible for simulating a filesystem. It intercepts filesystem syscalls and performs operations as defined by the VFS package.
 - OptiFSRoot attributes and operations.
   - Storing the path to the root of the underlying filesystem.
   - Instantiation of new OptiFSNodes.
   - Creating their virtual stable attributes (inode, generation, mode).
 - OptiFSNode attributes and operations.
   - Storing the OptiFSRoot for referencing.
   - 29 different file system syscalls (Lookup, Create, Mknode, Link, Rename, etc…).
 - OptiFSFile attributes and operations
   - Mutex for synchronisation.
   - Underlying filesystem’s file descriptor.
   - Stable attributes of the node belonging to the file.
   - Flags that the node was opened with (RDONLY, WRONLY, etc…).
   - 8 different file system syscalls (Read, Flush, Release, etc…).

#### 2.1.4 Permissions

##### 2.1.4.1 Description
The permissions package consists of our custom permissions system which handles and defines security and access to resources (nodes) in our FUSE filesystem. All of these checks can be found in the permissions.go file. Alongside this, there is a file named sysadmin.go, which monitors the sysadmin role and any updates to it. This is a vital part of the system, as OptiFS is designed so that it can only be run by a known sysadmin, and known sysadmins are the only users who can perform operations in the root directory.

##### 2.1.4.2 Responsibilities
This package is responsible for checking access of files and directories, and handling sysadmin operations.
 - Verify that the user has appropriate permissions (read/write/execute) in a file or directory.
 - Check if the user is the owner of a file or directory, and if they have read or write intent.
 - Extracting the UID and GID of a user.
 - Managing current sysadmin information:
   - Saving and retrieving (persistent) information.
   - Setting a (new) sysadmin - if their UID or GID is valid.
   - Query if the current user is a sysadmin.

#### 2.1.5 Metadata

##### 2.1.5.1 Description
This package is responsible for our custom metadata system. We de-duplicate content by the hash of its contents and perform hard links on the underlying filesystem. In order to maintain uniqueness and ownership of duplicate files, it is necessary for us to manage our own metadata for nodes using our own bespoke system - and this package is solely responsible for the mechanisms to make that happen and ensure they’re persistent between program instances.

##### 2.1.5.2 Responsibilities
This package is responsible for managing, storing and retrieving custom node metadata for our FUSE virtual filesystem.
 - Storing and retrieving non-directory metadata by content hashes and reference numbers.
 - Storing and retrieving directory metadata by paths.
 - Storing important node information for persistence between program instances.
 - Providing convenient APIs to other packages for the modification of node custom metadata.

### 2.2 Third-Party Packages

#### 2.2.1 hanwen/go-fuse
These are Go native bindings for the FUSE kernel module written by github user hanwen (Han-Wen Nienhuys).

We chose this library as it seems to be well maintained with comprehensive and up to date protocol support and performance that is competitive with libfuse. It also has a ‘BSD 3 Clause’ licence, which allows us to use it for this project.

This is used to interface with FUSE.

#### 2.2.2 x/sys/unix
This community-driven packge provides access to the raw system call interface of the underlying operating system.

We use this for our atomic rename exchange, using the Renameat2 syscall and RENAME_EXCHANGE flag.

#### 2.2.3 lukechampine.com/blake3
An implementation of the BLAKE3 cryptographic hash function in Go.

BLAKE3 was used as a hashing algorithm as it is deterministic, has a very low chance of collisions, and more importantly is blazingly fast!

We use this for hashing the content of files.

### 2.3 Go Standard Library

#### 2.3.1 syscall
Syscall allows the user to make calls to operating system primitives. It is used in OptiFS to perform operations on the underlying filesystem, such as opening a file, stat-ing a file etc.

#### 2.3.2 log
Log allows the user to print logging messages to the terminal. It is used in OptiFS as a form of debugging and to show what operations are being performed. 

#### 2.3.3 sort
Sort sorts slices in ascending order. It is used in OptiFS to sort extracted attributes from our metadata store, as there is no guaranteed order and we want to create deterministic behaviour.

#### 2.3.4 bytes
Bytes allows the user to work with byte slices. It is used in OptiFS to manage the buffer for hashing large files in blocks. It is also used so there is a temporary buffer to write attributes to and then copy them across.

#### 2.3.5 testing
Testing helps the user in providing automated testing. It is used in OptiFS to implement extensive unit testing in each package.

#### 2.3.6 reflect
Reflect helps the user in examining and even modifying complicated object’s structure and behaviour at runtime. It is exclusively used in OptiFS testing, where it compares complicated objects (structs) to each other.

#### 2.3.7 encoding/gob
Encoding/gob is used for encoding and decoding data structures, and is quite fast. It is used in OptiFS to serialise and deserialise all of the persistent stores and structures, for example sysadmin information.

#### 2.3.8 os
OS allows the user to interface with operating system functionality. It is used in OptiFS in the form of more safely/atomically creating and opening of files as opposed to the syscall package, while handling persistence.

#### 2.3.8.1 os/user
OS/user allows the user to lookup other users of the system’s information. It is used in OptiFS predominantly in the sysadmin functions, to check if the current user is a sysadmin, or if their UID/GID is valid.

#### 2.3.9 sync
Sync provides the user with lock functionality, so that processes can be blocked depending on who is doing what. It is used to create locks for all functions in OptiFS. This is a crucial feature, as operations over NFS need to be locked to prevent unusual behaviour.

#### 2.3.10 context
Context gets information about a specific request and the environment it is being performed in. They can be passed from function to function without error. It is used frivolously in OptiFS throughout all the different file and node functions.

#### 2.3.12 strconv
Strconv helps the user to convert strings to other types, such as floats or ints. It is used in OptiFS to convert attributes from strings to uint32, most notably in regards to the UID and GID of a user.

#### 2.3.13 fmt
Fmt allows the user to print to the terminal (I/O operations). However it can also be used to assist with formatting, with functions such as Sprintf (format the variable in a specific way, e.g. %x for hex), which is how it is used in OptiFS.

#### 2.3.14 time
Time is used to measure and display the current time. It is used in OptiFS to aid in setting the access time, modified time and change time.

#### 2.3.15 unsafe
Unsafe allows the user to disregard certain areas of type safety. This had to be used in OptiFS to get around passing the addresses of data types, which is not typically allowed in Go.

#### 2.3.16 encoding/binary
Encoding/binary allows for translation to and from binary, and encoding/decoding these translations. This is used in OptiFS to create inode numbers from a file’s hash.

#### 2.3.17 path/filepath
Path/filepath gets paths in regards to the operating system being used; for example using “/” on Linux, and “\” on Windows. It is used in OptiFS to get the absolute path of the underlying filesystem, so that it can be mounted properly over NFS.

#### 2.3.18 flag
Flag allows the user to define certain flags their program can be run with, and the behaviour these flags allow. It is used in OptiFS to set flags to define the operation of the system, for example running without persistence, and to change sysadmin users.

## 3. High Level Design

### 3.1 Data Flow

### 3.2 Components ????

### 3.3 Use Cases and Sequences


## 4. Problems & Resolutions
Writing this software proved to be the most challenging project either of us have ever worked on, and as a result we had many potentially project-ruining problems that we thankfully managed to get past. Below are a list of some large ones, but is certainly not even close to an exhaustive list of large issues that substantially delayed development, but were overcome.

### 4.1 Basic FUSE Implementation

#### 4.1.1 Problem
We were quite naive going into this project as we were completely unaware of FUSE or what a virtual filesystem was and we especially lacked expertise in how filesystems operated, so just coming to grips with FUSE actually possibly took a month or more. This was especially due to our Go bindings for FUSE containing lackluster documentation and often being different enough in certain areas to often invalidate the libfuse documentation.

#### 4.1.2 Resolution
We followed multiple different examples, like an ‘in-memory’ filesystem, to get to grips with how it works. We also started to become familiar with FUSE as a technology, and specifically its documentation.

### 4.2 NFS and FUSE Compatibility

#### 4.2.1 Problem
FUSE and NFS’s compatibility is not well documented in the slightest, and seems to be an incredibly niche usage of the two technologies. Unfortunately, this seems to be for good reason.

They are fundamentally different in their design. NFS assumes it’s talking directly to a well-behaved, kernel-level filesystem, whereas FUSE exists in userspace, outside of the privilege of the kernel, introducing additional challenges and overhead.

FUSE gives you immense control over how your filesystem behaves, whereas NFS strictly expects traditional filesystem behaviour.

Finally, NFS viciously caches data to speed up access, but this introduces great difficulties as the FUSE filesystem must ensure to invalidate this cache with all changes that it introduces.

All of these various problems and even more ensured that this was immensely challenging.

#### 4.2.2 Resolution
Unfortunately with a lack of online resources on the matter, we had to just perform incredible amounts of experimentation and a lot of trial-and-error until we got them working with each other in some manner. Typically, every week or so we’d manage to get NFS interfacing with more and more parts of our filesystem as we continued development.

### 4.3 Mounting NFS On Our FUSE Filesystem

#### 4.3.1 Problem
We found that getting our filesystem to even mount over NFS to be a substantial challenge, and definitely delayed us substantially in early development as our main focus early on was ensuring these two technologies were compatible with each other.

#### 4.3.2 Resolution
Resolving this problem required research into mount options for NFS, and quite simply a lot of experimentation.

In order to get FUSE to be correctly shared over NFS, you must;
 - Ensure your FUSE filesystem correctly implements STATFS.
 - Mount your FUSE filesystem in your shared directory BEFORE starting the NFS service.
 - Ensure your FUSE filesystem isn’t mounted at the root of the directory being shared over NFS.
 - Ensure you are using NFSv4, older versions of NFS do not support FUSE.
 - Ensure you are using the crossmnt or nohide config setting to allow the traversal of other filesystems mounted within the exported directory.

### 4.4 Content Deduplication

#### 4.4.1 Problem
Originally, we knew that we wanted to approach this problem through hashing the contents of a file and comparing it to a store to otherwise tell if it is unique or not. However, the actual process of deduplication was still pretty undefined for us, especially the underlying mechanisms and how we would link duplicate nodes to the same memory. We had put some thought into it, with the use of a ‘garbage collector’ in our functional specification and such, but it was all high-level planning.

#### 4.4.2 Resolution
We solved this through the creation of a specific type of virtual filesystem called a loopback filesystem. This is a virtual filesystem that sits on top of another filesystem, and utilises the underlying filesystem for persistent storage, but defines custom behaviour for filesystem operations.

We then hash regular file content, compare it to an internal store to see if it actually exists. If it does exist, ignore the write and create a hardlink in the underlying filesystem. If it doesn’t exist, perform the write. In both scenarios we then create our own custom metadata entry. Custom metadata is used to simulate duplicate files which are simply hardlinked underneath, as unique files with unique attributes such as ownership, permissions, timestamps, etc.

This approach fully utilises the underlying filesystem’s filesystem tree and storage mechanisms to offload the majority of data handling to the underlying filesystem.

Furthermore, this ensures that we don’t have to employ a garbage collector, as when all instances of a hardlink are unlinked, naturally it is removed.

### 4.5 Flawed Node Instantiation

#### 4.5.1 Problem
We had strange behaviour, where although we had our custom metadata system written and working correctly, when we created a duplicate file it wouldn’t be unique, and any changes to one file would be reflected in the other which was undesired behaviour.

This was a very tricky issue to diagnose and substantially delayed us again.

#### 4.5.2 Resolution
We found out that FUSE keeps track of nodes exclusively through the virtual inodes we provide. Originally, we utilised information of the node residing on the underlying filesystem to instantiate our virtual node, including the virtual inode number. Additionally, Nodes are instantiated anytime an operation is performed on them. 

We then realised the flaw in our implementation. When you create a duplicate file, the underlying node created is a hardlink. So when we instantiate our virtual node from the underlying node, we essentially instantiate the same virtual node attached to the source of the underlying hardlink, as opposed to a unique virtual node with a unique inode.

We fixed this by only instantiating a virtual node once, and then storing its data in a persistent store. We then instead query the persistent store for node information during instantiation, as opposed to relying on the underlying node.

This fixed our problem, and ensured unique virtual nodes irregardless of underlying nodes being hardlinked or not.

### 4.6 Lack of Directory Permissions

#### 4.6.1 Problem
Throughout the later stages of development, we solely had custom metadata for regular files, as we were simply passing requests to do with directories straight down to the underlying filesystem.

However, we found out that permissions meant nothing in our FUSE virtual filesystem, and no matter what they were set to in the underlying filesystem, our virtual filesystem was completely ignoring them, as if they didn’t even exist.

#### 4.6.2 Resolution
In order for FUSE to ignore the underlying filesystem’s permissions and utilise the permissions stated in our own custom metadata, we had to set a FUSE option ‘NullPermissions’ which causes it to bypass traditional unix-style file permissions (the read, write and execute bits for owner, group and others).

We quickly realised that although we were planning on just having custom metadata and permissions for regular files, and the rest being passed directly down to the underlying filesystem, it wasn’t possible - you either have everything on traditional unix-style file permissions, or nothing respecting them, you can’t have it be a hybrid.

But our metadata system was built upon being retrievable by the content’s hash. Directories don’t necessarily have content that you can hash - so we had to make a custom metadata system specifically tailored to directories as well in order to enforce permissions on them with our permission package, which in turn fixed our problem.

### 4.7 Broken Content Deduplication For Large Files

#### 4.7.1 Problem
We discovered late into the project that large files completely crashed our filesystem. It was originally found out that it was a problem with writing in our filesystem, where all of our deduplication logic is performed.

#### 4.7.2 Resolution
In our Write syscall implementation, we originally hashed the content being written and checked if it already existed, and then continued to either perform deduplication if it wasn’t unique, or simply allow the write if it was unique, and then create custom metadata for the file through this hash.

What we didn’t anticipate - for some reason - is that this works perfectly for small files that required one write. But when files are large, they are written to in blocks, and this will cause FUSE’s Write syscall to be called many times, depending on the block size of your filesystem I believe. For example, a 10gb file that we tested required 80,000 writes before it’s file descriptor was released.

We fixed this by moving the deduplication logic to when we close a file descriptor, in Release. This also required us to be more creative with how we generate the hash for the file’s content, we solved this by simply hashing each block written to disk, storing it in a buffer, and then hashing the entire buffer in Release. This prevented the entire content of a file being required to be loaded into memory to hash it, and it also ensured determinism, speed and efficiency.

This also introduced another problem where we were performing deduplication on all file descriptor closes, including ones for reads and executions. So we simply check the original open flags for writing intent, and perform deduplication only if the original file descriptor was used to perform writes.

### 4.8 Incorrect NFS Synchronisation

#### 4.8.1 Problem
We noticed that when testing our filesystem over NFS, any Create or Write operations by NFS clients were performed by our FUSE filesystem, but the directory entries weren’t reflected by the Client.

This meant that a client could create a regular file over NFS, and still interact with it in terms of reading, writing and executing, but it couldn’t perform lookups on the file, or see it as a child of any directory.

This was very strange behaviour as directories and special files (created through Mknod as opposed to Create) were working perfectly.

#### 4.8.2 Resolution
We originally thought this was due to NFS’s nature of viciously caching filesystem information, and that our FUSE filesystem wasn’t invalidating NFS’s cache correctly, causing it to display out-of-date information.

We then spent a week trying to invalidate the NFS cache, even going as far as to turning it off, which gave us slightly better behaviour but still with the bug persisting.

We then thought it may have been FUSE’s cache, and spent a long time trying to smartly provide hints to the kernel as to not cache our information. But this didn’t work either.

We then noticed that in the hanwen/go-fuse package there was an attribute ‘Gen’, which allows you to reuse the same inode number, as long as the ‘Gen’ is different. But as the documentation specifically talks about NFS for this part, we thought this might have something to do with FUSE giving hints to NFS for its caching, so we tried incrementing this anytime a file changed. But this, again, didn’t work.

Then, we found out that we had forgotten to implement our timestamps correctly being updated. So we implemented the updating of accessed and modified timestamps of the custom metadata of our nodes. But this did not work either.

Finally, we realised that CTIM stood for ‘change time’, as opposed to what we assumed was ‘creation time’, and as NFS’s caching system works on a CHANGE counter, we thought we had a good lead here. We then implemented the updating of our CHANGE timestamp, and it finally worked.

This was a huge rabbit hole that spanned a large duration of the project, but thankfully was fixed by the end of it.

### 4.9 Implementing Persistence

#### 4.9.1 Problem
We needed a way to ensure persistence of our custom metadata system between instances of our filesystem being run, considering it is simply just a userspace program. This was a unique problem that required a careful approach to ensure data integrity.

#### 4.9.2 Resolution
After doing research, we discovered that go has a built-in encoding library which allowed us to convert our data stores to and from binary as we pleased. Alongside this discovery, we also found that go converts these into “gobs” (go binaries) of data, which hold information about the type of structure that was encoded (struct, map, etc.).

## 5. Installation Guide

### 5.1 Fuse3
Our project is built with Fuse3, so ensure you have it installed before installing our software.

Simply install with your respective package manager.

Arch Linux:
```sh
sudo pacman -Sy fuse3
```
Debian:
```sh
sudo apt install fuse
```
> **⚠️ WARNING**: Installing FUSE3 on Ubuntu versions >=22.04 can break your system. [See here](https://askubuntu.com/questions/1409496/how-to-safely-install-fuse-on-ubuntu-22-04).

### 5.2 Go
Install a recent version of Go (1.21 or later is recommended). This can be easily done through your package manager.

Arch Linux:
```sh
sudo pacman -Sy go
```

Debian:
```sh
sudo apt install golang
```

**Note on GOPATH**: Modern GO development often uses modules, and you might not need to explicitly set your GOPATH environment variable explicitly. For more information on go, see [https://go.dev/doc/modules](https://go.dev/doc/modules).

### 5.3 OptiFS
Clone this Gitlab repository and ensure you are in the top-level filesystem package. (2024-CA326-zcollins-OptiFS/code/OptiFS/filesystem).

Once in the correct directory, run the command;
```sh
go install .
```

Go will smartly compile the project and place it in your binaries folder.

You now have OptiFS installed, and it can be run with the `filesystem` command.

### 5.4 NFSv4 (Optional)

#### 5.4.1 Installation of NFSv4
Our filesystem is designed to run locally-first. It simply just has compatibility with NFS. That being said however, one single user is unlikely to have many duplicate files - so the primary application for this filesystem would be an environment with many users.

To install NFSv4, use your package manager.

Arch Linux (server and client):
```sh
sudo pacman -Sy nfs-utils
```

Debian (server):
```sh
sudo apt install nfs-kernel-server
```

Debian (client):
```sh
sudo apt install nfs-common
```

#### 5.4.2 Exporting OptiFS over NFSv4
To export our filesystem over NFS, ensure you perform the following **IN ORDER**.
1. Configure your `/etc/exports` file and set up your NFS settings correctly, configuring the directories you wish to export.
2. Ensure to include `nohide` and `crossmnt` export options in the parent directory of where you're planning to mount OptiFS.
3. Ensure your configuration is correct **for NFSv4**, as OptiFS only supports NFSv4 - this involves setting the `fsid` option in `/etc/exports`.
4. Run `exportfs -arv` after finishing the modification of your `/etc/exports` file.
5. Mount OptiFS, ensuring it's under the directory with `nohide` and `crossmnt` settings. **In order to export OptiFS over NFS you must run it with root privileges.**
6. Start *specifically* an NFSv4 service (`sudo systemctl start nfsv4-server` for systemd)

Clients should now be able to interact with you OptiFS filesystem over NFS!

It is up to the individual to manage the security of your filesystem over NFS.
