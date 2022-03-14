BUILD_VERSION   := v1.0.3

prefix:=verysimple

cmd:=go build  -trimpath -ldflags "-X 'main.Version=${BUILD_VERSION}' -s -w -buildid="  -o

all: win10 linux_amd64 linux_arm64 macos_arm64



linux_amd64:
	GOARCH=amd64 GOOS=linux $(cmd) ${prefix}_linux_amd64_${BUILD_VERSION}

linux_arm64:
	GOARCH=arm64 GOOS=linux $(cmd) ${prefix}_linux_arm64_${BUILD_VERSION}

#我只提供mac 的apple silicon版本.

macos_arm64:
	GOARCH=arm64 GOOS=darwin $(cmd) ${prefix}_macos_${BUILD_VERSION}

win10:
	GOARCH=amd64 GOOS=windows $(cmd) ${prefix}_win10_${BUILD_VERSION}.exe


clean:
	rm -f ${prefix}_linux_amd64_${BUILD_VERSION}
	rm -f ${prefix}_linux_arm64_${BUILD_VERSION}
	rm -f ${prefix}_win10_${BUILD_VERSION}.exe
	rm -f ${prefix}_macos_${BUILD_VERSION}
