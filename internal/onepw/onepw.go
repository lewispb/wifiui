// Package onepw integrates with the 1Password CLI (`op`) for retrieving and
// storing wifi passphrases. All operations are best-effort: if `op` is not
// installed, not signed in, or the call otherwise fails, the operation
// silently returns (false / nil) so the caller can fall back to a manual
// prompt.
package onepw

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// Available reports whether the `op` CLI is on PATH.
func Available() bool {
	_, err := exec.LookPath("op")
	return err == nil
}

// account returns the value of WIFIUI_OP_ACCOUNT, or "" when the env var is
// unset. When unset, calls omit `--account` and rely on `op`'s default
// (which works when only one account is signed in).
func account() string {
	return os.Getenv("WIFIUI_OP_ACCOUNT")
}

func opCmd(ctx context.Context, args ...string) *exec.Cmd {
	full := args
	if a := account(); a != "" {
		full = append([]string{"--account=" + a}, args...)
	}
	return exec.CommandContext(ctx, "op", full...)
}

// Lookup returns the password stored in 1Password for the given SSID, or
// ("", false) if not found / not accessible.
func Lookup(ctx context.Context, ssid string) (string, bool) {
	if !Available() {
		return "", false
	}
	out, err := opCmd(ctx, "item", "get", ssid, "--fields", "password", "--reveal").Output()
	if err != nil {
		return "", false
	}
	pass := strings.TrimSpace(string(out))
	if pass == "" {
		return "", false
	}
	return pass, true
}

// Save writes the SSID/passphrase pair to 1Password. If an item with the same
// title already exists it is updated; otherwise a new Password-category item
// is created tagged with `wifi` and `wifiui`.
func Save(ctx context.Context, ssid, password string) error {
	if !Available() {
		return errors.New("op CLI not available")
	}
	if ssid == "" || password == "" {
		return errors.New("ssid or password empty")
	}

	// Edit succeeds when an item with this title already exists and exposes
	// a `password` field (any standard category).
	if err := opCmd(ctx, "item", "edit", ssid, "password="+password).Run(); err == nil {
		return nil
	}

	// Otherwise create a new Password-category item.
	return opCmd(ctx, "item", "create",
		"--category=password",
		"--title="+ssid,
		"--tags=wifi,wifiui",
		"password="+password,
	).Run()
}
