package main

import (
	"bytes"
	"net"
	"strings"
	"testing"
)

func TestPublicIP(t *testing.T) {
	for addr, want := range map[string]bool{
		"152.89.105.48": true, "2a01:4f8::1": true,
		"10.0.0.5": false, "192.168.1.2": false, "172.16.0.1": false, "100.64.1.1": false,
		"127.0.0.1": false, "192.0.2.1": false, "fd00::1": false, "2001:db8::1": false,
	} {
		if got := publicIP(net.ParseIP(addr)); got != want {
			t.Errorf("publicIP(%s) = %t, want %t", addr, got, want)
		}
	}
}

func TestServerCommandArguments(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{}, {"nope"}, {"run", "x"}, {"status", "x"}, {"disable", "x"},
		{"enable", "--internet", "--lan"}, {"enable", "--lan", "--public", "1.2.3.4"}, {"enable", "extra"},
		{"revoke"}, {"invite", "extra"},
	} {
		var out bytes.Buffer
		if err := runServerCommand(root, args, &out); err == nil {
			t.Errorf("server %s: no error", strings.Join(args, " "))
		}
	}
}
