package main

// codexturnsprobe 探针:0.156.1 新增只读方法的**运行时形状**实测
// (thread/turns/list、thread/items/list、thread/search),为"是否值得用它们
// 替换 thread/read 全量拉取/提供搜索"的评估提供分级 A 证据。
//
// 【全部零 LLM 成本】三个方法都是只读;线程取自 thread/list 的存量会话,
// 不 thread/start、不 turn/start,不写任何配置(红线与 codexreadprobe 同款)。
//
// 【背景证据链】
//  - 0.156.1 运行时接受这三个方法(校准探测 -32600 的合法方法清单,
//    docs/verify/evidence/codex-0156-probe.log 证据分级 A);
//  - 二进制静态串:`struct ThreadTurnsListParams with 5 elements`、
//    "omit includeTurns or set it to false, then page with thread/turns/list
//    and thread/items/list"、TurnsPage(vendored 0.155.1 schema 已有 unreferenced
//    定义 {data, nextCursor, backwardsCursor})—— 参数名待本探针定形。
import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
)

func main() {
	c, err := codexapp.Start()
	if err != nil {
		log.Fatal("spawn app-server:", err)
	}
	defer c.Close()

	initCtx, initCancel := context.WithTimeout(context.Background(), 60*time.Second)
	_, err = c.CallCtx(initCtx, "initialize", map[string]interface{}{
		"clientInfo": map[string]string{"name": "codexturnsprobe", "version": "0.1"},
	})
	initCancel()
	if err != nil {
		log.Fatal("initialize:", err)
	}
	fmt.Println("=== initialize OK ===")

	call := func(tag, method string, params interface{}) json.RawMessage {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		raw, err := c.CallCtx(ctx, method, params)
		if err != nil {
			fmt.Printf("=== %s ERROR: %v\n", tag, err)
			return nil
		}
		s := string(raw)
		// 按 rune 截断:字节截断会把 UTF-8 多字节字符切半,证据文件就废了
		if len([]rune(s)) > 500 {
			s = string([]rune(s)[:500]) + "…(截断)"
		}
		fmt.Printf("=== %s OK: %s\n", tag, s)
		return raw
	}

	// ---- 0) 选一条存量线程(有轮次历史的老会话最理想)----
	rawList := call("thread/list", "thread/list", map[string]interface{}{"limit": 5})
	var tl struct {
		Data []struct {
			Id      string `json:"id"`
			Name    string `json:"name"`
			Preview string `json:"preview"`
		} `json:"data"`
		NextCursor string `json:"nextCursor"`
	}
	if err := json.Unmarshal(rawList, &tl); err != nil || len(tl.Data) == 0 {
		log.Fatalf("thread/list 拿不到存量线程(本机无 codex 会话?): err=%v", err)
	}
	tid := tl.Data[0].Id
	fmt.Printf("=== 选定线程 tid=%s name=%q\n", tid, tl.Data[0].Name)

	// ---- 1) thread/turns/list:缺参报错暴露必填键;空对象探形状 ----
	call("turns/list(空参数)", "thread/turns/list", map[string]interface{}{})
	call("turns/list(未知键)", "thread/turns/list", map[string]interface{}{"threadId": tid, "bogus": 1})

	// ---- 2) thread/turns/list:threadId 一发(观察应答骨架与游标)----
	rawTurns := call("turns/list(threadId)", "thread/turns/list", map[string]interface{}{"threadId": tid})
	var tp struct {
		Data []struct {
			Id     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
		NextCursor      string `json:"nextCursor"`
		BackwardsCursor string `json:"backwardsCursor"`
	}
	_ = json.Unmarshal(rawTurns, &tp)
	fmt.Printf("=== turns/list 解析: %d turns, nextCursor=%q backwardsCursor=%q\n",
		len(tp.Data), tp.NextCursor, tp.BackwardsCursor)
	if tp.NextCursor != "" {
		call("turns/list(翻页)", "thread/turns/list", map[string]interface{}{
			"threadId": tid, "cursor": tp.NextCursor,
		})
	}

	// ---- 3) thread/items/list:缺参 + 带 turnId ----
	call("items/list(空参数)", "thread/items/list", map[string]interface{}{})
	if len(tp.Data) > 0 {
		call("items/list(threadId+turnId)", "thread/items/list", map[string]interface{}{
			"threadId": tid, "turnId": tp.Data[0].Id,
		})
	} else {
		call("items/list(threadId only)", "thread/items/list", map[string]interface{}{
			"threadId": tid,
		})
	}

	// ---- 4) thread/search:缺参 + 常见词 ----
	call("search(空参数)", "thread/search", map[string]interface{}{})
	call("search(searchTerm=探针)", "thread/search", map[string]interface{}{"searchTerm": "探针"})
	call("search(searchTerm=PONG)", "thread/search", map[string]interface{}{"searchTerm": "PONG"})
	call("search(searchTerm=探针,threadId)", "thread/search", map[string]interface{}{"searchTerm": "遗留", "threadId": tid})

	// ---- 5) 对照组:thread/read 不带 turns(应为轻快照)----
	call("thread/read(includeTurns=false)", "thread/read", map[string]interface{}{
		"threadId": tid, "includeTurns": false,
	})

	fmt.Println("=== 探针结束(全程只读,零 LLM 成本)===")
}
