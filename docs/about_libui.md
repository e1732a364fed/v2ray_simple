
The project use a customized golang binding project for libui-ng:
github.com/e1732a364fed/ui

because libui-ng has a few bugs, we need to fix it before put it into use.

本作使用一个自定义的 libui-ng 的 golang 绑定版本。

因为 libui-ng有很多问题（一些来自老项目），我们必须用前修好。

问题：
1. macos上闪退
2. windows上卡顿
3. 不支持图片显示
4. 不支持table的行高调节


我们修复1、2问题


compile libui-ng/libui-ng for darwin build

```sh
git clone https://github.com/libui-ng/libui-ng.git
cd libui-ng
git revert f4d89db386ed882bec8a03d2c5e572f99aeaa800
meson setup build --buildtype=release --default-library=static
ninja -C build

sudo mv build/meson-out/libui.a ~/go/pkg/mod/github.com/e1732a364fed/ui@v0.0.1-alpha.7/libui_darwin_arm64.a
```

see https://github.com/libui-ng/libui-ng/issues/160

for windows, use a script I provided in 
https://github.com/libui-ng/libui-ng/issues/161

