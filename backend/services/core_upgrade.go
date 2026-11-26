package services

import (
	"encoding/base64"
	"fmt"
	"net/netip"
	"strings"
)

func parseExpectedIPv4(value string) (string, error) {
	addr, err := netip.ParseAddr(value)
	if err != nil || !addr.Is4() {
		return "", fmt.Errorf("请输入预期的 IPv4 地址")
	}
	return addr.String(), nil
}

// 相对路径的 ELF 解释器由进程启动时的工作目录解析。
const coreRunningVersion = `running_core_version() (
 cd "$1/cwd" && "$1/exe" -v 2>&1
)
`

const coreDNSProbe = `probe_answers() {
 response=$(dig -r @127.0.0.1 "$name" A +time=2 +tries=1 +noall +comments +answer) &&
 printf '%s\n' "$response" | grep -q 'status: NOERROR,' &&
 printf '%s\n' "$response" | awk '$4=="A" {print $5; found=1} END {exit !found}'
}
probe() {
 answers=$(probe_answers) &&
 printf '%s\n' "$baseline" | grep -Fxq -- "$answers"
}
`

const coreUpgradeScript = coreRunningVersion + coreDNSProbe + `set -eu
umask 077
digest=$1 before=$2 after=$3 name=$4 answer=$5
for tool in base64 dig sha256sum flock timeout; do
 command -v "$tool" >/dev/null || { echo "节点缺少命令 $tool"; exit 1; }
done
exec 9>/run/lock/smartdns-manager-core.lock
flock -n 9 || { echo '此节点已有升级任务'; exit 1; }
running_pid() {
 systemctl show -p MainPID smartdns | cut -d= -f2
}
verify() {
 expected=$1
 for _ in 1 2 3; do
  pid=$(running_pid) || return 1
  if [ "$pid" -gt 0 ] && systemctl is-active --quiet smartdns &&
   [ "$(sha256sum "/proc/$pid/exe" | cut -d' ' -f1)" = "$expected" ] && probe; then return 0; fi
  sleep 1
 done
 return 1
}
pid=$(running_pid)
test "$pid" -gt 0
path=$(readlink "/proc/$pid/exe")
case "$path" in
 /usr/sbin/smartdns|/usr/bin/smartdns|/usr/local/sbin/smartdns|/usr/local/bin/smartdns) ;;
 *) echo '当前运行路径不支持自动升级'; exit 1;;
esac
[ "$(readlink -f "$path")" = "$path" ] || { echo '当前核心路径包含符号链接'; exit 1; }
running_core_version "/proc/$pid" | grep -Fq "($before)" || { echo '当前版本已变化，请重新检测'; exit 1; }
old_digest=$(sha256sum "/proc/$pid/exe" | cut -d' ' -f1)
[ "$(sha256sum "$path" | cut -d' ' -f1)" = "$old_digest" ] || { echo '磁盘文件与运行中的核心不一致'; exit 1; }
baseline=$(probe_answers) || { echo "升级前 DNS 查询未通过：$name 没有返回 A 记录"; exit 1; }
if [ -n "$answer" ]; then
 baseline=$answer
 probe || { echo "升级前 DNS 未返回预期地址 $answer"; exit 1; }
fi
work=$(mktemp -d "${path%/*}/.smartdns-upgrade.XXXXXXXX")
changed=0
finish() {
 rc=$?
 trap - EXIT HUP INT TERM
 if [ "$changed" = 1 ]; then
  if cp -a "$work/previous" "$work/restore" && mv -f "$work/restore" "$path" && timeout 25 systemctl restart smartdns && verify "$old_digest"; then
   echo '升级失败，已恢复旧核心且 DNS 复测通过'
  else
   echo "升级失败且回退验证未通过。旧核心备份 $work/previous"
  fi
  exit 1
 fi
 rm -f "$work/candidate"
 if [ ! -e "$work/previous" ]; then rmdir "$work"; fi
 exit "$rc"
}
trap finish EXIT
trap 'exit 1' HUP INT TERM
cp -a "$path" "$work/candidate"
base64 -d > "$work/candidate" <<'SMARTDNS_ASSET'
@@SMARTDNS_ASSET@@
SMARTDNS_ASSET
printf '%s  %s\n' "$digest" "$work/candidate" | sha256sum -c - >/dev/null
timeout 5 "$work/candidate" -v 2>&1 | grep -Fq "($after)" || { echo '候选核心版本校验失败'; exit 1; }
cp -a "$path" "$work/previous"
[ "$(sha256sum "$path" | cut -d' ' -f1)" = "$old_digest" ] || { echo '核心文件已变化，请重新检测'; exit 1; }
changed=1
mv -f "$work/candidate" "$path"
timeout 25 systemctl restart smartdns
verify "$digest"
changed=0
printf '升级到 %s，DNS 验证通过。旧核心备份 %s/previous\n' "$after" "$work"
`

func (c *SSHClient) runCoreUpgrade(command string, data []byte) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	session.Stdin = strings.NewReader(strings.Replace(coreUpgradeScript, "@@SMARTDNS_ASSET@@", base64.StdEncoding.EncodeToString(data), 1))
	output, err := session.CombinedOutput("if [ \"$(id -u)\" = 0 ]; then exec " + command + "; else exec sudo -n " + command + "; fi")
	return strings.TrimSpace(string(output)), err
}
