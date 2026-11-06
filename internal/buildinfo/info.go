package buildinfo

import "runtime"

// Set by the packaging build. Development builds remain visibly unversioned.
var Version = "dev"
var Revision = "unknown"

// Public update verification key; the signing key never enters a build.
var UpdatePublicKey = ""

type Info struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Go       string `json:"go"`
}

func Current() Info { return Info{Version, Revision, runtime.GOOS, runtime.GOARCH, runtime.Version()} }
