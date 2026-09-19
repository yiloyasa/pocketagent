// codex_threads.go —— 会话组织:分组(section)、分支(fork)、回滚(rollback)、归档。
//
// 【为什么这些必须进手机端】会话一多,手机上的会话列表就退化成"一长串标题":
// 分不了组、试错试坏了一条没法回退、电脑上归档了的在手机上还占着位置。
// 桌面 codex 早就有这三件事,而且**状态存在 codex 侧**——我们只要如实读写,
// 双端就自然一致(在电脑上分的组,手机上立刻能看到)。
//
// 【边界】fork 与 rollback 都会改变"这个会话之后是什么":fork 分出一条新会话,
// rollback 直接丢掉最近 N 个回合。协议里 rollback 明确写着"不会回滚 agent 已经
// 对本地文件做的改动"(官方注释:This only modifies the thread's history and does
// not revert local file changes)。所以调用方(App)必须先让用户二次确认 ——
// 这个文件只负责忠实执行,不做"贴心"的自动兜底。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pocketagent/daemon/internal/codexapp"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// CodexThreadEntry 是 thread/list 一条记录里我们关心的部分。
type CodexThreadEntry struct {
	ID        string
	Title     string
	Cwd       string
	UpdatedAt int64 // unix 秒(codex 侧单位)
	// SectionID 来自 thread.section.id;空 = 未分组。
	SectionID string
	// Archived 来自 thread/list 的 archived 过滤结果(单独一次查询),
	// 这里只标记"这条是不是从归档查询里拿到的"。
	Archived bool
}

// ListThreadEntries 拉一次 thread/list。
//
// includeArchived=true 时需要两次调用:协议的 archived 参数是**互斥过滤**
// (true 只回归档的,false/null 只回非归档的),一次拿不全。
func (p *CodexProvider) ListThreadEntries(searchTerm, sectionID, sortKey string, sortAsc, includeArchived bool, limit int) ([]CodexThreadEntry, error) {
	var out []CodexThreadEntry
	if err := p.withClient(func(*codexapp.Client) error { return nil }); err != nil {
		return nil, err
	}
	passes := []bool{false}
	if includeArchived {
		passes = append(passes, true)
	}
	for _, archived := range passes {
		entries, err := p.listThreadEntriesOnce(searchTerm, sortKey, sortAsc, archived, limit)
		if err != nil {
			// 归档那一次失败不该让整个列表变空:非归档的已经拿到了。
			if len(out) > 0 {
				return out, nil
			}
			return nil, err
		}
		out = append(out, entries...)
	}
	if sectionID != "" {
		// 分组过滤在本地做:codex 的 thread/list 没有 section 过滤参数。
		kept := out[:0]
		for _, e := range out {
			if e.SectionID == sectionID {
				kept = append(kept, e)
			}
		}
		out = kept
	}
	return out, nil
}

func (p *CodexProvider) listThreadEntriesOnce(searchTerm, sortKey string, sortAsc, archived bool, limit int) ([]CodexThreadEntry, error) {
	var out []CodexThreadEntry
	cursor := ""
	for page := 0; page < 10; page++ {
		params := map[string]interface{}{"archived": archived}
		if cursor != "" {
			params["cursor"] = cursor
		}
		if searchTerm != "" {
			params["searchTerm"] = searchTerm
		}
		if limit > 0 {
			// 分页拉,但每页不超过剩余需求
			remain := limit - len(out)
			if remain <= 0 {
				return out, nil
			}
			params["limit"] = remain
		}
		raw, err := p.client.Call("thread/list", params)
		if err != nil {
			return out, err
		}
		// 【实测纠正】result.data 是**数组**,不是 {items:[...]}。
		// (JSON Schema 里的 "items" 是数组元素声明,不是字段名 —— 这里曾经
		// 因此解析失败,而失败又被上层的 rollout 扫描兜底掩盖了,很久没被发现。)
		var res struct {
			Data []struct {
				ID        string  `json:"id"`
				Name      *string `json:"name"`
				Preview   string  `json:"preview"`
				Cwd       string  `json:"cwd"`
				UpdatedAt int64   `json:"updatedAt"`
				Section   *struct {
					ID string `json:"id"`
				} `json:"section"`
			} `json:"data"`
			NextCursor *string `json:"nextCursor"`
		}
		if json.Unmarshal(raw, &res) != nil {
			return out, fmt.Errorf("thread/list 应答无法解析: %s", clip(string(raw), 200))
		}
		for _, it := range res.Data {
			e := CodexThreadEntry{
				ID:        it.ID,
				Cwd:       it.Cwd,
				UpdatedAt: it.UpdatedAt,
				Archived:  archived,
			}
			if it.Name != nil && *it.Name != "" {
				e.Title = *it.Name
			} else if it.Preview != "" {
				r := []rune(it.Preview)
				if len(r) > 40 {
					r = r[:40]
				}
				e.Title = string(r)
			}
			if it.Section != nil {
				e.SectionID = it.Section.ID
			}
			out = append(out, e)
		}
		if res.NextCursor == nil || *res.NextCursor == "" {
			break
		}
		cursor = *res.NextCursor
	}
	// codex 的排序参数我们原样透传,但**本地再排一次**:分页 + 两次查询(归档)拼出来的
	// 结果顺序一定是乱的,不重排会出现"归档的混在中间"。
	sortEntries(out, sortKey, sortAsc)
	return out, nil
}

func sortEntries(entries []CodexThreadEntry, sortKey string, asc bool) {
	// 默认按更新时间(与 codex 一致),新的在前。
	less := func(a, b CodexThreadEntry) bool { return a.UpdatedAt > b.UpdatedAt }
	if sortKey == "created" {
		// 协议里没有 created 排序键暴露给 thread/list 响应,这里退化为更新时间
		// (两者在我们的使用场景里差别只在"很久没动过的会话",不值得多打一次请求)。
		less = func(a, b CodexThreadEntry) bool { return a.UpdatedAt > b.UpdatedAt }
	}
	if asc {
		inner := less
		less = func(a, b CodexThreadEntry) bool { return inner(b, a) }
	}
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && less(entries[j], entries[j-1]); j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}

// ---- 分组(section)----

// ThreadSections 列出 codex 侧的全部分组。
func (p *CodexProvider) ThreadSections() ([]*pb.ThreadSection, error) {
	out := []*pb.ThreadSection{}
	err := p.withStatusClient(func(ctx context.Context, c *codexapp.Client) error {
		cursor := ""
		for page := 0; page < 10; page++ {
			params := map[string]interface{}{}
			if cursor != "" {
				params["cursor"] = cursor
			}
			raw, err := c.CallCtx(ctx, "threadSection/list", params)
			if err != nil {
				return err
			}
			var res struct {
				Data []struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					Appearance *struct {
						Color *string `json:"color"`
						Icon  *string `json:"icon"`
					} `json:"appearance"`
				} `json:"data"`
				NextCursor *string `json:"nextCursor"`
			}
			if err := json.Unmarshal(raw, &res); err != nil {
				return fmt.Errorf("threadSection/list 应答无法解析: %s", clip(string(raw), 200))
			}
			for _, s := range res.Data {
				item := &pb.ThreadSection{Id: s.ID, Name: s.Name}
				if s.Appearance != nil {
					item.Color = derefTrim(s.Appearance.Color)
					item.Icon = derefTrim(s.Appearance.Icon)
				}
				out = append(out, item)
			}
			if res.NextCursor == nil || *res.NextCursor == "" {
				break
			}
			cursor = *res.NextCursor
		}
		return nil
	})
	return out, err
}

func (p *CodexProvider) CreateThreadSection(name string) (*pb.ThreadSection, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("分组名不能为空")
	}
	var out *pb.ThreadSection
	err := p.withClient(func(c *codexapp.Client) error {
		raw, err := c.Call("threadSection/create", map[string]interface{}{"name": strings.TrimSpace(name)})
		if err != nil {
			return err
		}
		var res struct {
			Section struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"section"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return fmt.Errorf("threadSection/create 应答无法解析: %s", clip(string(raw), 200))
		}
		out = &pb.ThreadSection{Id: res.Section.ID, Name: res.Section.Name}
		return nil
	})
	return out, err
}

func (p *CodexProvider) RenameThreadSection(sectionID, name string) (*pb.ThreadSection, error) {
	if sectionID == "" {
		return nil, fmt.Errorf("缺少分组 id")
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("分组名不能为空")
	}
	var out *pb.ThreadSection
	err := p.withClient(func(c *codexapp.Client) error {
		raw, err := c.Call("threadSection/update", map[string]interface{}{
			"sectionId": sectionID, "name": strings.TrimSpace(name),
		})
		if err != nil {
			return err
		}
		var res struct {
			Section struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"section"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return fmt.Errorf("threadSection/update 应答无法解析: %s", clip(string(raw), 200))
		}
		out = &pb.ThreadSection{Id: res.Section.ID, Name: res.Section.Name}
		return nil
	})
	return out, err
}

func (p *CodexProvider) DeleteThreadSection(sectionID string) error {
	if sectionID == "" {
		return fmt.Errorf("缺少分组 id")
	}
	return p.withClient(func(c *codexapp.Client) error {
		_, err := c.Call("threadSection/delete", map[string]interface{}{"sectionId": sectionID})
		return err
	})
}

// MoveThreadToSection 把会话移进/移出分组。sectionID 为空表示移出。
//
// 【协议坑】sectionId 必须显式传 null 才是"移出分组",传空字符串会被
// 当成"某个叫空字符串的分组"。所以这里用 nil 而不是 ""。
func (p *CodexProvider) MoveThreadToSection(codexThreadID, sectionID string) error {
	if codexThreadID == "" {
		return fmt.Errorf("缺少 codex 线程 id")
	}
	var dest interface{}
	if sectionID != "" {
		dest = sectionID
	}
	return p.withClient(func(c *codexapp.Client) error {
		_, err := c.Call("thread/section/move", map[string]interface{}{
			"threadId": codexThreadID, "sectionId": dest,
		})
		return err
	})
}

// ---- 分支 / 回滚 / 归档 / 改名 ----

// ForkThread 从某个回合之后分出一条新会话,返回新 codex 线程 id。
//
// lastTurnID 为空 = 从最新状态分叉。协议语义是"fork through, inclusive",
// 即 lastTurnID 这一回合**会**被复制过去。
func (p *CodexProvider) ForkThread(codexThreadID, lastTurnID string) (string, error) {
	if codexThreadID == "" {
		return "", fmt.Errorf("这个会话还没有在电脑上落地,无法分叉")
	}
	newID := ""
	err := p.withClient(func(c *codexapp.Client) error {
		params := map[string]interface{}{"threadId": codexThreadID}
		if lastTurnID != "" {
			params["lastTurnId"] = lastTurnID
		}
		raw, err := c.Call("thread/fork", params)
		if err != nil {
			return err
		}
		var res struct {
			Thread struct {
				ID string `json:"id"`
			} `json:"thread"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return fmt.Errorf("thread/fork 应答无法解析: %s", clip(string(raw), 200))
		}
		newID = res.Thread.ID
		if newID == "" {
			return fmt.Errorf("thread/fork 没回新线程 id: %s", clip(string(raw), 200))
		}
		return nil
	})
	return newID, err
}

// RollbackThread 丢掉最近 numTurns 个回合。
func (p *CodexProvider) RollbackThread(codexThreadID string, numTurns int32) error {
	if codexThreadID == "" {
		return fmt.Errorf("这个会话还没有在电脑上落地,无法回滚")
	}
	if numTurns <= 0 {
		return fmt.Errorf("回滚回合数必须大于 0")
	}
	return p.withClient(func(c *codexapp.Client) error {
		_, err := c.Call("thread/rollback", map[string]interface{}{
			"threadId": codexThreadID, "numTurns": numTurns,
		})
		return err
	})
}

// SetCodexArchived 归档 / 取消归档(codex 侧为事实来源)。
func (p *CodexProvider) SetCodexArchived(codexThreadID string, archived bool) error {
	if codexThreadID == "" {
		return fmt.Errorf("这个会话还没有在电脑上落地,无法归档")
	}
	method := "thread/unarchive"
	if archived {
		method = "thread/archive"
	}
	return p.withClient(func(c *codexapp.Client) error {
		_, err := c.Call(method, map[string]interface{}{"threadId": codexThreadID})
		return err
	})
}

// SetCodexThreadName 把标题写回 codex(双端一致的唯一做法)。
func (p *CodexProvider) SetCodexThreadName(codexThreadID, name string) error {
	if codexThreadID == "" {
		return fmt.Errorf("这个会话还没有在电脑上落地,无法改名")
	}
	return p.withClient(func(c *codexapp.Client) error {
		_, err := c.Call("thread/name/set", map[string]interface{}{
			"threadId": codexThreadID, "name": name,
		})
		return err
	})
}
