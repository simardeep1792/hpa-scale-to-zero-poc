IMAGE ?= hpa-scale-to-zero-poc:dev
DELL_HOST ?= 100.77.239.77
DEPLOY_USER ?= root

.PHONY: test build deploy-dell

test:
	go test ./...

build:
	docker build -t $(IMAGE) .

deploy-dell:
	DELL_HOST=$(DELL_HOST) DEPLOY_USER=$(DEPLOY_USER) IMAGE=$(IMAGE) ./scripts/deploy-dell.sh
