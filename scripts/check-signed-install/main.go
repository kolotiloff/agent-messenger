// The release job copies this driver into the previous release's checkout.
// This exercises the updater already installed on the user's computer.
package main

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/selfupdate"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

func main() {
	if len(os.Args) != 6 {
		panic("expected channel, public key, installed program, old version, new version")
	}
	channel := updatefeed.Channel{Directory: os.Args[1], PublicKey: os.Args[2]}
	m, err := channel.Load()
	if err != nil || m.Version != os.Args[5] || m.Build <= selfupdate.Build(os.Args[4]) {
		panic(fmt.Sprintf("signed upgrade destination: %s (%v)", m.Version, err))
	}
	if runtime.GOOS == "darwin" {
		response := httptest.NewRecorder()
		channel.ServeHTTP(response, httptest.NewRequest("GET", "http://127.0.0.1/api/updates/feed.xml", nil))
		if response.Code != 200 || !strings.Contains(response.Body.String(), m.Signature) || !strings.Contains(response.Body.String(), m.SHA256+".zip") {
			panic("previous Sparkle feed rejected the signed update")
		}
		return
	}
	updater := selfupdate.Updater{Channel: channel, Executable: os.Args[3], Running: selfupdate.Build(os.Args[4])}
	result, err := updater.Apply(context.Background())
	if err != nil || result.Version != m.Version || !updater.MatchesInstalledUpdate(os.Args[3], m.Version) {
		panic(fmt.Sprintf("installed package: %s (%v)", result.State, err))
	}
}
