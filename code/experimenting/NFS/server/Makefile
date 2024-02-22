PHONY: run build stop remove ip follow enter

build:
	docker build -t vfs .

run:
	docker run --privileged -it -d --name nfs-server -v ./logs:/logs -v /home/ryann62/College/Project/2024-CA326-zcollins-OptiFS/code/NFS/server/vol:/nfs-share -p 2049:2049 vfs

stop:
	docker container stop --time=10 nfs-server

remove:
	docker container rm -f nfs-server

ip:
	@docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' nfs-server

follow:
	docker logs -f nfs-server

enter:
	docker exec -it nfs-server /bin/sh
