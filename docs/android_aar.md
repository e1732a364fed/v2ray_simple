
若要生成用于安卓app的aar，见 machine/genaar.sh

其用了 androidAAR 这个 build tag 来使用 machine/forAndroid.go文件

debian上，先运行 

```
sudo apt install android-sdk
export ANDROID_HOME=/usr/lib/android-sdk
```

安装ndk
https://developer.android.com/studio/projects/install-ndk

ubuntu22安装 sdkmanager: apt install sdkmanager

其他平台： https://developer.android.com/studio#command-tools 下载安装

安卓的aar可用于vsb项目