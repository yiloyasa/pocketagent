// Package testkit 收编各 *app / 基础包测试里**逐字同款**的平台小件:
// 轮询等待子进程退出、轮询等待 fake 子进程产出文件、劫持标准 log 抓输出、
// 断言本包源码不含开发机专属路径。
//
// 【为什么收编】这些 helper 在 codexapp/zcodeapp(平台分支各一份)、codexapp/
// piapp、eventstore/term、dshapp/piapp 里各抄一份,正文与错误文案逐字相同
// (有的文件注释甚至互相点名"与 xx 同款")。它们与被测包的协议毫无关系,纯属
// 测试基建,收编到本包后"改一处、处处生效",平台分支(Windows/非 Windows)
// 也只需维护一份。
//
// 【等价性】收编时语句与错误文案逐字搬运,断言一字未改;唯一增补是
// AssertNoDeveloperPaths 里的 t.Helper()(只把失败行号指回调用点,不改判定条件)。
// 调用方仍在各自包的测试里调用(用例集合与断言集合不变)。本包只应被 *_test.go
// 引用 —— 生产代码不得 import(否则 testing 会被链进 daemon 二进制)。
package testkit
