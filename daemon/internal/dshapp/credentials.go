package dshapp

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// dshCredentialWhitelist 是我们主动支持、允许从 dsh-credentials.env 注入子进程的
// "环境变量键"白名单。
//
// 【为什么是"能力声明"而非"agent/模型名单"(合规红线)】这里只声明 daemon 愿意
// 帮忙透传哪几个"凭据键",与 dsh 支持哪些 agent / 模型毫无关系;新增一个键是一次
// 显式的能力开放动作,不隐含任何 agent 或模型选择。名单外的键一律忽略(见
// LoadCredentialEnv):绝不读、绝不透传、绝不大声报错。
var dshCredentialWhitelist = map[string]bool{
	"DEEPSEEK_API_KEY": true,
}

// DefaultDataDir 返回 daemon 数据目录默认值,与 cmd/pocketagentd/main.go 的
// -data flag 默认值一致(~/.pocketagentd)。
//
// 【为什么本包自己解析默认值而不读全局 flag】传递链路(server/main 持有真值)被
// 项目约束列为不可改文件;为保持本包自洽、零耦合,这里只做"拿不到显式目录时"
// 的兜底 —— 调用方若握有真实数据目录,应经 StartOptions.DataDir 显式传入覆盖。
func DefaultDataDir() string {
	return defaultDataDir()
}

// defaultDataDir 与 client.go 的 defaultDshHome 同源:都以 ~/.pocketagentd 为
// daemon 数据根(home 取不到时退 TempDir)。
func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "pocketagentd")
	}
	return filepath.Join(home, ".pocketagentd")
}

// CredentialEnvPath 返回 dsh-credentials.env 的完整路径。dataDir 为空时回落到
// DefaultDataDir()(尊重显式数据目录;否则用默认值)。
func CredentialEnvPath(dataDir string) string {
	if dataDir == "" {
		dataDir = DefaultDataDir()
	}
	return filepath.Join(dataDir, "dsh-credentials.env")
}

// LoadCredentialEnv 解析 <dataDir>/dsh-credentials.env,返回允许注入 dsh 子进程的
// KEY=VALUE 行。设计要点(逐项对应项目约束):
//   - 文件不存在 → 返回 nil,不算错误,静默降级(dsh 直接用进程 env,功能照常);
//   - 注释(# 开头)与空行忽略;按首个 '=' 切分,值两侧空白 trim,值内允许 '=';
//   - 非法行(无 '=' / 空键)log 一行警告并跳过该行,不让整个 dsh 起不来;
//   - 白名单外键忽略(不读、不透传、不报错);
//   - env 优先:进程环境已有同名变量时该键不进结果(文件只作补充,绝不覆盖进程 env);
//   - 权限过宽(仅非 Windows 检查,且只看"组/其他可写"):log 警告并跳过注入
//     (不注入世界可写的密钥文件);Windows 跳过权限检查(chmod 语义不同,不报错)。
//
// 【红线】绝不读写用户 ~/.dsh/:凭据只来自本 daemon 数据目录下的 dsh-credentials.env,
// 与用户桌面 dsh 的 ~/.dsh/.credentials.yaml 零共享、零接触、零复制。
func LoadCredentialEnv(dataDir string) []string {
	path := CredentialEnvPath(dataDir)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 静默降级:无文件 ≠ 错误
		}
		log.Printf("[dsh] 读取凭据文件失败(跳过注入): path=%s err=%v", path, err)
		return nil
	}
	defer f.Close()

	if runtime.GOOS != "windows" {
		if fi, statErr := f.Stat(); statErr == nil {
			// 只看"组/其他可写"(0022):默认 0644 可读不拦,0666 才拦;
			// 世界可写的密钥文件有泄露风险,宁可跳过注入也不冒进。
			if fi.Mode().Perm()&0022 != 0 {
				log.Printf("[dsh] 凭据文件权限过宽(组/其他可写),跳过注入: path=%s", path)
				return nil
			}
		}
	}

	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			log.Printf("[dsh] 凭据文件非法行(无 '=')已跳过: %s", line)
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if key == "" {
			log.Printf("[dsh] 凭据文件非法行(空键)已跳过: %s", line)
			continue
		}
		if !dshCredentialWhitelist[key] {
			// 白名单外键:主动支持的能力声明外,忽略(不报错、不注入)
			continue
		}
		if os.Getenv(key) != "" {
			// 进程环境已有同名变量:env 优先,文件只补充,不覆盖。
			continue
		}
		out = append(out, key+"="+val)
	}
	if err := sc.Err(); err != nil {
		log.Printf("[dsh] 读取凭据文件异常(已解析部分仍注入): path=%s err=%v", path, err)
	}
	return out
}
