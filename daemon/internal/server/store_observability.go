// store_observability.go —— 会话持久化(threads.json 落盘)失败的观测埋点。
//
// 【背景(体检 P2-4 / T-13)】server 包里有一批"尽力而为"的 Store.Update 调用:
// 内存态已改,磁盘写失败(saveLocked 的 WriteFile/Rename 失败)时把错误丢弃。
// 磁盘异常期间这些变化静默失效,重启后"改了又变回去",且日志里没有任何线索。
// 本助手给这些点位统一埋日志:err == nil(成功)不打日志,正常路径零开销;
// 非 nil 打一条带操作名与线程 ID 的错误行,排障时可直接定位是哪条链路写丢了。
//
// 【为什么只加日志不返回错误】这些点位多在启动迁移/provider 回调/流式循环里,
// 没有把错误回传给调用方的通道;改签名会扩散到 agent 回调乃至协议层,超出
// 本批"只加观测"的范围。section-clear 已升级返回错误(T-17);其余点位维持
// 日志口径(T-24 更新,原"见 T-13 汇报的建议清单"已消化完毕)。
package server

import (
	"fmt"
	"log"
)

// storeUpdateErrMsg 生成落盘失败日志行(纯函数,格式由单测钉住)。
func storeUpdateErrMsg(op, threadID string, err error) string {
	return fmt.Sprintf("[store] 更新落盘失败 op=%s thread=%s err=%v", op, threadID, err)
}

// logStoreUpdateErr 记录一次失败的持久化;err == nil(成功)不打日志。
//
// 用 log.Print 而不是 Printf:err 消息里可能带 % 等格式符,预格式化后再
// 原样输出,避免被二次解释。
func logStoreUpdateErr(op, threadID string, err error) {
	if err == nil {
		return
	}
	log.Print(storeUpdateErrMsg(op, threadID, err))
}
