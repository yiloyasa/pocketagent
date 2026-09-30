# Pi 原生设置探针记录(auto_compaction/steering_mode 读写语义,隔离 agent 目录)

> 生成:2026-09-30 09:02:42 工具:daemon/cmd/piprobe(直接 node 拉起 cli.js --mode rpc,Go 手写 JSONL 分帧)

结果:**3 通过 / 0 替代 / 0 失败**(共 3 项)

## 1. get_state 原生状态读取 —— 通过

autoCompactionEnabled=true(在场=true) steeringMode=one-at-a-time(在场=true) autoRetryEnabled=<nil>(在场=false)
```
{"autoCompactionEnabled":true,"followUpMode":"one-at-a-time","isCompacting":false,"isStreaming":false,"messageCount":0,"model":{"api":"unknown","baseUrl":"","contextWindow":0,"cost":{"cacheRead":0,"cacheWrite":0,"input":0,"output":0},"id":"unknown","input":[],"maxTokens":0,"name":"unknown","provider":"unknown","reasoning":false},"pendingMessageCount":0,"sessionFile":"C:\\Users\\yanzi\\AppData\\Local\\Temp\\piprobe-native-1383133629\\sessions\\2026-09-30T01-02-36-894Z_01a0efd5-bb5e-77fd-8ed7-cd50…
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
{"id":"piprobe-1","type":"response","command":"get_state","success":true,"data":{"model":{"id":"unknown","name":"unknown","api":"unknown","provider":"unknown","baseUrl":"","reasoning":false,"input":[],"cost":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0},"contextWindow":0,"maxTokens":0},"thinkingLevel":"off","isStreaming":false,"isCompacting":false,"steeringMode":"one-at-a-time","followUpMode":"one-at-a-time","sessionFile":"C:\\Users\\yanzi\\AppData\\Local\\Temp\\piprobe-native-1383133629\\sessions\\2026-09-30T01-02-36-894Z_01a0efd5-bb5e-77fd-8ed7-cd50643cda3d.jsonl","sessionId":"01a0efd5 …[截断,整行 725 字节]
{"id":"piprobe-2","type":"response","command":"set_auto_compaction","success":true}
{"id":"piprobe-3","type":"response","command":"get_state","success":true,"data":{"model":{"id":"unknown","name":"unknown","api":"unknown","provider":"unknown","baseUrl":"","reasoning":false,"input":[],"cost":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0},"contextWindow":0,"maxTokens":0},"thinkingLevel":"off","isStreaming":false,"isCompacting":false,"steeringMode":"one-at-a-time","followUpMode":"one-at-a-time","sessionFile":"C:\\Users\\yanzi\\AppData\\Local\\Temp\\piprobe-native-1383133629\\sessions\\2026-09-30T01-02-36-894Z_01a0efd5-bb5e-77fd-8ed7-cd50643cda3d.jsonl","sessionId":"01a0efd5 …[截断,整行 726 字节]
{"id":"piprobe-4","type":"response","command":"set_steering_mode","success":true}
```
