# golang implementation of DataPower Commander

## Workspace

We have to set GOPATH variable to go directory.

Under go/src are go projects (created and downloaded)

## Installing libaries

```
go get github.com/nsf/termbox-go
go get github.com/antchfx/jsonquery
go get github.com/antchfx/xmlquery
go get github.com/howeyc/gopass
```

## Run

From project directory:

```go
go run dpcmder.go
```

From anywhere as long as $GOPATH is set:

```go
go run github.com/croz-ltd/dpcmder
```


## Build

Build from project directory:

```sh
go build dpcmder.go

GOOS=windows GOARCH=386 go build -o dpcmder-win-386.exe dpcmder.go
GOOS=windows GOARCH=amd64 go build -o dpcmder-win-amd64.exe dpcmder.go
GOOS=darwin GOARCH=386 go build -o dpcmder-mac-386 dpcmder.go
GOOS=darwin GOARCH=amd64 go build -o dpcmder-mac-amd64 dpcmder.go
GOOS=linux GOARCH=386 go build -o dpcmder-linux-386 dpcmder.go
GOOS=linux GOARCH=amd64 go build -o dpcmder-linux-amd64 dpcmder.go
```
