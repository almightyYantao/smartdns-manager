package services

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"smartdns-manager/models"
)

func TestDNSProbe(t *testing.T) {
	for _, tc := range []struct {
		name    string
		code    dnsmessage.RCode
		address [4]byte
		want    string
	}{
		{name: "正确答案", address: [4]byte{192, 0, 2, 10}},
		{name: "错误答案", address: [4]byte{192, 0, 2, 11}, want: "答案不符"},
		{name: "域名不存在", code: dnsmessage.RCodeNameError, want: "NXDOMAIN"},
		{name: "服务错误", code: dnsmessage.RCodeServerFailure, want: "响应码 2"},
		{name: "查询超时", want: "超时"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				buf := make([]byte, 1024)
				n, addr, err := server.ReadFromUDP(buf)
				if err != nil || tc.name == "查询超时" {
					return
				}
				var reply dnsmessage.Message
				if err := reply.Unpack(buf[:n]); err != nil {
					return
				}
				reply.Response, reply.RCode = true, tc.code
				if tc.code == dnsmessage.RCodeSuccess {
					reply.Answers = []dnsmessage.Resource{{Header: dnsmessage.ResourceHeader{Name: reply.Questions[0].Name, Class: dnsmessage.ClassINET}, Body: &dnsmessage.AResource{A: tc.address}}}
				}
				if packet, err := reply.Pack(); err == nil {
					server.WriteToUDP(packet, addr)
				}
			}()
			timeout := time.Second
			if tc.name == "查询超时" {
				timeout = 100 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			result, err := probeDNS(ctx, server.LocalAddr().String(), models.TelemetryTarget{DNSName: "example.com", DNSExpectedA: "192.0.2.10"})
			server.Close()
			<-done
			if tc.want == "" {
				if err != nil || !strings.Contains(result, "192.0.2.10") {
					t.Fatalf("%q %v", result, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("预期 %s，实际 %v", tc.want, err)
			}
		})
	}
}

func TestDNSProbePersistsAlertState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.TelemetryTarget{}, &models.TelemetryResult{}); err != nil {
		t.Fatal(err)
	}
	target := models.TelemetryTarget{Type: "dns", Name: "测试", Target: "192.0.2.53", Enabled: true}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	samples := []bool{false, false, true, false, false, false, false, true, false, true, true, true}
	for i, success := range samples {
		if err := db.First(&target, target.ID).Error; err != nil {
			t.Fatal(err)
		}
		result := &models.TelemetryResult{TargetID: target.ID, CheckedAt: time.Now(), Success: success, Latency: 10}
		event, err := (&TelemetryService{db: db}).recordDNSProbe(target, result)
		want := map[int]string{5: "dns_probe_failed", 10: "dns_probe_recovered"}[i]
		if err != nil || event != want {
			t.Fatalf("第 %d 次探测，预期 %q，实际 %q，错误 %v", i, want, event, err)
		}
	}
	if err := db.First(&target, target.ID).Error; err != nil {
		t.Fatal(err)
	}
	if target.DNSAlerting || target.DNSLastSuccess == nil || target.CheckCount != len(samples) || target.SuccessCount != 5 || target.AvgLatency != 10 {
		t.Fatalf("统计不符 %+v", target)
	}
}
