# 候选物冻结 / 环境记录 (2026-09-19-device-build)

- 冻结时间: 2026-09-19 22:29:32
- Git HEAD: 8cc0c5e8f95a0da807b3d68823bc970b8c1e767e
- 工作树脏文件数: 334 (完整 diff 见 git-diff.txt)
- APK SHA-256: 554c87b1359de44603bf04744a1bfd06a62ef1cb7ea0e3759c75a74bbd16aae5
- Daemon SHA-256: f220d385321a2ecf5702d6a6fb63c452abb2800d97f1e7dd9dae5c6624cf9b4c
- SenseVoice 资产校验: PASS

## 工具链

```
OK	go	rc=0	go version go1.27.1 windows/amd64
OK	gcc  (-race 依赖)	rc=0	clang version 22.1.8 (https://github.com/llvm/llvm-project.git ca7933e47d3a3451d81e72ac174dcb5aa28b59d1)
OK	pwsh	rc=0	7.6.6
OK	powershell	rc=0	5.1.26100.9444
OK	java	rc=0	openjdk version "17.0.20" 2026-07-21
OK	git	rc=0	git version 2.55.0.windows.2
OK	adb	rc=0	Android Debug Bridge version 1.0.41
```

## 设备

```
品牌 = Xiaomi
型号 = 2211133C
设备代号 = fuxi
Android = 16
API = 36
ROM = OS3.0
ROM(incre) = OS3.0.308.0.WMCCNXM
SDK 指纹 = Xiaomi/fuxi/fuxi:16/BP2A.250605.031.A3/OS3.0.308.0.WMCCNXM:user/release-keys
CPU ABI = arm64-v8a

--- meminfo (head) ---
MemTotal:       11459124 kB
MemFree:          449220 kB
MemAvailable:    5455964 kB
Buffers:            2036 kB
Cached:          3939040 kB

--- df /data ---
Filesystem       Size Used Avail Use% Mounted on
/dev/block/dm-58 224G 180G   44G  81% /data/user/0
```
