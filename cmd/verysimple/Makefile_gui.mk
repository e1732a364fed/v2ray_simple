# 本文件的一些解释请参考 Makefile_release.
# 本文件用于编译客户端版本的verysimple。
# 该版本使用cgo。
# gui因为开启了cgo，是较难交叉编译的，建议在目标平台上编译。或者搜索一下 "xgo"

prefix :=vs_gui
winsuffix       :=.exe

cmd:=go build -tags "gui $(tags)"  -trimpath -ldflags "-X 'main.Version=${BUILD_VERSION}' -s -w -buildid="  -o


ifeq ($(OS),Windows_NT) 
    detected_OS   :=Windows
	defaultSuffix :=${winsuffix}
else
    detected_OS := $(shell sh -c 'uname 2>/dev/null || echo Unknown')
endif

ifdef PACK
define compile
	CGO_ENABLED=1 GOOS=$(2) GOARCH=$(3) GOARM=$(5) $(cmd) ${prefix}_$(1)
	mv ${prefix}_$(1) verysimple$(4)
	tar -cJf ${prefix}_$(1).tar.xz verysimple$(4) -C ../../ examples/
	rm verysimple$(4)
endef

else

ifeq ($(detected_OS),Windows)

define compile
	set CGO_ENABLED=1&& set GOOS=$(2)&& set GOARCH=$(3)&& $(cmd) ${prefix}_$(1)$(4)
endef

else
define compile
	CGO_ENABLED=1 GOOS=$(2) GOARCH=$(3) GOARM=$(5) $(cmd) ${prefix}_$(1)$(4)
endef

endif


endif

defaultOutFn    :=${prefix}

${defaultOutFn}:
	$(call compile,native,,,$(defaultSuffix))

all: linux_amd64 linux_arm64 macos macm win10 win10_arm

linux_amd64:
	$(call compile,linux_amd64,linux,amd64)

linux_arm64:
	$(call compile,linux_arm64,linux,arm64)

macos:
	$(call compile,macOS_intel,darwin,amd64)

macm:
	$(call compile,macOS_apple,darwin,arm64)

win10:
	$(call compile,win10,windows,amd64,.exe)

win10_arm:
	$(call compile,win10_arm64,windows,arm64,.exe)


clean:
	rm -f ${prefix}
	rm -f ${prefix}.exe
	rm -f ${prefix}_*
	rm -f *.tar.xz
