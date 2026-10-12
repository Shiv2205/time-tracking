package secrets

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Credentials struct {
	ClientID     string    `json:"clientId"`
	ClientSecret string    `json:"clientSecret"`
	Region       string    `json:"region"`
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}
type Vault struct{ path string }

func New(dir string) *Vault { return &Vault{path: filepath.Join(dir, "credentials.bin")} }
func (v *Vault) Load() (Credentials, error) {
	var c Credentials
	raw, err := os.ReadFile(v.path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	raw, err = unprotect(raw)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(raw, &c)
	return c, err
}
func (v *Vault) Save(c Credentials) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	raw, err = protect(raw)
	if err != nil {
		return err
	}
	temp := v.path + ".tmp"
	if err = os.WriteFile(temp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(temp, v.path)
}
func (v *Vault) Clear() error {
	err := os.Remove(v.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
