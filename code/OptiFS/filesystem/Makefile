.PHONY: rnfs, nfsmount, nfsumount

rnfs:
	@sudo systemctl stop nfsv4-server.service && sudo systemctl start nfsv4-server.service

fsmount:
	@sudo go run ./main.go -debug /nfs-share/vfs/test ./test

fsumount:
	@sudo systemctl stop nfsv4-server.service
	@sudo umount -l /nfs-share/vfs/test
