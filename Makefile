
all: build
TAG?=$(shell bash -c 'git log --pretty=format:'%h' -n 1')

FLAGS=
ENVVAR=
GOOS?=darwin
REGISTRY?=686244538589.dkr.ecr.us-east-2.amazonaws.com
BASEIMAGE?=alpine:3.9
#BUILD_NUMBER=$$(date +'%Y%m%d-%H%M%S')
#BUILD_NUMBER := $(shell bash -c 'echo $$(date +'%Y%m%d-%H%M%S')')
include $(ENV_FILE)
export

build: clean dependency wire
	$(ENVVAR) GOOS=$(GOOS) go build -o chart-sync

wire:
	wire

dependency:
	go mod tidy

clean:
	rm -rf chart-sync

run: build
	./chart-sync

.PHONY: build
docker-build-image:  build
	 docker build -t chart-sync:$(TAG) .

.PHONY: build, all, wire, clean, run, set-docker-build-env, docker-build-push, orchestrator,
docker-build-push: docker-build-image
	docker tag chart-sync:${TAG}  ${REGISTRY}/chart-sync:${TAG}
	docker push ${REGISTRY}/chart-sync:${TAG}
