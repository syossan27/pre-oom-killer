docker-build:
	docker build -f Dockerfile --platform linux/amd64 -t ghcr.io/syossan27/pre-oom-killer:latest .

docker-push:
	docker push ghcr.io/syossan27/pre-oom-killer:latest