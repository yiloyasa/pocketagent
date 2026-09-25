# Pi 原生设置探针记录(auto_compaction/steering_mode 读写语义,隔离 agent 目录)

> 生成:2026-09-25 11:15:41 工具:daemon/cmd/piprobe(直接 node 拉起 cli.js --mode rpc,Go 手写 JSONL 分帧)

结果:**3 通过 / 0 替代 / 0 失败**(共 3 项)

## 1. get_state 原生状态读取 —— 通过

autoCompactionEnabled=true(在场=true) steeringMode=one-at-a-time(在场=true) autoRetryEnabled=<nil>(在场=false)
```
{"autoCompactionEnabled":true,"followUpMode":"one-at-a-time","isCompacting":false,"isStreaming":false,"messageCount":0,"model":{"api":"unknown","baseUrl":"","contextWindow":0,"cost":{"cacheRead":0,"cacheWrite":0,"input":0,"output":0},"id":"unknown","input":[],"maxTokens":0,"name":"unknown","provider":"unknown","reasoning":false},"pendingMessageCount":0,"sessionFile":"C:\\Users\\yanzi\\AppData\\Local\\Temp\\piprobe-native-2574595479\\sessions\\2026-09-25T03-15-35-802Z_01a0d68f-aef9-739f-a1f3-aebe…
```

## 2. set_auto_compaction(false) 写通道 —— 通过

set 后 get_state.autoCompactionEnabled=false;隔离 agentDir/settings.json 内容={
  "compaction": {
    "enabled": false
  }
}
```
{
  "compaction": {
    "enabled": false
  }
}
```

## 3. set_steering_mode(all) 写通道 —— 通过

隔离 agentDir/settings.json 含 steeringMode=true;内容={
  "compaction": {
    "enabled": false
  },
  "steeringMode": "all"
}
```
{
  "compaction": {
    "enabled": false
  },
  "steeringMode": "all"
}
```

## 原始报文附录(全部,截断)

```
{"id":"piprobe-1","type":"response","command":"get_state","success":true,"data":{"model":{"id":"unknown","name":"unknown","api":"unknown","provider":"unknown","baseUrl":"","reasoning":false,"input":[],"cost":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0},"contextWindow":0,"maxTokens":0},"thinkingLevel":"off","isStreaming":false,"isCompacting":false,"steeringMode":"one-at-a-time","followUpMode":"one-at-a-time","sessionFile":"C:\\Users\\yanzi\\AppData\\Local\\Temp\\piprobe-native-2574595479\\sessions\\2026-09-25T03-15-35-802Z_01a0d68f-aef9-739f-a1f3-aebe70fad2bb.jsonl","sessionId":"01a0d68f …[截断,整行 725 字节]
{"id":"piprobe-2","type":"response","command":"set_auto_compaction","success":true}
{"id":"piprobe-3","type":"response","command":"get_state","success":true,"data":{"model":{"id":"unknown","name":"unknown","api":"unknown","provider":"unknown","baseUrl":"","reasoning":false,"input":[],"cost":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0},"contextWindow":0,"maxTokens":0},"thinkingLevel":"off","isStreaming":false,"isCompacting":false,"steeringMode":"one-at-a-time","followUpMode":"one-at-a-time","sessionFile":"C:\\Users\\yanzi\\AppData\\Local\\Temp\\piprobe-native-2574595479\\sessions\\2026-09-25T03-15-35-802Z_01a0d68f-aef9-739f-a1f3-aebe70fad2bb.jsonl","sessionId":"01a0d68f …[截断,整行 726 字节]
{"id":"piprobe-4","type":"response","command":"set_steering_mode","success":true}
```

## 结论(对 PiProvider 设计的约束)

| 项 | 实测 |
|---|---|
| get_state.autoCompactionEnabled | 在场(bool),只读呈现的数据源 |
| get_state.steeringMode | 在场(字符串,one-at-a-time/all),只读呈现的数据源 |
| get_state.autoRetryEnabled | **不在场**(无运行时可读面,不呈现、不伪造) |
| set_auto_compaction 落盘 | **写入 `<agentDir>/settings.json` 的 compaction.enabled(全局持久)** |
| set_steering_mode 落盘 | **写入 `<agentDir>/settings.json` 的 steeringMode(全局持久)** |

**红线判定**:set_auto_compaction / set_auto_retry / set_steering_mode 三条写通道经
pi 自己的 settingsManager 落到**用户桌面全局配置**(~/.pi 的 settings.json;生产路径
collab 未启用时 daemon 不注入 PI_CODING_AGENT_DIR,子进程用用户真实目录)。按红线 1
(绝不写用户桌面配置),PocketAgent **不提供这三项的写 UI**;autoCompactionEnabled 与
steeringMode 只做只读呈现,note 如实说明"由 Pi 桌面端管理"。
(本探针的写副作用全程落在临时隔离目录,未触碰用户 ~/.pi。)
