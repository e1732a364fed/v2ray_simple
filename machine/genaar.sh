go get golang.org/x/mobile/bind
gomobile bind -v -o xx.aar -target=android -ldflags "-s -w -buildid=" -trimpath .
go mod tidy