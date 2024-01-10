# NFS SERVER

# Using UBUNTU as a base for development
FROM ubuntu

# Create a directory for setup
RUN mkdir /setup

# Copy important files in
COPY ./src/server-setup/entrypoint.sh /setup/entrypoint.sh
COPY ./src/VFS/VFS /setup/VFS

# Ensure correct permissions
RUN chmod +x /setup/entrypoint.sh && \
    chmod +x /setup/VFS

# Install NFS and fuse
RUN apt-get update && \
    apt-get install -y nfs-kernel-server fuse

# Create a directory to share
RUN mkdir /nfs-share

# Update the exports file
RUN echo "/nfs-share *(rw,sync,no_subtree_check,no_root_squash,fsid=1)" > /etc/exports

# Make a logs directory
RUN mkdir /logs

# Attach the volume
VOLUME ./logs /logs

# Expose the NFS port
EXPOSE 2049

# Start the NFS server
# Start the NFS server and required services
CMD ["/setup/entrypoint.sh"]
