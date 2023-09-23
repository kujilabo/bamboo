SHELL=/bin/bash

.PHONY: all
all:
	# $(MAKE) lint
	$(MAKE) gen-swagger
	$(MAKE) gen-src
	$(MAKE) gen-proto
	$(MAKE) gazelle
	$(MAKE) update-mod
	$(MAKE) build
	$(MAKE) test
	$(MAKE) dev-docker-build

.PHONY: lint
lint:
	@pushd ./cocotola-api/ && \
		docker run --rm -i hadolint/hadolint < Dockerfile && \
	popd
	@pushd ./cocotola-synthesizer-api/ && \
		docker run --rm -i hadolint/hadolint < Dockerfile && \
	popd
	@pushd ./cocotola-tatoeba-api/ && \
		docker run --rm -i hadolint/hadolint < Dockerfile && \
	popd
	@pushd ./cocotola-translator-api/ && \
		docker run --rm -i hadolint/hadolint < Dockerfile && \
	popd

	@pushd ./cocotola-api/src && \
		golangci-lint run --config ../../.github/.golangci.yml && \
	popd
	@pushd ./cocotola-synthesizer-api/src && \
		golangci-lint run --config ../../.github/.golangci.yml && \
	popd
	@pushd ./cocotola-tatoeba-api/src && \
		golangci-lint run --config ../../.github/.golangci.yml && \
	popd
	@pushd ./cocotola-translator-api/src && \
		golangci-lint run --config ../../.github/.golangci.yml && \
	popd

.PHONY: gen-swagger
gen-swagger:
	@pushd ./cocotola-api/ && \
		swag init -d src -o src/docs && \
	popd
	@pushd ./cocotola-synthesizer-api/ && \
		swag init -d src -o src/docs && \
	popd
	@pushd ./cocotola-tatoeba-api/ && \
		swag init -d src -o src/docs && \
	popd
	@pushd ./cocotola-translator-api/ && \
		swag init -d src -o src/docs && \
	popd

.PHONY: gen-src
gen-src:
	@pushd ./cocotola-api/ && \
		go generate ./src/... && \
	popd
	@pushd ./cocotola-synthesizer-api/ && \
		go generate ./src/... && \
	popd
	@pushd ./cocotola-tatoeba-api/ && \
		go generate ./src/... && \
	popd
	@pushd ./cocotola-translator-api/ && \
		go generate ./src/... && \
	popd

.PHONY: gen-proto
gen-proto:
	@pushd ./bamboo-worker1 && \
	protoc --go_out=./src/ --go_opt=paths=source_relative \
        --go-grpc_out=./src/ --go-grpc_opt=paths=source_relative \
        proto/worker1.proto && \
	popd
	@pushd ./bamboo-worker-redis-redis && \
	protoc --go_out=./src/ --go_opt=paths=source_relative \
        --go-grpc_out=./src/ --go-grpc_opt=paths=source_relative \
        proto/redis_redis.proto && \
	popd
	@pushd ./bamboo-lib && \
	protoc --go_out=./ --go_opt=paths=source_relative \
        --go-grpc_out=./ --go-grpc_opt=paths=source_relative \
        proto/bamboo_lib.proto && \
	popd

.PHONY: update-mod
update-mod:
	@pushd ./lib/ && \
		go get -u ./... && \
	popd
	@pushd ./bamboo-lib/ && \
		go get -u ./... && \
	popd
	@pushd ./bamboo-worker1/ && \
		go get -u ./... && \
	popd
	@pushd ./bamboo-worker-redis-redis/ && \
		go get -u ./... && \
	popd
	@pushd ./bamboo-app1/ && \
		go get -u ./... && \
	popd

# update-deps:
# 	bazel run //tools/update-deps
work-init:
	@go work init

work-use:
	@go work use bamboo-app1 bamboo-worker1 bamboo-worker-redis-redis bamboo-lib lib

gazelle:
	# sudo chmod 777 -R docker/development
	# @bazel run //:gazelle -- update-repos -from_file=./go.work
	@gazelle update-repos -from_file=./go.work
	@bazel run //:gazelle

build:
	@bazel build //...

run-app1:
	@bazel run //bamboo-app1/src

run-worker1:
	@bazel run //bamboo-worker1/src

run-worker-redis-redis:
	@bazel run //bamboo-worker-redis-redis/src

run-request-controller:
	@bazel run //bamboo-request-controller/src

test:
	@bazel test //... --test_output=all --test_timeout=60

docker-build:
	bazel build //src:go_image

docker-run:
	bazel run //src:go_image

dev-docker-up:
	@docker compose -f docker/development/docker-compose.yml up -d
	sleep 10
	# @chmod -R 777 docker/test

dev-docker-down:
	@docker compose -f docker/development/docker-compose.yml down
	sleep 10
	# @chmod -R 777 docker/test

dev-docker-clean:
	@rm -rf docker/development/mysql-*

test-docker-up:
	@docker-compose -f docker/test/docker-compose.yml up -d
	sleep 10
	@chmod -R 777 docker/test

test-docker-down:
	@docker-compose -f docker/test/docker-compose.yml down

.PHONY: dev-docker-build
dev-docker-build:
	@pushd ./cocotola-api/ && \
		docker build -t cocotola-api . && \
	popd
	@pushd ./cocotola-synthesizer-api/ && \
		docker build -t cocotola-synthesizer-api . && \
	popd
	@pushd ./cocotola-tatoeba-api/ && \
		docker build -t cocotola-tatoeba-api . && \
	popd
	@pushd ./cocotola-translator-api/ && \
		docker build -t cocotola-translator-api . && \
	popd
