package agency

import (
	"context"
	"encoding/json"
	"os"
)

// StoreFileSource 从 JSON 文件探测模型候选(自适应提取,零固定 schema)。
// 典型路径:pi 的 ~/.pi/agent/models-store.json(pi update 维护的模型目录)。
type StoreFileSource struct {
	Path     string
	Provider string // 空 = 由提取器从文件内 provider 分桶键继承
}

func (s StoreFileSource) Name() string { return "store:" + s.Path }

func (s StoreFileSource) Probe(ctx context.Context) ([]Candidate, error) {
	raw, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return ExtractCandidates(v), nil
}
