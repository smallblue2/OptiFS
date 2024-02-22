# OptiFS Technical Specification

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
