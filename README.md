
# OptiFS - A Modern Day Optimised Network File Management System

![Pipeline passing status](https://gitlab.computing.dcu.ie/collinz2/2024-ca326-zcollins-optifs/badges/main/pipeline.svg)
![Current release of OptiFS](https://gitlab.computing.dcu.ie/collinz2/2024-ca326-zcollins-optifs/-/badges/release.svg
)

## Description
OptiFS is an intermediary virtual filesystem intended to be used over NFS with large groups of machines and users simultaneously. 

OptiFS strategically intercepts system calls from the user and implements custom behaviour on files or directories to  optimise storage through content deduplication. 

The system is fully compatible with NFSv4, where users can connect to the same server and, in real time, synchronously utilise the filesystem and its storage-saving capabilites.

Additionally, although the system is designed to run seamlessly with NFS, it can also be ran as a standalone local filesystem.

## Getting started

### How To Install and Use
If you would like to install our optimised filesystem, or learn how to use it, see our user manual [here](https://gitlab.computing.dcu.ie/collinz2/2024-ca326-zcollins-optifs/-/blob/main/user_manual/user_manual.md)!

### Learning About The Project
If you would like to know about how the project works, problems we encountered and more, visit our technical manual [here](https://gitlab.computing.dcu.ie/collinz2/2024-ca326-zcollins-optifs/-/blob/main/technical_manual/technical_manual.md)!

## Authors and acknowledgment
[Niall Ryan](mailto:niall.ryan62@mail.dcu.ie) - Main Contributor

[Zoe Collins](mailto:zoe.collins2@mail.dcu.ie) - Main Contributor

## Project status
The current release is v1.0.0, but the project is still in continuous development, and bugs are still to likely arise!
