export HUB ?=

docker-build:
	docker build -t ${HUB}/tokenrequest:v0.0.2 .
