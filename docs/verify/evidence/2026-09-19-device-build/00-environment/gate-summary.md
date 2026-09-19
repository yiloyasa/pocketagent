# 构建门禁结果 (方案 §5.1)

- 执行时间: 2026-09-19 22:31:21

| 步骤 | 结果 | 耗时 | rc |
|---|---|---:|---|
| 门禁1  go test ./... | PASS | 27.8s | rc=0 |
| 门禁2  go test -race | PASS | 28.8s | rc=0 |
| 门禁3  单测 | PASS | 25.9s | rc=0 |
| 门禁4  assembleDebug | PASS | 9.8s | rc=0 |

**门禁总判定: PASS**