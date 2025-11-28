.PHONY: build run tidy test-proto proto-go

build:
	@go build -o build/main.exe cmd/main.go

run: build
	@./build/main.exe

tidy:
	@go mod tidy
	@goimports-reviser -rm-unused -set-alias -format -recursive .

proto-go:
	@protoc --proto_path=./internal/api/proto --go_out=. ./internal/api/proto/gateway.proto

# 生成 Dart 代码 (前提：flutter pub global activate protoc_plugin)
# todo
proto-dart:
	protoc --proto_path=./internal/api/proto --dart_out=../im_client_flutter/lib/pb ./internal/api/proto/gateway.proto
