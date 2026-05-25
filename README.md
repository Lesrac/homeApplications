# Pre-requisites
To run locally (for testing) run docker compose.

# How To build
```
go build
```
run the built "homeApplications" file corresponding to your OS.

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