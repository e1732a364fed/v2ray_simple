
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

