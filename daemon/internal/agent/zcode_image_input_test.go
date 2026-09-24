// zcode_image_input_test.go —— 手机图片附件真正进入 zcode session/send 的链路测试。
//
// 【形状依据】2026-09-19 live 实测(cmd/zcprobe -image):attachments 元素
// {kind:"image", filename, mimeType, sizeBytes, localPath:<绝对路径>} 被服务器
// 归一(反混淆 MEs)后读文件转 data URL,收图模型真实答出"红色";模型层对
// inputFormat.supportsImage=false 的模型直接抛 "Model does not support image
// input",所以门控不过必须降级为文本占位(与 codex 侧降级语义对齐)。
//
// 【fake 怎么接】zcodeapp.LocateZCode 尊重 ZCODE_NODE/ZCODE_CJS:把 ZCODE_CJS
// 指向测试现场生成的假 app-server 脚本(node 运行),按 ZCode Protocol(NDJSON,
// 无 jsonrpc 字段)应答,并把收到的每个请求记进 PA_FAKE_ZCODE_LOG 供父进程断言;
// PA_FAKE_ZCODE_SNAPSHOT 可注入 session/create 快照(门控降级路径换 current 模型
// 的 inputFormat 用)。本机没有 node 时跳过(fake 子进程需要 node 拉起)。
//
// 【与 codex fake 的分工】codex_effect_test.go 的 TestMain/helper 是 codex 协议
// 形状(有 initialize 握手、thread/start),复用不了 —— zcode 这里用独立脚本,
// 不动那份文件。
package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// zcodeTinyPNG 一张真实的 1x1 PNG(让 fake 链路里的图片字节非空;fake 不解析内容)。
var zcodeTinyPNG, _ = base64.StdEncoding.DecodeString(
	"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")

// zcodeImageTurnRequest 造一个"文本 + 一张图"的 TurnRequest(对齐手机附件形状)。
func zcodeImageTurnRequest(thread *store.Thread, ws string) TurnRequest {
	return TurnRequest{
		Thread:    thread,
		Workspace: ws,
		InputBlocks: []pb.ContentBlock{
			{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "你好,看这张图"}}},
			{Block: &pb.ContentBlock_Image{Image: &pb.ImageBlock{Data: zcodeTinyPNG, MediaType: "image/png"}}},
		},
	}
}

// runZcodeImageTurn 跑一轮自定义请求并收集推给手机的消息。
func runZcodeImageTurn(t *testing.T, p *ZcodeProvider, req TurnRequest) []*pb.TurnServerMessage {
	t.Helper()
	var mu sync.Mutex
	var got []*pb.TurnServerMessage
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, _, err := p.Run(ctx, req, func(m *pb.TurnServerMessage) {
		mu.Lock()
		got = append(got, m)
		mu.Unlock()
	}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return got
}

// fakeZcodeServerScript 假 zcode app-server(node 运行)。
// 【形状纪律】应答帧 {id,result}、通知帧 {method,params}(协议无 jsonrpc 字段,
// zcprobe 第一轮实证);session/create 回快照;session/send 先应答再推
// turn.started → model.streaming(text_delta)→ turn.completed。
const fakeZcodeServerScript = `// 假 ZCode app-server(测试用):请求记日志,按 ZCode Protocol NDJSON 应答。
const fs = require('fs');
const logPath = process.env.PA_FAKE_ZCODE_LOG || '';
const snapshotRaw = process.env.PA_FAKE_ZCODE_SNAPSHOT || '';
let snapshot = null;
try { snapshot = JSON.parse(snapshotRaw); } catch (e) {}
if (!snapshot) {
  snapshot = {session:{sessionId:'sess_fake'}, settings:{model:{
    current:{providerId:'p1', modelId:'MiniMax-M3'},
    available:[{ref:{providerId:'p1', modelId:'MiniMax-M3'}, label:'MiniMax-M3',
      contextWindow:1000000, reasoning:{levels:[], defaultLevel:'enabled'},
      properties:{inputFormat:{supportsText:true, supportsImage:true,
        supportsVideo:false, supportsAudio:false, supportsPdf:false}}}]}}}};
let seq = 0;
function write(obj) { process.stdout.write(JSON.stringify(obj) + '\n'); }
function logReq(method, params) {
  if (!logPath) return;
  try { fs.appendFileSync(logPath, JSON.stringify({method, params}) + '\n'); } catch (e) {}
}
let buf = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', (chunk) => {
  buf += chunk;
  let idx;
  while ((idx = buf.indexOf('\n')) >= 0) {
    const line = buf.slice(0, idx);
    buf = buf.slice(idx + 1);
    if (!line.trim()) continue;
    let m;
    try { m = JSON.parse(line); } catch (e) { continue; }
    if (!m.method) continue;
    logReq(m.method, m.params);
    if (m.id === undefined || m.id === null) continue;
    const sid = (m.params && m.params.sessionId) || 'sess_fake';
    if (m.method === 'provider/updateAccountConfig') {
      write({id:m.id,result:{receivedRevision:m.params.revision}});
      continue;
    }
    if (m.method === 'session/resume') {
      write({id:m.id,result:snapshot});
      continue;
    }
    if (m.method === 'session/setMode') {
      if (!m.params.sessionId) {
        write({id:m.id,error:{code:-32602,message:'Invalid params — sessionId: missing; mode: Invalid option: expected one of "plan"|"build"|"edit"|"yolo"|"auto"'}});
      } else {
        snapshot.settings.mode={current:m.params.mode==='plan'?'build':m.params.mode};
        write({id:m.id,result:snapshot});
      }
      continue;
    }
    if (m.method === 'session/close') {
      const code = process.env.PA_FAKE_ZCODE_CLOSE_ERROR;
      write(code ? {id:m.id,error:{code:Number(code),message:'native-close-error'}} : {id:m.id,result:{closed:true}});
      continue;
    }
    if (m.method === 'session/create') {
      write({id: m.id, result: snapshot});
      continue;
    }
    if (m.method === 'session/send') {
      write({id: m.id, result: {}});
      const env = (type, payload) => write({method: 'session/event', params: {
        sessionId: sid, turnId: 'turn_fake', eventId: 'e' + (++seq), seq: seq,
        deliveryKind: 'desktop-continuous', type, payload}});
      env('turn.started', {turnNumber: 0});
      if (process.env.PA_FAKE_ZCODE_STATE_UPDATE) {
        write({method:'state.updated',params:{sessionId:sid,scope:'session',patch:{model:{current:{providerId:'p1',modelId:'native-switched',options:{reasoningLevel:'future-level'}}}}}});
        write({method:'future.unknown.notification',params:{sessionId:sid,newField:true}});
      }
      if (process.env.PA_FAKE_ZCODE_EVENTS) {
        // 注入任意 session/event 序列(标题同步等事件形状的接线测试用);
        // 元素形状 {type, payload},payload 缺省给 {}。
        for (const e of JSON.parse(process.env.PA_FAKE_ZCODE_EVENTS)) {
          env(e.type, e.payload || {});
        }
      }
      env('model.streaming', {kind: 'text_delta', delta: 'fake-ok', done: false});
      env('turn.completed', {response: 'fake-ok', resultType: 'success',
        usage: {inputTokens: 1, outputTokens: 1, totalTokens: 2,
          cacheReadTokens: 0, cacheWriteTokens: 0, reasoningTokens: 0}});
      continue;
    }
    write({id: m.id, result: {}});
  }
});
`

// newFakeZcodeProvider 造一个"zcode app-server"实际是 node 跑假脚本的 provider,
// 返回请求日志路径;snapshot 注入非空时用它替换默认快照。
func newFakeZcodeProvider(t *testing.T, snapshot string) (*ZcodeProvider, string) {
	t.Helper()
	// 只依赖 node,不要求装着 ZCode 桌面端(与 LocateZCode 的默认探测解耦)。
	node := os.Getenv("ZCODE_NODE")
	if node == "" {
		node = filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "nodejs", "node.exe")
	}
	if _, err := os.Stat(node); err != nil {
		t.Skipf("本机没有 node(%s),跳过 zcode 图片 fake 端到端", node)
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "rpc.log")
	scriptPath := filepath.Join(dir, "fake-zcode-app-server.cjs")
	if err := os.WriteFile(scriptPath, []byte(fakeZcodeServerScript), 0o644); err != nil {
		t.Fatalf("写 fake app-server 脚本: %v", err)
	}
	t.Setenv("ZCODE_NODE", node)
	t.Setenv("ZCODE_CJS", scriptPath)
	t.Setenv("ZCODE_DESKTOP_HOME_DIR", dir) // Never read the user's credentials in a fake test.
	fixtureConfig := filepath.Join(dir, "provider.json")
	if err := os.WriteFile(fixtureConfig, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZCODE_BUILTIN_PROVIDER_CONFIG_FILE", fixtureConfig)
	t.Setenv("ZCODE_PERSONAL_PROVIDER_CONFIG_FILE", fixtureConfig)
	t.Setenv("PA_FAKE_ZCODE_LOG", logPath)
	if snapshot != "" {
		t.Setenv("PA_FAKE_ZCODE_SNAPSHOT", snapshot)
	}
	p := NewZcodeProvider()
	t.Cleanup(func() {
		if p.client != nil {
			p.client.Close()
		}
		// Windows 上子进程退出才释放日志句柄:等到日志可删再返回(与 codex fake 同款)。
		for i := 0; i < 100; i++ {
			if err := os.Remove(logPath); err == nil || os.IsNotExist(err) {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	})
	return p, logPath
}

// zcodeFakeRequests 读回父进程断言所需的全部已记录请求。
func zcodeFakeRequests(t *testing.T, logPath string) []fakeReq {
	t.Helper()
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读 fake zcode 日志: %v", err)
	}
	var out []fakeReq
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var r fakeReq
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("日志行无法解析: %v(%s)", err, line)
		}
		out = append(out, r)
	}
	return out
}

// findZcodeSendParams 取第一条 session/send 的 params(断言为 JSON 对象)。
func findZcodeSendParams(t *testing.T, reqs []fakeReq) map[string]interface{} {
	t.Helper()
	for _, r := range reqs {
		if r.Method != "session/send" {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal(r.Params, &m); err != nil {
			t.Fatalf("session/send params 解析: %v", err)
		}
		return m
	}
	t.Fatalf("fake zcode 没有收到 session/send")
	return nil
}

// TestZcodeImageInputReachesSessionSendAsAttachment 端到端:带图输入必须让
// session/send 带上实测形状的 attachments(kind/filename/mimeType/sizeBytes/
// localPath 绝对路径),content 原样;轮次结束后临时图片已被清理。
func TestZcodeImageInputReachesSessionSendAsAttachment(t *testing.T) {
	p, logPath := newFakeZcodeProvider(t, "")
	ws := t.TempDir()
	runZcodeImageTurn(t, p, zcodeImageTurnRequest(&store.Thread{ID: "daemon-zimg"}, ws))

	send := findZcodeSendParams(t, zcodeFakeRequests(t, logPath))
	if send["content"] != "你好,看这张图" {
		t.Fatalf("content 应为原样文本(零裁剪), got=%v", send["content"])
	}
	raw, ok := send["attachments"].([]interface{})
	if !ok || len(raw) != 1 {
		t.Fatalf("attachments 应为 1 个元素, got=%v", send["attachments"])
	}
	att, _ := raw[0].(map[string]interface{})
	if att["kind"] != "image" || att["mimeType"] != "image/png" {
		t.Fatalf("attachments[0] kind/mimeType 错误: %v", att)
	}
	path, _ := att["localPath"].(string)
	if !filepath.IsAbs(path) {
		t.Fatalf("localPath 应为本机绝对路径, got=%q", path)
	}
	if filepath.Ext(path) != ".png" {
		t.Fatalf("localPath 扩展名应按 MIME 推断为 .png, got=%q", path)
	}
	if size, _ := att["sizeBytes"].(float64); int(size) != len(zcodeTinyPNG) {
		t.Fatalf("sizeBytes 应与原图字节一致: got=%v want=%d", att["sizeBytes"], len(zcodeTinyPNG))
	}
	if name, _ := att["filename"].(string); !strings.HasSuffix(name, ".png") {
		t.Fatalf("filename 应带 .png 扩展名, got=%q", name)
	}
	// 轮次已结束,临时图片应已被清理(defer cleanup 在 Run 返回前执行)。
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("轮次结束后临时图片应被删除, path=%q err=%v", path, err)
	}
}

// TestZcodeImageInputDegradesForTextOnlyModel 能力门控降级(current 模型快照明确
// supportsImage=false,如 GLM 系):不发 attachments,content 追加 [图片:…] 占位。
func TestZcodeImageInputDegradesForTextOnlyModel(t *testing.T) {
	snap := `{"session":{"sessionId":"sess_fake"},"settings":{"model":{
		"current":{"providerId":"p2","modelId":"glm-5.2"},
		"available":[{"ref":{"providerId":"p2","modelId":"glm-5.2"},"label":"GLM-5.2",
		  "contextWindow":1000000,"reasoning":{"levels":[],"defaultLevel":"max"},
		  "properties":{"inputFormat":{"supportsText":true,"supportsImage":false}}}]}}}`
	p, logPath := newFakeZcodeProvider(t, snap)
	ws := t.TempDir()
	runZcodeImageTurn(t, p, zcodeImageTurnRequest(&store.Thread{ID: "daemon-zimg-glm"}, ws))

	send := findZcodeSendParams(t, zcodeFakeRequests(t, logPath))
	if _, has := send["attachments"]; has {
		t.Fatalf("纯文本模型不应带 attachments, got=%v", send["attachments"])
	}
	content, _ := send["content"].(string)
	if !strings.HasPrefix(content, "你好,看这张图") || !strings.Contains(content, "[图片:image-1.png]") {
		t.Fatalf("content 应为原样文本 + [图片:…] 占位, got=%q", content)
	}
}

// TestZcodeImageInputKeptWhenThreadModelDeclaresImage 门控正例 + 解析顺序:快照
// current 不收图,但线程显式指定的模型收图 → 按线程模型放行(与 applyModel 的
// "线程显式优先"语义对齐)。
func TestZcodeImageInputKeptWhenThreadModelDeclaresImage(t *testing.T) {
	snap := `{"session":{"sessionId":"sess_fake"},"settings":{"model":{
		"current":{"providerId":"p2","modelId":"glm-5.2"},
		"available":[
		  {"ref":{"providerId":"p2","modelId":"glm-5.2"},"label":"GLM-5.2",
		   "reasoning":{"defaultLevel":"max"},
		   "properties":{"inputFormat":{"supportsText":true,"supportsImage":false}}},
		  {"ref":{"providerId":"p1","modelId":"MiniMax-M3"},"label":"MiniMax-M3",
		   "reasoning":{"defaultLevel":"enabled"},
		   "properties":{"inputFormat":{"supportsText":true,"supportsImage":true}}}]}}}`
	p, logPath := newFakeZcodeProvider(t, snap)
	ws := t.TempDir()
	runZcodeImageTurn(t, p, zcodeImageTurnRequest(&store.Thread{ID: "daemon-zimg-mm", ModelID: "zcode/MiniMax-M3"}, ws))

	send := findZcodeSendParams(t, zcodeFakeRequests(t, logPath))
	raw, ok := send["attachments"].([]interface{})
	if !ok || len(raw) != 1 {
		t.Fatalf("线程指定收图模型应放行 attachments, got=%v", send["attachments"])
	}
}

// TestZcodeImageTargetModel 纯函数级:生效模型的解析优先级
// (线程显式 > 快照 current > 快照 lastUsed > "")。
func TestZcodeImageTargetModel(t *testing.T) {
	snap := json.RawMessage(`{"settings":{"model":{
		"current":{"providerId":"p1","modelId":"cur-model"},
		"lastUsed":{"providerId":"p0","modelId":"used-model"}}}}`)
	if got := zcodeImageTargetModel("zcode/explicit-1", snap); got != "explicit-1" {
		t.Fatalf("线程显式模型应优先, got=%q", got)
	}
	if got := zcodeImageTargetModel("zcode/default", snap); got != "cur-model" {
		t.Fatalf("default 应回落快照 current, got=%q", got)
	}
	if got := zcodeImageTargetModel("", snap); got != "cur-model" {
		t.Fatalf("空线程模型回落快照 current, got=%q", got)
	}
	noCur := json.RawMessage(`{"settings":{"model":{"lastUsed":{"providerId":"p0","modelId":"used-model"}}}}`)
	if got := zcodeImageTargetModel("zcode/default", noCur); got != "used-model" {
		t.Fatalf("无 current 应回落 lastUsed, got=%q", got)
	}
	if got := zcodeImageTargetModel("zcode/default", json.RawMessage(`{}`)); got != "" {
		t.Fatalf("快照无模型信息应返回空串, got=%q", got)
	}
	if got := zcodeImageTargetModel("zcode/default", json.RawMessage(`broken`)); got != "" {
		t.Fatalf("坏快照应返回空串, got=%q", got)
	}
}

// TestZcodeSupportsImageResolution 纯函数级门控判定:
// 只有"明确声明 supportsImage=false"才拦;判定不了(快照缺失/模型不在表/无缓存)放行。
func TestZcodeSupportsImageResolution(t *testing.T) {
	snap := json.RawMessage(`{"settings":{"model":{
		"current":{"providerId":"p1","modelId":"MiniMax-M3"},
		"available":[
		  {"ref":{"providerId":"p1","modelId":"MiniMax-M3"},
		   "properties":{"inputFormat":{"supportsText":true,"supportsImage":true}}},
		  {"ref":{"providerId":"p2","modelId":"glm-5.2"},
		   "properties":{"inputFormat":{"supportsText":true,"supportsImage":false}}}]}}}`)

	p := NewZcodeProvider()
	if !p.zcodeSupportsImage("zcode/default", snap) {
		t.Fatal("current=MiniMax-M3(supportsImage=true) 应放行")
	}
	if p.zcodeSupportsImage("zcode/glm-5.2", snap) {
		t.Fatal("显式 glm-5.2(supportsImage=false) 应拦下")
	}
	// 判定不了 → 放行
	if !p.zcodeSupportsImage("zcode/unknown-model", snap) {
		t.Fatal("快照里查不到的模型应放行")
	}
	if !p.zcodeSupportsImage("zcode/default", nil) {
		t.Fatal("无快照且无缓存应放行")
	}
	if !p.zcodeSupportsImage("zcode/default", json.RawMessage(`broken`)) {
		t.Fatal("坏快照应放行")
	}
	// 缓存路径(threadMap 命中,无新鲜快照):refByModel 的 SupportsImages 位。
	p.mu.Lock()
	p.refByModel["glm-5.2"] = zcodeModelRef{ProviderID: "p2", SupportsImages: false}
	p.refByModel["MiniMax-M3"] = zcodeModelRef{ProviderID: "p1", SupportsImages: true}
	p.mu.Unlock()
	if p.zcodeSupportsImage("zcode/glm-5.2", nil) {
		t.Fatal("缓存位 false 应拦下")
	}
	if !p.zcodeSupportsImage("zcode/MiniMax-M3", nil) {
		t.Fatal("缓存位 true 应放行")
	}
	if !p.zcodeSupportsImage("zcode/absent", nil) {
		t.Fatal("缓存里没有该模型应放行")
	}
}

// TestZcodeSendParamsShape 纯函数级:放行的图片落临时文件,attachments 元素与
// 实测形状一致;cleanup 删除临时文件。
func TestZcodeSendParamsShape(t *testing.T) {
	imgs := []turnImage{
		{data: zcodeTinyPNG, mediaType: "image/png"},
		{data: []byte{0xff, 0xd8, 0xff, 0xe0}, mediaType: "image/jpeg"},
	}
	params, cleanup := zcodeSendParams("sess_1", "看这两张", imgs, true)
	defer cleanup()

	if params["sessionId"] != "sess_1" || params["content"] != "看这两张" {
		t.Fatalf("sessionId/content 不应被动过: %v", params)
	}
	atts, ok := params["attachments"].([]map[string]interface{})
	if !ok || len(atts) != 2 {
		t.Fatalf("attachments 应为 2 个元素, got=%v", params["attachments"])
	}
	for i, att := range atts {
		if att["kind"] != "image" {
			t.Fatalf("attachments[%d].kind 应为 image, got=%v", i, att["kind"])
		}
		path, _ := att["localPath"].(string)
		if !filepath.IsAbs(path) {
			t.Fatalf("attachments[%d].localPath 应为绝对路径, got=%q", i, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("attachments[%d] 临时文件应已落盘: %v", i, err)
		}
		if len(data) != len(imgs[i].data) {
			t.Fatalf("attachments[%d] 落盘字节应与原图一致", i)
		}
	}
	if atts[0]["mimeType"] != "image/png" || atts[1]["mimeType"] != "image/jpeg" {
		t.Fatalf("mimeType 应随输入: %v %v", atts[0]["mimeType"], atts[1]["mimeType"])
	}
	// cleanup 后临时文件删除
	cleanup()
	for _, att := range atts {
		path, _ := att["localPath"].(string)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("cleanup 后临时图片应删除, path=%q err=%v", path, err)
		}
	}
}

// TestZcodeSendParamsPlaceholder 纯函数级:imageOK=false 时不落文件、不带
// attachments,占位符按序编号、扩展名随 MIME;无图时不得出现 attachments 键。
func TestZcodeSendParamsPlaceholder(t *testing.T) {
	imgs := []turnImage{
		{data: zcodeTinyPNG, mediaType: "image/png"},
		{data: []byte{0xff, 0xd8, 0xff, 0xe0}, mediaType: "image/jpeg"},
	}
	params, cleanup := zcodeSendParams("sess_1", "正文", imgs, false)
	defer cleanup() // 应为空操作(没落任何文件)

	if _, has := params["attachments"]; has {
		t.Fatalf("降级时不应带 attachments: %v", params["attachments"])
	}
	content, _ := params["content"].(string)
	if !strings.Contains(content, "[图片:image-1.png]") || !strings.Contains(content, "[图片:image-2.jpg]") {
		t.Fatalf("content 应含按序占位(扩展名随 MIME): %q", content)
	}

	// 空文本 + 降级:占位即正文(content 不允许空到没有信息量)
	params2, cleanup2 := zcodeSendParams("sess_1", "", imgs[:1], false)
	defer cleanup2()
	content2, _ := params2["content"].(string)
	if content2 != "[图片:image-1.png]" {
		t.Fatalf("空文本时占位应为正文, got=%q", content2)
	}

	// 无图:参数与旧版纯文本路径完全一致(无 attachments 键)
	params3, cleanup3 := zcodeSendParams("sess_1", "纯文本", nil, true)
	defer cleanup3()
	if len(params3) != 2 || params3["content"] != "纯文本" {
		t.Fatalf("无图参数应只有 sessionId/content: %v", params3)
	}
}
