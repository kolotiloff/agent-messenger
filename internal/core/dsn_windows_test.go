//go:build windows

package core

import (
	"strings"
	"testing"
)

func TestDSNKeepsTheDriveInThePath(t *testing.T) {
	got := dsn(`D:\a\_temp\data\queue.sqlite`)
	if !strings.HasPrefix(got, "file:///D:/a/_temp/data/queue.sqlite?") {
		t.Fatalf("dsn %s", got)
	}
}
