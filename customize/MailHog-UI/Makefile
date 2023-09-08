all: bindata fmt ui

ui:
	go install .

bindata:
	rm -f assets/assets.go
	go-bindata -o assets/assets.go -pkg assets assets/...

fmt:
	go fmt ./...

.PNONY: all ui bindata fmt
