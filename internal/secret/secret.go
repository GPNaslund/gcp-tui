// Package secret stores connection-profile passwords in the operating system's
// keyring (Secret Service on Linux, Keychain on macOS, Credential Manager on
// Windows) so they are never written to the config file in plaintext.
package secret

import "github.com/zalando/go-keyring"

const service = "gcp-tui"

func key(env, profile string) string {
	return env + "/" + profile
}

// Set stores a password for the env/profile pair.
func Set(env, profile, password string) error {
	return keyring.Set(service, key(env, profile), password)
}

// Get retrieves a stored password.
func Get(env, profile string) (string, error) {
	return keyring.Get(service, key(env, profile))
}

// Delete removes a stored password. A missing entry is not an error.
func Delete(env, profile string) error {
	if err := keyring.Delete(service, key(env, profile)); err != nil && err != keyring.ErrNotFound {
		return err
	}
	return nil
}
