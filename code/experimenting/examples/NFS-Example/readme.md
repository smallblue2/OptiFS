# NFS Docker Example
This is a simple docker example that spins up an NFS server and client, which share a volume through NFS.

The volume that it shares is also linked to the host machine, as it's the `nfs-volume` directory within this directory.

# Use Example
Spin up the containers & network:
`docker-compose up --build -d`

Make a file in the nfs-volume called 'hello' from the client:
`docker exec -it nfs-client sh -c "cd /mnt/nfs && touch hello"`

View the created files:
`docker exec nfs-server ls -l /nfsshare`
