# 本文件用于编译release版的 vs_gui, 用于github action中的matrix, 所以不需要在这里指定GOOS

all: build pack

build:
	go build -tags "gui $(tags)"  -trimpath -ldflags "-X 'main.Version=${BUILD_VERSION}' -s -w -buildid="
	
pack:
	tar -cJf ${PACKNAME}.tar.xz verysimple* -C ../../ examples/
	rm verysimple*

pack_gtar:
	gtar -cJf ${PACKNAME}.tar.xz verysimple* -C ../../ examples/
	rm verysimple*