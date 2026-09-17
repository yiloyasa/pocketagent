// Package store 负责"会话(Thread)"的持久化。
//
// 【讲解】P0 用最简单的方案:全部数据放一个 JSON 文件,启动时读进内存,
// 每次修改后整体写回。这对单机单用户的 agent 完全够用——等 P1 再换 SQLite,
// 到时候只需要改这个包,外面调用方不动。这叫"封装变化点"。
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

type Thread struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Workspace string `json:"workspace"`
	Goal      string `json:"goal"`
	ModelID   string `json:"model_id"`
	// Codex 桥接:对应 codex app-server 的 threadId(daemon 重启后仍能续上会话)
	CodexThreadID string `json:"codex_thread_id,omitempty"`
	// 思考强度(low/medium/high/xhigh);空 = 跟随桌面默认
	Effort string `json:"effort,omitempty"`
	// 所属分组(codex 侧 threadSection 的 id);空 = 未分组。
	// 分组本身存在于 codex,这里只缓存"这条会话在哪个组",
	// 免得列一次会话就要为每条去问一次 codex。
	SectionID string          `json:"section_id,omitempty"`
	Status    pb.ThreadStatus `json:"status"`
	CreatedAt int64           `json:"created_at"`
	UpdatedAt int64           `json:"updated_at"`
	Messages  []StoredMessage `json:"messages"`
	Usage     pb.Usage        `json:"usage"`
}

// StoredMessage 是一轮对话的一条记录(用户输入或助手回复)。
type StoredMessage struct {
	Role      string // "user" | "assistant"
	Blocks    []pb.ContentBlock
	CreatedAt int64
}

// proto 消息里带 oneof(文本/图片/工具调用),encoding/json 处理不了,
// 所以消息块用 protojson 序列化——这是 protobuf 官方的 JSON 格式。
type storedMessageJSON struct {
	Role      string            `json:"role"`
	Blocks    []json.RawMessage `json:"blocks"`
	CreatedAt int64             `json:"created_at"`
}

func (m StoredMessage) MarshalJSON() ([]byte, error) {
	out := storedMessageJSON{Role: m.Role, CreatedAt: m.CreatedAt}
	for i := range m.Blocks {
		raw, err := protojson.Marshal(&m.Blocks[i])
		if err != nil {
			return nil, err
		}
		out.Blocks = append(out.Blocks, raw)
	}
	return json.Marshal(out)
}

func (m *StoredMessage) UnmarshalJSON(data []byte) error {
	var in storedMessageJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	m.Role, m.CreatedAt = in.Role, in.CreatedAt
	for _, raw := range in.Blocks {
		var b pb.ContentBlock
		if err := protojson.Unmarshal(raw, &b); err != nil {
			return err
		}
		m.Blocks = append(m.Blocks, b)
	}
	return nil
}

type FileStore struct {
	mu   sync.Mutex
	path string
	data struct {
		Threads          map[string]*Thread `json:"threads"`
		DefaultWorkspace string             `json:"default_workspace"`
		DefaultModelID   string             `json:"default_model_id"`
		DefaultEffort    string             `json:"default_effort"`
	}
}

func NewFileStore(dataDir string) (*FileStore, error) {
	s := &FileStore{path: filepath.Join(dataDir, "threads.json")}
	s.data.Threads = map[string]*Thread{}
	raw, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, err
	}
	if s.data.Threads == nil {
		s.data.Threads = map[string]*Thread{}
	}
	return s, nil
}

func (s *FileStore) saveLocked() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path) // 先写临时文件再改名,断电不会写坏
}

func (s *FileStore) Create(t *Thread) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	t.CreatedAt, t.UpdatedAt = now, now
	if t.Status == pb.ThreadStatus_THREAD_STATUS_UNSPECIFIED {
		t.Status = pb.ThreadStatus_THREAD_STATUS_IDLE
	}
	s.data.Threads[t.ID] = t
	return s.saveLocked()
}

func (s *FileStore) Get(id string) (*Thread, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.data.Threads[id]
	return t, ok
}

func (s *FileStore) List(includeArchived bool) []*Thread {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Thread, 0, len(s.data.Threads))
	for _, t := range s.data.Threads {
		if !includeArchived && t.Status == pb.ThreadStatus_THREAD_STATUS_ARCHIVED {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}

func (s *FileStore) Update(t *Thread) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t.UpdatedAt = time.Now().UnixMilli()
	s.data.Threads[t.ID] = t
	return s.saveLocked()
}

func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Threads, id)
	return s.saveLocked()
}

func (s *FileStore) DefaultWorkspace() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.DefaultWorkspace
}
func (s *FileStore) SetDefaultWorkspace(ws string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DefaultWorkspace = ws
	return s.saveLocked()
}
func (s *FileStore) DefaultModelID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.DefaultModelID
}
func (s *FileStore) SetDefaultModelID(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DefaultModelID = id
	return s.saveLocked()
}
func (s *FileStore) DefaultEffort() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.DefaultEffort
}
func (s *FileStore) SetDefaultEffort(e string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DefaultEffort = e
	return s.saveLocked()
}
