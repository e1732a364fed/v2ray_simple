BUILD_VERSION   := v1.0.3

prefix          :=verysimple

linuxAmd        :=_linux_amd64_
linuxArm        :=_linux_arm64_
macosArm        :=_macos_m1_
windows         :=_win10_

linuxAmdFn:=${prefix}${linuxAmd}${BUILD_VERSION}
linuxArmFn:=${prefix}${linuxArm}${BUILD_VERSION}
macM1Fn   :=${prefix}${macosArm}${BUILD_VERSION}
winFn     :=${prefix}${windows}${BUILD_VERSION}

cmd:=go build  -trimpath -ldflags "-X 'main.Version=${BUILD_VERSION}' -s -w -buildid="  -o

all: win10 linux_amd64 linux_arm64 macos_arm64



linux_amd64:
	GOARCH=amd64 GOOS=linux $(cmd) $(linuxAmdFn)

linux_arm64:
	GOARCH=arm64 GOOS=linux $(cmd) $(linuxArmFn)

#我只提供mac 的apple silicon版本.

macos_arm64:
	GOARCH=arm64 GOOS=darwin $(cmd) $(macM1Fn)

win10:
	GOARCH=amd64 GOOS=windows $(cmd) ${winFn}.exe


clean:
	rm -f $(linuxAmdFn)
	rm -f $(linuxArmFn)
	rm -f ${winFn}.exe
	rm -f $(macM1Fn)
