# OptiFS User Manual

# Table of Contents

- [1. Installing the System](#1-installing-the-system)
  - [1.1 Installing Fuse3](#11-installing-fuse3)
  - [1.2 Installing Go](#12-installing-go)
  - [1.3 Installing OptiFS](#13-installing-optifs)
- [2. Running OptiFS Locally](#2-running-optifs-locally)
  - [2.1 Specify Mount Point](#21-specify-mount-point)
  - [2.2 Specify Underlying Filesystem](#22-specify-underlying-filesystem)
  - [2.3 Flags](#23-flags)
    - [2.3.1 -change-sysadmin-gid](#231--change-sysadmin-gid)
    - [2.3.2 -change-sysadmin-uid](#232--change-sysadmin-uid)
    - [2.3.3 -debug](#233--debug)
    - [2.3.4 -disable-icheck](#234--disable-icheck)
    - [2.3.5 -rm-persistence](#235--rm-persistence)
    - [2.3.6 -save](#236--save)
- [3. Sysadmin Operations](#3-sysadmin-operations)
  - [3.1 Root Access](#31-root-access)
  - [3.2 Persistent Storage Save Location](#32-persistent-storage-save-location)
  - [3.3 Assignment of Permissions](#33-assignment-of-permissions)
- [4. Mounting Over NFSv4](#4-mounting-over-nfsv4)
- [5. General Operations](#5-general-operations)
- [6. Shutting Down OptiFS](#6-shutting-down-optifs)
  - [6.1 Shutting Down Locally](#61-shutting-down-locally)
  - [6.1 Shutting Down over NFSv4](#61-shutting-down-over-nfsv4)

## 1. Installing the System

### 1.1 Installing Fuse3
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


### 1.2 Installing Go
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


### 1.3 Installing OptiFS
Clone this Gitlab repository and ensure you are in the top-level filesystem package. (2024-CA326-zcollins-OptiFS/code/OptiFS/filesystem).

Once in the correct directory, run the command;
```sh
go install .
```

Go will smartly compile the project and place it in your binaries folder.

You now have OptiFS installed, and it can be run with the `filesystem` command.


## 2. Running OptiFS Locally
OptiFS can be ran by executing a single command:
```sh
filesystem <flags> <mount_point> <underlying_filesystem>
```
**Note:** If the system is being run for the first time, whoever runs the system will be set as a sysadmin. Subsequent runnings of the system require the sysadmin to execute the command.


### 2.1 Specify Mount Point
OptiFS requires a mount point, where the actual virtual filesystem will run. This is what the normal users of the filesystem will see, and is specified with the `<mount_point>` argument.


### 2.2 Specify Underlying Filesystem
Optifs requires an underlying filesystem to be mounted on top of. This is specified with the `<underlying_filesystem>` argument.


### 2.3 Flags
Flags are built-in options for running the filesystem. OptiFS has six flags to choose from:

```sh
usage: filesystem <mountpoint> <underlying filesystem>

options:
  -change-sysadmin-gid string
   	 changes the sysadmin group of the system
  -change-sysadmin-uid string
   	 changes the sysadmin (through UID) of the system
  -debug
   	 enter debug mode
  -disable-icheck
   	 disables the integrity check of the persistent data of the filesystem
  -rm-persistence
   	 remove persistence saving (saving of virtual node metadata)
  -save string
   	 choose the location of saved hashmaps and sysadmin info
```

#### 2.3.1 -change-sysadmin-gid
This flag is used to change the sysadmin group ID. An example usage would be `change-sysadmin-gid=1000`

#### 2.3.2 -change-sysadmin-uid
This flag is used to change the sysadmin user ID. An example usage would be `change-sysadmin-uid=1000`

#### 2.3.3 -debug
This flag, if set, enables logging from the Go Fuse package. This shows information about all kernel requests and replies.

#### 2.3.4 -disable-icheck
This flag, if set, doesn’t check the integrity of persistent data of the filesystem. This will not let OptiFS update its own metadata storage if the state of the underlying filesystem has changed.

#### 2.3.5 -rm-persistence
This flag, if set, will remove persistent saving of virtual node metadata, and sysadmin information.

#### 2.3.6 -save
This flag allows you to choose where exactly you want persistent data to be stored. If not set, OptiFS provides a default location to store this data, and sets the permissions to `0700`.

## 3. Sysadmin Operations

### 3.1 Root Access

### 3.2 Persistent Storage Save Location

### 3.3 Assignment of Permissions

## 4. Mounting Over NFSv4
Firstly, download NFS for your desired distribution and purpose:

| Distro | Machine  | How to Install |
|---|---|---|
| **Arch Linux** | Client & Server | sudo pacman -Sy nfs-utils |
| **Debian** | Server | sudo apt install nfs-kernel-server |
| **Debian** | Client | sudo apt install nfs-common |

To export our filesystem over NFS, ensure you perform the following **IN ORDER**.
1. Configure your `/etc/exports` file and set up your NFS settings correctly, configuring the directories you wish to export.
2. Ensure to include `nohide` and `crossmnt` export options in the parent directory of where you're planning to mount OptiFS.
3. Ensure your configuration is correct **for NFSv4**, as OptiFS only supports NFSv4 - this involves setting the `fsid` option in `/etc/exports`.
4. Run `exportfs -arv` after finishing the modification of your `/etc/exports` file.
5. Mount OptiFS, ensuring it's under the directory with `nohide` and `crossmnt` settings. **In order to export OptiFS over NFS you must run it with root privileges.**
6. Start *specifically* an NFSv4 service (`sudo systemctl start nfsv4-server` for systemd)

Clients should now be able to interact with you OptiFS filesystem over NFS!


## 5. General Operations
As OptiFS is a virtual filesystem, it is operated just like any other filesystem. Normal filesystem operations can be performed, including, but not limited to:
* ls
* cd 
* mkdir
* touch
* rm
* echo

## 6. Shutting Down OptiFS

### 6.1 Shutting Down Locally
To shut down OptiFS locally, simply run the following command:
```sh
sudo umount <mount_point>
```
Where mount_point is the mount point specified at runtime.


### 6.1 Shutting Down over NFSv4
To shut down OptiFS over NFS, you simply perform the same steps as mounting the filesystem, but in reverse order:

1. Stop the NFSv4 service (`sudo systemctl stop nfsv4-server` on Arch)
2. Unmount OptiFS (`sudo umount mount`)

