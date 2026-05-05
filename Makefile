APP    := wifiui
PREFIX ?= $(HOME)/.local
BIN    := $(PREFIX)/bin
APPS   := $(PREFIX)/share/applications

.PHONY: setup build run test vet install uninstall tidy clean

setup:
	./script/setup

test:
	go test ./...

vet:
	go vet ./...

build:
	mkdir -p bin
	go build -o bin/ ./cmd/...

run:
	go run ./cmd/$(APP)

tidy:
	go mod tidy

install: build
	install -Dm755 bin/$(APP) $(BIN)/$(APP)
	install -Dm755 script/wifiui-launch $(BIN)/wifiui-launch
	install -Dm644 packaging/$(APP).desktop $(APPS)/$(APP).desktop

uninstall:
	rm -f $(BIN)/$(APP) $(BIN)/wifiui-launch $(APPS)/$(APP).desktop

clean:
	rm -rf bin
