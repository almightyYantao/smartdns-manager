package services

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"smartdns-manager/models"
)

const DNSProbeInterval = 30 * time.Second

func ValidateDNSProbe(target models.TelemetryTarget) error {
	server, err := netip.ParseAddr(strings.TrimSpace(target.Target))
	if err != nil || server.Zone() != "" || server.IsUnspecified() || server.IsMulticast() || server.IsLinkLocalUnicast() {
		return errors.New("请输入有效的 DNS 服务器 IP 地址")
	}
	name := strings.TrimSuffix(strings.TrimSpace(target.DNSName), ".")
	if len(name) == 0 || len(name) > 253 {
		return errors.New("请输入有效的查询域名")
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' || strings.ContainsFunc(label, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-')
		}) {
			return errors.New("请输入有效的查询域名")
		}
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(target.DNSExpectedA))
	if err != nil || !ip.Is4() {
		return errors.New("请输入有效的预期 IPv4 地址")
	}
	return nil
}

func probeDNS(ctx context.Context, endpoint string, target models.TelemetryTarget) (string, error) {
	name, err := dnsmessage.NewName(strings.TrimSuffix(strings.TrimSpace(target.DNSName), ".") + ".")
	if err != nil {
		return "", err
	}
	var nonce [2]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	question := dnsmessage.Question{Name: name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}
	query := dnsmessage.Message{Header: dnsmessage.Header{ID: binary.BigEndian.Uint16(nonce[:]), RecursionDesired: true}, Questions: []dnsmessage.Question{question}}
	packet, err := query.Pack()
	if err != nil {
		return "", err
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "udp", endpoint)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return "", err
		}
	}
	if _, err := conn.Write(packet); err != nil {
		return "", err
	}
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			var netErr net.Error
			if errors.Is(ctx.Err(), context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
				return "", errors.New("DNS 查询超时")
			}
			return "", err
		}
		if n < 2 || binary.BigEndian.Uint16(buf[:2]) != query.ID {
			continue
		}
		var reply dnsmessage.Message
		if err := reply.Unpack(buf[:n]); err != nil {
			return "", fmt.Errorf("DNS 响应格式错误: %w", err)
		}
		if !reply.Response || reply.OpCode != 0 || len(reply.Questions) != 1 || !strings.EqualFold(reply.Questions[0].Name.String(), name.String()) || reply.Questions[0].Type != question.Type || reply.Questions[0].Class != question.Class {
			continue
		}
		if reply.Truncated {
			return "", errors.New("DNS UDP 响应被截断")
		}
		return checkDNSAnswer(reply, strings.ToLower(name.String()), strings.TrimSpace(target.DNSExpectedA))
	}
}

func checkDNSAnswer(reply dnsmessage.Message, name, expected string) (string, error) {
	if reply.RCode == dnsmessage.RCodeNameError {
		return "NXDOMAIN", errors.New("DNS 返回 NXDOMAIN")
	}
	if reply.RCode != dnsmessage.RCodeSuccess {
		status := fmt.Sprintf("响应码 %d", reply.RCode)
		return status, fmt.Errorf("DNS 返回%s", status)
	}
	aliases := make(map[string]string)
	for _, rr := range reply.Answers {
		if alias, ok := rr.Body.(*dnsmessage.CNAMEResource); ok && rr.Header.Class == dnsmessage.ClassINET {
			aliases[strings.ToLower(rr.Header.Name.String())] = strings.ToLower(alias.CNAME.String())
		}
	}
	for steps := 0; aliases[name] != ""; steps++ {
		if steps >= len(aliases) {
			return "NOERROR", errors.New("DNS 响应包含循环 CNAME")
		}
		name = aliases[name]
	}
	var answers []string
	for _, rr := range reply.Answers {
		if a, ok := rr.Body.(*dnsmessage.AResource); ok && rr.Header.Class == dnsmessage.ClassINET && strings.EqualFold(rr.Header.Name.String(), name) {
			answers = append(answers, netip.AddrFrom4(a.A).String())
		}
	}
	if len(answers) == 0 {
		return "NOERROR", errors.New("DNS 响应缺少 A 记录")
	}
	summary := "NOERROR · " + strings.Join(answers, ", ")
	if !slices.Contains(answers, expected) {
		return summary, fmt.Errorf("DNS 答案不符，预期包含 %s", expected)
	}
	return summary, nil
}
