/*
package configAdapter provides methods to convert proxy.ListenConf and proxy.DialConf to some 3rd party formats.

对于第三方工具的配置, 支持 quantumultX, clash, 以及 v2rayN 的配置格式

参考 https://github.com/e1732a364fed/v2ray_simple/discussions/163

以及 docs/url.md

本包依然秉持KISS原则，用最笨的代码、最少的依赖，实现最小的可执行文件大小以及最快的执行速度。
*/
package configAdapter
