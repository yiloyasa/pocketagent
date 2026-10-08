//go:build windows

// jobobject_windows_test.go —— Job Object 兜底层的最小自证测试(Windows-only,
// 照包内 jobobject_windows.go 的 build tag 惯例)。
//
// 【钉什么】Assign 返回后,当前进程必须真的在刚建的 Job 里 —— 用
// QueryInformationJobObject(JobObjectBasicProcessIdList)枚举 Job 成员、
// 找到本进程 pid 自证。这是"内核级父子绑定"生效的前提:绑定失败而静默,
// 守护链就退化成"有孤儿可杀"而无人知晓,包注释里的承诺必须有测试背书。
//
// 【为什么安全】测试进程被绑进 Kill-on-Close Job:句柄由本测试持有直到进程
// 退出,内核在进程终止后收尾 Job —— 不影响同进程内其他测试;CI 若已有外层
// Job,Win8+ 嵌套 Job 语义允许二次绑定。
package jobobject

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// jobProcListInfo 镜像 JOBOBJECT_BASIC_PROCESS_ID_LIST 的内存布局
// (x/sys 未封装该结构;头两个 DWORD 计数,后跟 ULONG_PTR 数组,x64 下
// 计数区与数组间无额外填充 —— 8 字节已对齐)。
type jobProcListInfo struct {
	Assigned uint32
	InList   uint32
	Pids     [16]uintptr
}

func TestAssignPutsCurrentProcessInJob(t *testing.T) {
	job, err := Assign()
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if job == 0 {
		t.Fatal("Assign 应返回有效 Job 句柄")
	}
	var list jobProcListInfo
	if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicProcessIdList,
		uintptr(unsafe.Pointer(&list)), uint32(unsafe.Sizeof(list)), nil); err != nil {
		t.Fatalf("QueryInformationJobObject: %v", err)
	}
	if list.InList == 0 {
		t.Fatal("Job 成员列表为空,当前进程不在 Job 里")
	}
	me := windows.GetCurrentProcessId()
	for _, pid := range list.Pids[:list.InList] {
		if uint32(pid) == me {
			return // 自证成功:本进程 pid 在 Job 成员列表里
		}
	}
	t.Fatalf("当前进程 pid=%d 不在 Job 成员列表(成员数 %d)中", me, list.InList)
}
