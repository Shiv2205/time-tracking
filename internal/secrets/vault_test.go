package secrets

import (
	"bytes"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestCredentialRoundTripAndClear(t *testing.T) {
	v := New(t.TempDir())
	want := Credentials{ClientID: "test-id", ClientSecret: "secret-marker", RefreshToken: "refresh-marker", Region: "us", ExpiresAt: time.Now().UTC().Truncate(time.Second)}
	if err := v.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := v.Load()
	if err != nil || got != want {
		t.Fatalf("roundtrip: %+v %v", got, err)
	}
	if runtime.GOOS == "windows" {
		raw, _ := os.ReadFile(v.path)
		if bytes.Contains(raw, []byte(want.ClientSecret)) {
			t.Fatal("plaintext secret on Windows")
		}
	}
	want.RefreshToken = "updated"
	if err = v.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err = v.Load()
	if err != nil || got.RefreshToken != "updated" {
		t.Fatal(got, err)
	}
	if err = v.Clear(); err != nil {
		t.Fatal(err)
	}
	got, err = v.Load()
	if err != nil || got.ClientID != "" {
		t.Fatal(got, err)
	}
}
