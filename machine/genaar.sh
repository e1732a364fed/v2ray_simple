go get golang.org/x/mobile/bind
gomobile bind -o xx.aar -androidapi 19 -target=android -tags androidAAR -ldflags "-s -w -buildid=" -trimpath .
go mod tidy