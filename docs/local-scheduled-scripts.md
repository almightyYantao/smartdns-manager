# 在管理台本机执行定时脚本

自定义脚本配置增加可选字段 `execution_mode`。省略或设置为 `remote` 时继续沿用远程节点执行方式；设置为 `local` 时，在管理服务所在环境执行，结果与错误写入现有任务历史。

```json
{
  "execution_mode": "local",
  "node_ids": [],
  "script": "exec python3 /persistent/tools/update.py --config /persistent/private/config.json --apply",
  "working_dir": "/persistent/tools",
  "timeout": 900,
  "env_vars": {},
  "run_as_user": ""
}
```

本次仅增加后端/API 能力，暂未增加专门的前端模式选择器；可通过现有任务管理 API 的 `config` JSON 设置这些字段（外层 `config` 为 JSON 字符串）。本机任务不能同时指定远程节点；脚本使用管理服务已有的系统用户，不执行提权或用户切换。运行所需文件应对该用户开放最小权限，敏感配置保持 `600`，并存储在持久化目录。

脚本遇到非零退出状态、超时或取消时，任务会标记为失败，标准输出与错误输出保留在执行历史中。Linux/macOS 上超时或取消会终止脚本所在进程组（包括普通子进程）；主动脱离该进程组的守护进程不在此保证范围。其他平台只取消直接进程；本功能依赖 `/bin/sh`，推荐在 Linux 容器中使用。调度表达式使用包含秒的六段格式，例如 `0 30 6 * * *`。

可用以下命令执行不依赖数据库或远程节点的测试：

```sh
cd backend
go test ./services -run 'Test(Local|CustomScript)' -count=1
```

本机执行并非沙箱：脚本可访问管理服务用户可读写的文件、环境变量及网络。仅可信管理员应有创建和修改脚本任务的权限；输出会进入任务历史，脚本应自行避免打印凭据。Docker 部署时“本机”指管理服务所在容器，并不指 Docker 宿主机。省略 `execution_mode` 的已有任务保持远程执行，不会自动迁移。
