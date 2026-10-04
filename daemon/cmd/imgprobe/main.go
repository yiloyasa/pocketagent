package main

// 图片输入探针:验证 codex app-server 是否真的接收 localImage 形状的输入块。
//
// 【测什么】turn/start 的 input 里放 {"type":"localImage","path":<绝对路径>},
// 观察 codex 是否报错、模型回复是否与图片内容相关 —— 拿"codex 真看到图"的证据。
// 同时打印 model/list 里每个模型的 inputModalities(daemon 侧能力门控的数据源)。
//
// 【用法】imgprobe [图片绝对路径] [提问];不传参数时自生成一张已知内容的
// 测试图(与 resume 模式同款),不再依赖仓库里现成的截图文件。
// 纯文本探针见 cmd/turnprobe(结构完全同款:initialize → thread/start → turn/start)。
//
// 【resume 模式】imgprobe resume —— 图片输入的 resume 生命周期定性实验:
//
//	1) 程序自己生成一张"内容只有本探针知道"的 PNG(随机背景色 + 随机数量黑点,
//     防止模型靠上一轮的文字回答"背答案")并落成临时文件(与 daemon 的
//     materializeImage 同为 CreateTemp 临时文件);
//	2) turn 1 发图,要求只回"已收到"、不描述内容(保证后续轮次无法从文字作弊);
//	3) 删除该临时文件 —— 精确模拟 daemon 在 turn 终局 defer cleanup 的行为;
//	4) 同会话 turn 2 问"刚才那张图背景什么颜色、几个黑点"(线程还在同一
//     app-server 内存里,验的是"内存历史是否还依赖原文件");
//	5) 杀掉 app-server 重开,thread/resume 后 turn 3 问同样问题 —— 这才是疑点
//     本体:resume 重放历史时若按路径重读原文件,此处必然 NO_IMAGE;
//	6) 顺带扫描 ~/.codex/sessions 的 rollout 文件,报告图片在该线程里的记录
//     形态(原路径 vs base64 编码),给行为结论一个可核验的文件证据。
import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/fs"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
)

// capturedEvent 一条通知的method + 原始参数(探针按轮次窗口过滤用)。
type capturedEvent struct {
	method string
	params json.RawMessage
}

// eventCollector 通知收集器。readLoop goroutine 会并发调用 add,
// 与主 goroutine 的 snapshot 之间用互斥锁保护(探针也要讲基本法)。
type eventCollector struct {
	mu sync.Mutex
	ev []capturedEvent
}

func (ec *eventCollector) add(method string, params json.RawMessage) {
	ec.mu.Lock()
	ec.ev = append(ec.ev, capturedEvent{method: method, params: params})
	ec.mu.Unlock()
}

func (ec *eventCollector) snapshot() []capturedEvent {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return append([]capturedEvent(nil), ec.ev...)
}

// agentTextsOfTurn 从 [fromIdx, len) 窗口内抽取 agentMessage 正文:
// item/completed(agentMessage)与 turn/completed(turn.items)两个来源。
func (ec *eventCollector) agentTextsOfTurn(fromIdx int) []string {
	var out []string
	for _, e := range ec.snapshot()[fromIdx:] {
		switch {
		case e.method == "item/completed":
			var n struct {
				Item struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"item"`
			}
			if json.Unmarshal(e.params, &n) == nil && n.Item.Type == "agentMessage" && strings.TrimSpace(n.Item.Text) != "" {
				out = append(out, n.Item.Text)
			}
		case e.method == "turn/completed":
			var n struct {
				Turn struct {
					Items []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"items"`
				} `json:"turn"`
			}
			if json.Unmarshal(e.params, &n) == nil {
				for _, it := range n.Turn.Items {
					if it.Type == "agentMessage" && strings.TrimSpace(it.Text) != "" {
						out = append(out, it.Text)
					}
				}
			}
		}
	}
	return out
}

// turnErrorOfTurn 返回窗口内第一条 error 通知的报文(codex 限流等)。
func (ec *eventCollector) turnErrorOfTurn(fromIdx int) string {
	for _, e := range ec.snapshot()[fromIdx:] {
		if e.method == "error" {
			return string(e.params)
		}
		if e.method == "turn/completed" && strings.Contains(string(e.params), `"error"`) {
			return string(e.params)
		}
	}
	return ""
}

// turnDone reports whether a turn/completed arrived in the window.
func (ec *eventCollector) turnDone(fromIdx int) bool {
	for _, e := range ec.snapshot()[fromIdx:] {
		if e.method == "turn/completed" {
			return true
		}
	}
	return false
}

// startProbeServer 拉起一个 app-server 并装上通知收集钩子。
func startProbeServer(ec *eventCollector) (*codexapp.Client, error) {
	c, err := codexapp.Start()
	if err != nil {
		return nil, err
	}
	c.NotifHandler = func(method string, params json.RawMessage) {
		fmt.Printf("  [%s] NOTIF %s\n", time.Now().Format("15:04:05.000"), method)
		ec.add(method, params)
	}
	c.ReqHandler = func(id int64, method string, params json.RawMessage) {
		fmt.Printf("  [%s] REQ(id=%d) %s %.200s\n", time.Now().Format("15:04:05.000"), id, method, string(params))
		_ = c.Respond(id, map[string]string{"decision": "accept"})
	}
	if _, err := c.Call("initialize", map[string]interface{}{"clientInfo": map[string]string{"name": "imgprobe-resume", "version": "0.2"}}); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// runTurn 发起一轮 turn/start 并阻塞到终局;返回(从该轮起点抽取的 agent 文本, 错误报文)。
// 出现 1302(限流)立即原样上报,由调用方决定终止 —— 不硬扛。
func runTurn(c *codexapp.Client, ec *eventCollector, threadID, text string, imgPath string, timeout time.Duration) ([]string, string) {
	from := len(ec.snapshot())
	input := []map[string]string{{"type": "text", "text": text}}
	if imgPath != "" {
		input = append(input, map[string]string{"type": "localImage", "path": imgPath})
	}
	if _, err := c.Call("turn/start", map[string]interface{}{
		"threadId":       threadID,
		"input":          input,
		"approvalPolicy": "never",
		"sandboxPolicy":  map[string]interface{}{"type": "dangerFullAccess"},
	}); err != nil {
		return nil, "turn/start 调用失败: " + err.Error()
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		if msg := ec.turnErrorOfTurn(from); msg != "" && strings.Contains(msg, "1302") {
			// 限流:立即上报,不硬扛
			return ec.agentTextsOfTurn(from), "限流(1302): " + clipStr(msg, 300)
		}
		if ec.turnDone(from) {
			return ec.agentTextsOfTurn(from), ""
		}
	}
	return ec.agentTextsOfTurn(from), "等待 turn/completed 超时(" + timeout.String() + ")"
}

// probeImage 是一张内容只有探针自己知道的测试图。
type probeImage struct {
	bgName string // 背景色的中文名(正确答案的一部分)
	dots   int    // 黑点数量(正确答案的另一部分)
	path   string // 落盘的临时文件路径
}

// makeProbeImage 生成测试 PNG 并落成临时文件(与 daemon 的 materializeImage
// 同为 os.CreateTemp 产物,文件名模式也对齐)。
func makeProbeImage() (*probeImage, error) {
	bgChoices := []struct {
		name string
		c    color.RGBA
	}{
		{"红色", color.RGBA{214, 48, 49, 255}},
		{"绿色", color.RGBA{39, 174, 96, 255}},
		{"蓝色", color.RGBA{41, 128, 185, 255}},
		{"黄色", color.RGBA{241, 196, 15, 255}},
	}
	bg := bgChoices[rand.Intn(len(bgChoices))]
	dots := 3 + rand.Intn(4) // 3~6 个,一眼可数

	const w, h = 320, 200
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg.c}, image.Point{}, draw.Src)
	// 一行黑色实心圆点,水平居中
	r, gap := 20, 60
	x0 := (w-dots*gap)/2 + r
	cy := h / 2
	black := color.RGBA{10, 10, 10, 255}
	for i := 0; i < dots; i++ {
		cx := x0 + i*gap
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if dx*dx+dy*dy <= r*r {
					img.Set(cx+dx, cy+dy, black)
				}
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp("", "pocketagent-img-resume-*.png")
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return nil, err
	}
	return &probeImage{bgName: bg.name, dots: dots, path: f.Name()}, nil
}

// inspectRollout 在 ~/.codex/sessions 里找包含该 threadId 的 rollout 文件,
// 报告图片在历史里的记录形态:原路径引用 vs base64 内联编码。
func inspectRollout(threadID, imgBase string) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("ROLLOUT 无法定位用户目录:", err)
		return
	}
	root := filepath.Join(home, ".codex", "sessions")
	var rolloutPath string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".jsonl") {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		if strings.Contains(string(raw), threadID) {
			rolloutPath = p
			return fs.SkipAll
		}
		return nil
	})
	if rolloutPath == "" {
		fmt.Println("ROLLOUT 未找到包含线程的 rollout 文件(可能未落盘)")
		return
	}
	fmt.Println("ROLLOUT 文件:", rolloutPath)
	raw, err := os.ReadFile(rolloutPath)
	if err != nil {
		fmt.Println("ROLLOUT 读取失败:", err)
		return
	}
	pathHits, b64Hits, localImageLines := 0, 0, 0
	var sample string
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" {
			continue
		}
		isLI := strings.Contains(line, "localImage")
		hasPath := imgBase != "" && strings.Contains(line, imgBase)
		hasB64 := strings.Contains(line, "base64,") || strings.Contains(line, "data:image")
		if isLI {
			localImageLines++
		}
		if hasPath {
			pathHits++
			if sample == "" {
				sample = clipStr(line, 400)
			}
		}
		if hasB64 {
			b64Hits++
		}
	}
	fmt.Printf("ROLLOUT 统计: localImage 记录=%d 条, 含原文件名路径=%d 条, 含 base64/dataURL=%d 条\n",
		localImageLines, pathHits, b64Hits)
	if sample != "" {
		fmt.Println("ROLLOUT 含路径行摘录:", sample)
	}
}

func clipStr(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// runResumeProbe 执行 resume 生命周期定性实验(流程见文件头【resume 模式】)。
func runResumeProbe() {
	img, err := makeProbeImage()
	if err != nil {
		log.Fatal("生成测试图片:", err)
	}
	defer os.Remove(img.path) // 兜底:即使中途 log.Fatal 退出也尽量不剩垃圾(进程退出本就回收临时目录)
	fmt.Printf("测试图片: %s (背景=%s, 黑点=%d)\n", img.path, img.bgName, img.dots)

	// ---- 阶段 1:发图(turn 1)----
	ec1 := &eventCollector{}
	c1, err := startProbeServer(ec1)
	if err != nil {
		log.Fatal("启动 app-server:", err)
	}
	tr, err := c1.Call("thread/start", map[string]interface{}{"cwd": os.TempDir()})
	if err != nil {
		c1.Close()
		log.Fatal("thread/start:", err)
	}
	var ts struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	_ = json.Unmarshal(tr, &ts)
	threadID := ts.Thread.ID
	fmt.Println("thread id:", threadID)

	fmt.Println("---- turn 1: 发图,要求只回「已收到」----")
	texts, errMsg := runTurn(c1, ec1, threadID, "收到图片后不要描述内容,只回复:已收到。", img.path, 240*time.Second)
	fmt.Println("turn1 回复:", strings.Join(texts, " | "), "错误:", errMsg)
	if errMsg != "" {
		if strings.Contains(errMsg, "1302") {
			fmt.Println("== 探测终止:命中限流 1302 ==")
			c1.Close()
			os.Exit(2)
		}
		fmt.Println("== 探测终止:turn 1 未正常完成 ==")
		c1.Close()
		os.Exit(2)
	}

	// ---- 阶段 2:删临时文件(精确模拟 daemon turn 终局 defer cleanup)----
	if err := os.Remove(img.path); err != nil && !os.IsNotExist(err) {
		log.Fatal("删除临时图片失败:", err)
	}
	if _, err := os.Stat(img.path); !os.IsNotExist(err) {
		log.Fatal("临时图片删除后仍存在?")
	}
	fmt.Println("---- 临时图片已删除(模拟 daemon turn 终局清理)----")

	// ---- 阶段 3:同会话追问(turn 2)----
	q := "我们刚才那张图片的背景是什么颜色?里面有几个黑色圆点?如果你实际看不到图片内容,请只回答 NO_IMAGE。"
	fmt.Println("---- turn 2: 同会话追问(原文件已删)----")
	texts2, errMsg2 := runTurn(c1, ec1, threadID, q, "", 240*time.Second)
	ans2 := strings.Join(texts2, " | ")
	fmt.Println("turn2 回复:", ans2, "错误:", errMsg2)

	// ---- 阶段 4:重启 app-server + thread/resume 后追问(turn 3,疑点本体)----
	c1.Close()
	fmt.Println("---- app-server 已重启,thread/resume 后追问 ----")
	ec2 := &eventCollector{}
	c2, err := startProbeServer(ec2)
	if err != nil {
		log.Fatal("重启 app-server:", err)
	}
	defer c2.Close()
	if _, err := c2.Call("thread/resume", map[string]interface{}{"threadId": threadID}); err != nil {
		log.Fatal("thread/resume:", err)
	}
	texts3, errMsg3 := runTurn(c2, ec2, threadID, q, "", 240*time.Second)
	ans3 := strings.Join(texts3, " | ")
	fmt.Println("turn3 回复:", ans3, "错误:", errMsg3)

	// ---- 阶段 5:rollout 文件证据 ----
	inspectRollout(threadID, filepath.Base(img.path))

	// ---- 定性 ----
	seen := func(ans string) bool {
		return ans != "" && !strings.Contains(ans, "NO_IMAGE") &&
			(strings.Contains(ans, img.bgName) || strings.Contains(ans, fmt.Sprint(img.dots)))
	}
	fmt.Println("==== 定性 ====")
	fmt.Printf("正确答案: 背景=%s 黑点=%d\n", img.bgName, img.dots)
	fmt.Printf("同会话(turn2): %v → %s\n", seen(ans2), verdictOf(seen(ans2)))
	fmt.Printf("resume 后(turn3): %v → %s\n", seen(ans3), verdictOf(seen(ans3)))
}

func verdictOf(seen bool) string {
	if seen {
		return "图片内容可见"
	}
	return "图片内容失效(NO_IMAGE/答错)"
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "resume" {
		rand.Seed(time.Now().UnixNano())
		runResumeProbe()
		return
	}

	// 缺省图片:探针自生成一张已知内容的测试图(makeProbeImage,与 resume
	// 模式同款)。不写死任何个人专属绝对路径 —— 那会随源码/二进制分发,且
	// 换台机器必然不存在;显式传参(os.Args[1])仍是第一优先。
	question := "这张截图里是什么内容?一句话描述你看到的主要东西。如果你实际看不到图片,请只回答 NO_IMAGE。"
	imgPath := ""
	if len(os.Args) > 1 {
		imgPath = os.Args[1]
	}
	if len(os.Args) > 2 {
		question = os.Args[2]
	}
	if imgPath == "" {
		rand.Seed(time.Now().UnixNano())
		img, err := makeProbeImage()
		if err != nil {
			log.Fatal("生成缺省测试图片:", err)
		}
		defer os.Remove(img.path)
		imgPath = img.path
	}
	if _, err := os.Stat(imgPath); err != nil {
		log.Fatal("图片不存在:", err)
	}

	c, err := codexapp.Start()
	if err != nil {
		log.Fatal("spawn app-server:", err)
	}
	defer c.Close()

	var events []string
	c.NotifHandler = func(method string, params json.RawMessage) {
		line := fmt.Sprintf("[%s] NOTIF %s %.400s", time.Now().Format("15:04:05.000"), method, string(params))
		fmt.Println(line)
		events = append(events, line)
	}
	c.ReqHandler = func(id int64, method string, params json.RawMessage) {
		fmt.Printf("[%s] REQ(id=%d) %s %.200s\n", time.Now().Format("15:04:05.000"), id, method, string(params))
		_ = c.Respond(id, map[string]string{"decision": "accept"})
	}

	if _, err := c.Call("initialize", map[string]interface{}{"clientInfo": map[string]string{"name": "imgprobe", "version": "0.1"}}); err != nil {
		log.Fatal("initialize:", err)
	}

	// 1) 模型列表与 inputModalities(能力门控的数据源,codex 实测为准)
	raw, err := c.Call("model/list", map[string]interface{}{})
	if err != nil {
		log.Println("model/list 失败(继续):", err)
	} else {
		var ml struct {
			Data []struct {
				Id              string   `json:"id"`
				DisplayName     string   `json:"displayName"`
				InputModalities []string `json:"inputModalities"`
				IsDefault       bool     `json:"isDefault"`
			} `json:"data"`
		}
		if json.Unmarshal(raw, &ml) == nil {
			for _, m := range ml.Data {
				fmt.Printf("MODEL %-28s modalities=%v default=%v\n", m.Id, m.InputModalities, m.IsDefault)
			}
		}
	}

	// 2) 开线程 + 发图(cwd 用系统临时目录,避开中文路径等无关变量)
	tr, err := c.Call("thread/start", map[string]interface{}{"cwd": os.TempDir()})
	if err != nil {
		log.Fatal("thread/start:", err)
	}
	var ts struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	_ = json.Unmarshal(tr, &ts)
	fmt.Println("thread id:", ts.Thread.ID)

	start := time.Now()
	if _, err := c.Call("turn/start", map[string]interface{}{
		"threadId": ts.Thread.ID,
		"input": []map[string]string{
			{"type": "text", "text": question},
			{"type": "localImage", "path": imgPath},
		},
		"approvalPolicy": "never",
		"sandboxPolicy":  map[string]interface{}{"type": "dangerFullAccess"},
	}); err != nil {
		log.Fatal("turn/start:", err)
	}

	deadline := time.Now().Add(180 * time.Second)
	done := false
	for time.Now().Before(deadline) && !done {
		time.Sleep(500 * time.Millisecond)
		for _, e := range events {
			if strings.Contains(e, "NOTIF turn/completed") || strings.Contains(e, "NOTIF error") {
				done = true
			}
		}
	}

	// 3) 把 agentMessage 完整捞出来(判断"回复是否与图相关"的证据)
	for _, raw := range events {
		if !strings.Contains(raw, "agentMessage") {
			continue
		}
		var n struct {
			Item struct {
				Text string `json:"text"`
			} `json:"item"`
		}
		if json.Unmarshal([]byte(raw[strings.Index(raw, "{"):]), &n) == nil && strings.TrimSpace(n.Item.Text) != "" {
			fmt.Println("== AGENT ==", n.Item.Text)
		}
	}
	fmt.Printf("== 探测结束,共 %d 条事件,用时 %v ==\n", len(events), time.Since(start).Round(time.Millisecond))
}
