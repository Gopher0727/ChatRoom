.PHONY: build run tidy test-proto proto-go

build:
	@go build -o build/main.exe cmd/main.go

run: build
	@./build/main.exe

tidy:
	@go mod tidy
	@goimports-reviser -rm-unused -set-alias -format -recursive .
