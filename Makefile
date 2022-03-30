# 该Makefile 只支持linux中使用. 不过幸亏golang厉害，交叉编译相当简单
#
# for building with filename "verysimple" and pack into verysimple_xxx.tgz:
#	make PACK=1
#
# 我们只支持64位
#
# for embedding geoip file:
#	make tags="embed_geoip" macm1
#
# 目前发布版直接使用go1.18编译，你如果想编译出相同文件，也要使用go1.18才行

BUILD_VERSION   := v1.1.1

prefix          :=verysimple

linuxAmd        :=_linux_amd64
linuxArm        :=_linux_arm64
macosAmd        :=_macos
macosArm        :=_macm1
windows         :=_win10

#这些Fn变量是用于发布包压缩包的名称，不是可执行文件名称，可执行文件统一叫 verysimple

linuxAmdFn:=${prefix}${linuxAmd}
linuxArmFn:=${prefix}${linuxArm}
macFn     :=${prefix}${macosAmd}
macM1Fn   :=${prefix}${macosArm}
winFn     :=${prefix}${windows}


cmd:=go build -tags $(tags)  -trimpath -ldflags "-X 'main.Version=${BUILD_VERSION}' -s -w -buildid="  -o


ifdef PACK
define compile
	GOOS=$(2) GOARCH=$(3) $(cmd) $(1)
	mv $(1) verysimple$(4)
	tar -czf $(1).tgz verysimple$(4) examples/
	rm verysimple$(4)
endef

else

define compile
	GOOS=$(2) GOARCH=$(3) $(cmd) $(1)$(4)
endef
endif


all: win10 linux_amd64 linux_arm64 macos macm1

#注意调用参数时，逗号前后不能留空格

linux_amd64:
	$(call compile, $(linuxAmdFn),linux,amd64)

linux_arm64:
	$(call compile, $(linuxArmFn),linux,arm64)

macos:
	$(call compile, $(macFn),darwin,amd64)

#提供macos 的apple silicon版本.
macm1:
	$(call compile, $(macM1Fn),darwin,arm64)

win10:
	$(call compile, $(winFn),windows,amd64,.exe)


clean:
	rm -f verysimple
	rm -f verysimple.exe

	rm -f $(linuxAmdFn)
	rm -f $(linuxArmFn)
	rm -f ${winFn}.exe
	rm -f $(macFn)
	rm -f $(macM1Fn)

	rm -f $(linuxAmdFn).tgz
	rm -f $(linuxArmFn).tgz
	rm -f ${winFn}.tgz
	rm -f $(macFn).tgz
	rm -f $(macM1Fn).tgz

	
