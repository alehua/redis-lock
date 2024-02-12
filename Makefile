.PHONY: mock
mock:
	@go get go.uber.org/mock/mockgen/model
	@mockgen -package=mocks -destination=mocks/redis_cmdable.mock.go github.com/redis/go-redis/v9 Cmdable
	@go mod tidy