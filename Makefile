# 该Makefile 只支持linux中使用. 不过幸亏golang厉害，交叉编译相当简单
#
# for building with filename "verysimple" and pack into verysimple_xxx.tgz:
#	make PACK=1
#
# 我们只支持64位
#
# for embedding geoip file:
#	make tags="embed_geoip" macm1
# 目前发布版直接使用go1.18编译，你如果想编译出相同文件，也要使用go1.18才行

BUILD_VERSION   := v1.0.4

prefix          :=verysimple

linuxAmd        :=_linux_amd64_
linuxArm        :=_linux_arm64_
macosAmd        :=_macos_
macosArm        :=_macos_m1_
windows         :=_win10_



linuxAmdFn:=${prefix}${linuxAmd}${BUILD_VERSION}
linuxArmFn:=${prefix}${linuxArm}${BUILD_VERSION}
macFn     :=${prefix}${macosAmd}${BUILD_VERSION}
macM1Fn   :=${prefix}${macosArm}${BUILD_VERSION}
winFn     :=${prefix}${windows}${BUILD_VERSION}


cmd:=go build -tags $(tags)  -trimpath -ldflags "-X 'main.Version=${BUILD_VERSION}' -s -w -buildid="  -o


ifdef PACK
define compile
	GOOS=$(2) GOARCH=$(3) $(cmd) $(1)
	mv $(1) verysimple$(4)
	tar -czf $(1).tgz verysimple$(4)
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

#我也提供macos 的apple silicon版本.
macm1:
	$(call compile, $(macM1Fn),darwin,arm64)

win10:
	$(call compile, $(winFn),windows,amd64,.exe)


clean:
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

	
