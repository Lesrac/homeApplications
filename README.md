# Pre-requisites
To run locally (for testing) install flyway-cli.

# How To build
```
go build
```

## Create binary for server
Check available distributions:
```
go tool dist list
```
Check distro on server, example for Unix:
```
uname -a
```
Build for server:
```
$env:GOOS = 'linux'
$env:GOARCH = 'arm64'
go build
```