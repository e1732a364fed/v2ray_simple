# 该Makefile 可支持多种系统上进行编译，但是如果你要打包，给出了 PACK=1，则只能在非windows上用。
#
# for packing into verysimple_xxx.tgz:
#	make PACK=1 linux_amd64
#
# 简单起见，该makefile只负责编译64位的。
#
# for embedding geoip file:
#	make tags="embed_geoip" macm
#
# other tags: noquic nocli
#
# 编译后，还会生成一个 "tags" 和 "BUILD_VERSION" 文件，记录此次编译所使用的 tag 和生成的版本号
#
# 目前发布版直接使用go1.19编译，你如果想编译出相同文件，也要使用go1.19才行

prefix          :=verysimple
winsuffix       :=.exe


dependency                  :=*.go ../../*.go ../../**/*.go ../../**/**/*.go
dependency_with_Makefile    :=Makefile $(dependency)

ifeq ($(OS),Windows_NT) 
    detected_OS   :=Windows
	defaultSuffix :=${winsuffix}
else
    detected_OS := $(shell sh -c 'uname 2>/dev/null || echo Unknown')

ifeq ($(detected_OS),Darwin)
	md5cmd :=md5
else
	md5cmd :=md5sum
endif

md5cmdline:=$(md5cmd) $(dependency) | $(md5cmd)

endif

defaultOutFn    :=${prefix}${defaultSuffix}


# 该Makefile文件 不用来编译官方发布包，官方发布包是用 Makefile_release 编译的。
# 所以这里版本可以随便自己填了 或者自己给出命令行参数来调节. 很棒吧. 
# 参数控制版本号示例:  运行  make BUILD_VERSION=myversion_1


# 默认 BUILD_VERSION  会获取当前git 的 commit id, 以及所有go文件的md5结果总和的md5. 如果你不是 git clone的 而是直接下载的源码文件，则 是获取不到commit的,
# 注意，因为 macos, linux 和 windows 上 所执行的 md5命令的输出略有不同, 所以完全相同的文件在不同系统上实际得到的 总和md5 值是不同的。所以 md5值 要和 build_on 信息 结合 进行判断。

# BUILD_VERSION   :=myversion
BUILD_VERSION   :=[ build_on: $(detected_OS), commit: $(shell git rev-parse --short HEAD), all_go_files_md5: $(shell $(md5cmdline)), tags:$(tags) ]


cmd:=go build -tags "$(tags)"  -trimpath -ldflags "-X 'main.Version=${BUILD_VERSION}' -s -w -buildid=" -o


ifdef PACK
define compile
	CGO_ENABLED=0 GOOS=$(2) GOARCH=$(3) $(cmd) $(1)
	mv $(1) ${prefix}$(4)
	tar -cJf $(1).tar.xz ${prefix}$(4) -C ../../ examples/
	rm ${prefix}$(4)
endef

else

ifeq ($(detected_OS),Windows)

define compile
	set CGO_ENABLED=0&& set GOOS=$(2)&& set GOARCH=$(3)&& $(cmd) $(1)$(4)
endef

else

define compile
	CGO_ENABLED=0 GOOS=$(2) GOARCH=$(3) $(cmd) $(1)$(4)
endef

endif

endif

${defaultOutFn}: BUILD_VERSION $(dependency_with_Makefile) 
	$(call compile, $(prefix),,,$(defaultSuffix))


ifneq ($(detected_OS),Windows)

# https://stackoverflow.com/questions/26145267/how-do-i-force-a-target-to-be-rebuilt-if-a-variable-is-set

# Notes:
#1. echo for macos is not working when using -n;
#2. phony is necessary

define DEPENDABLE_VAR

.PHONY: phony
$1: phony
	@ [[ `cat $1 2>&1` == '$($1)' ]] || /bin/echo -n $($1) > $1
endef

else

define DEPENDABLE_VAR

.PHONY: phony
$1: phony
	@findstr /l /c:"$($1)" $1 >nul 2>&1 || echo|set /p dummyName= "$($1)" > $1
endef

endif

$(eval $(call DEPENDABLE_VAR,BUILD_VERSION))

all: linux_amd64 linux_arm64 android_arm64 macos macm win10 
	@echo "compiling for common platforms"

getver:
	@echo $(BUILD_VERSION)

getmd5:
	@$(md5cmdline)


#这些Fn变量是 PACK开关未打开 时的 可执行文件 的名称 以及 PACK开关打开 时 压缩包的名称, 不是压缩包内部的可执行文件名称. 压缩包内部的可执行文件 统一叫 verysimple

linuxAmdFn     :=${prefix}_linux_amd64
linuxArmFn     :=${prefix}_linux_arm64
androidArm64Fn :=${prefix}_android_arm64
macFn          :=${prefix}_macos
macMFn        :=${prefix}_macm
winFn          :=${prefix}_win10


#注意调用参数时，逗号前后不能留空格

linux_amd64:
	$(call compile,$(linuxAmdFn),linux,amd64)

linux_arm64:
	$(call compile,$(linuxArmFn),linux,arm64)

android_arm64:
	$(call compile,$(androidArm64Fn),android,arm64)

macos:
	$(call compile,$(macFn),darwin,amd64)

#macos apple silicon
macm:
	$(call compile,$(macMFn),darwin,arm64)

win10:
	$(call compile,$(winFn),windows,amd64,$(winsuffix))
winarm:
	$(call compile,$(winFn),windows,arm64,$(winsuffix))



clean:
	rm -f ${prefix}
	rm -f ${prefix}.exe
	rm -f BUILD_VERSION

	rm -f $(linuxAmdFn)
	rm -f $(linuxArmFn)
	rm -f ${winFn}.exe
	rm -f $(macFn)
	rm -f $(macMFn)
	rm -f $(androidArm64Fn)

	rm -f $(linuxAmdFn).tar.xz
	rm -f $(linuxArmFn).tar.xz
	rm -f ${winFn}.tar.xz
	rm -f $(macFn).tar.xz
	rm -f $(macMFn).tar.xz
	rm -f $(androidArm64Fn).tar.xz

rmlog:
	rm -f vs_log
	rm -f vs_log_client*
	rm -f vs_log_server*
	
