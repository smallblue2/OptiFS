PHONY: run build stop remove ip follow enter

build:
	docker build -t vfs .

run:
	docker run --privileged -it -d --name nfs-server -p 2049:2049 -v ./logs:/logs vfs

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
