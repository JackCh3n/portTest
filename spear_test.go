package main

import (
	"testing"
	"time"
)

func TestTcpingMode(t *testing.T) {
	// 测试本地已知端口
	ports := []int{80, 443, 8080}
	timeout := 3 * time.Second
	result := scanTarget("127.0.0.1", ports, timeout)

	if result.Total != len(ports) {
		t.Errorf("Total ports = %d, want %d", result.Total, len(ports))
	}
	if result.Target != "127.0.0.1" {
		t.Errorf("Target = %q, want %q", result.Target, "127.0.0.1")
	}

	for _, r := range result.Results {
		if r.Port == 0 {
			t.Errorf("Port should not be 0")
		}
		if r.Status != "open" && r.Status != "closed" {
			t.Errorf("Port %d status = %q, want open or closed", r.Port, r.Status)
		}
	}
}

func TestPingPort(t *testing.T) {
	// 测试一个已知关闭的端口
	result := pingPort("127.0.0.1", 1, 2*time.Second)
	if result.Status != "closed" {
		t.Errorf("Port 1 status = %q, want closed", result.Status)
	}

	// 测试一个不存在的主机
	result = pingPort("192.0.2.1", 80, 1*time.Second)
	if result.Status != "closed" {
		t.Errorf("Ping non-existent host status = %q, want closed", result.Status)
	}
}

func TestParsePortRange(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
	}{
		{"80", []int{80}},
		{"80,443", []int{80, 443}},
		{"80, 443, 8080", []int{80, 443, 8080}},
		{"1-5", []int{1, 2, 3, 4, 5}},
		{"", nil},
		{"invalid", nil},
	}

	for _, tt := range tests {
		ports := parsePortRange(tt.input)
		if len(ports) != len(tt.expected) {
			t.Errorf("parsePortRange(%q) = %v, want %v", tt.input, ports, tt.expected)
		}
		for i := range ports {
			if ports[i] != tt.expected[i] {
				t.Errorf("parsePortRange(%q)[%d] = %d, want %d", tt.input, i, ports[i], tt.expected[i])
			}
		}
	}
}

func TestGetServiceName(t *testing.T) {
	tests := []struct {
		port     int
		expected string
	}{
		{80, "HTTP"},
		{443, "HTTPS"},
		{3306, "MySQL"},
		{99999, "Unknown"},
	}

	for _, tt := range tests {
		name := getServiceName(tt.port)
		if name != tt.expected {
			t.Errorf("getServiceName(%d) = %q, want %q", tt.port, name, tt.expected)
		}
	}
}

func TestQuickPing(t *testing.T) {
	ports := []int{80, 443, 8080}
	success, results := QuickPing("127.0.0.1", ports, 3*time.Second)

	if success < 0 || success > len(ports) {
		t.Errorf("Success count = %d, want 0-%d", success, len(ports))
	}
	if len(results) != len(ports) {
		t.Errorf("Results count = %d, want %d", len(results), len(ports))
	}
}
