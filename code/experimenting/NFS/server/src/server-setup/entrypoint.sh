#!/bin/bash

# Function to handle SIGTERM
cleanup() {
    echo "Stopping NFS and VFS..."
    fusermount -u /nfs-share

    exit 0
}

# Trap SIGTERM
trap cleanup SIGTERM

# Clearing log files
echo "" > /logs/setup.log
echo "" > /logs/setup.err
echo "" > /logs/vfs.log
echo "" > /logs/vfs.err

# Run virtual filesystem in nfs-share
/setup/VFS -m /nfs-share > /logs/vfs.log 2> /logs/vfs.err &

# Start required services for NFS
rpcbind >> /logs/setup.log 2>> /logs/setup.err
exportfs -r >> /logs/setup.log 2>> /logs/setup.err
rpc.nfsd -G 10 >> /logs/setup.log 2>> /logs/setup.err
service nfs-kernel-server start >> /logs/setup.log 2>> /logs/setup.err

# Keep the container running
tail -f /dev/null
