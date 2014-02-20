# static [![wercker status](https://app.wercker.com/status/51fcaab5e7b3879d4aa0462c9d7a2d51/s/ "wercker status")](https://app.wercker.com/project/bykey/51fcaab5e7b3879d4aa0462c9d7a2d51) [![Go Walker](http://gowalker.org/api/v1/badge)](http://gowalker.org/github.com/typepress/static)

static file Handler, support gzip precompression

静态文件输出, 支持 gzip 预压缩

规则
====

 - 文件不存在直接返回, 不产生 NotFound
 - 请求 Method 必须是 GET/HEAD
 - Martini 下请预先把站点根路径用 Map(http.Dir(baseDirOfSite)) 准备好.
 - 如果 baseDirOfSite 为 "" 直接返回
 - 如果 URL.Path 不是目录直接返回
 - 如果 URL.Path 以 "/index.html" 结尾, 301 到 "./"
 - 如果 URL.Path 是目录且不以 "/" 结尾, 301 到 "./"
 - 如果 URL.Path 是目录, 自动查找 index.html 索引
 - 如果 已经设置 "Content-Encoding" 不进行 gzip
 - gzip 预压缩, 扩展名为 .gz, 预置 .css, .html, .js 类型 charset 为 utf-8.

License
=======
BSD-2-Clause