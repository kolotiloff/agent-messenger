//go:build darwin

package platform

import (
	"os"
	"testing"
)

func TestObservationWithoutShellPATH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	obs, err := ReadObservation(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	owned, err := OwnsObservation(obs)
	if err != nil || !owned {
		t.Fatalf("observation without shell PATH = %t, %v", owned, err)
	}
}
