package netLayer

// geosite是v2fly社区维护的，非常有用！本作以及任何其它项目都没必要另起炉灶，
// 需要直接使用v2fly所提供的资料
// geosite数据格式可参考
// https://github.com/v2fly/v2ray-core/blob/master/app/router/routercommon/common.proto
// 或者xray的 app/router/config.proto
// 我们这里就不内嵌 该 proto 文件了，直接复制生成的go文件过来即可

/*
用到的东西

router.Domain_Attribute
router.Domain_RootDomain
router.Domain_Regex
router.Domain_Plain
router.Domain_Full
router.Domain_Attribute_BoolValue
router.Domain_Attribute_IntValue
router.GeoSite
router.GeoSiteList
*/
