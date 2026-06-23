package config

import "testing"

func TestSecurityWarnings_DefaultsFlagged(t *testing.T) {
	c := &Config{
		JWTSecret:         defaultJWTSecret,
		EncryptionKey:     defaultEncryptionKey,
		BootstrapPassword: defaultBootstrapPassword,
	}
	if got := len(c.SecurityWarnings()); got != 3 {
		t.Fatalf("all default values should produce 3 warnings, got %d", got)
	}
}

func TestSecurityWarnings_OverriddenClean(t *testing.T) {
	c := &Config{
		JWTSecret:         "a-real-secret",
		EncryptionKey:     "another-real-32-byte-key-value!!",
		BootstrapPassword: "s3cure",
	}
	if got := c.SecurityWarnings(); len(got) != 0 {
		t.Fatalf("no warnings expected after overriding all values, got %v", got)
	}
}

func TestGetEnvBool(t *testing.T) {
	cases := map[string]bool{"1": true, "true": true, "yes": true, "on": true,
		"0": false, "false": false, "no": false, "off": false}
	for in, want := range cases {
		t.Setenv("OAH_TEST_BOOL", in)
		if got := getEnvBool("OAH_TEST_BOOL", !want); got != want {
			t.Fatalf("getEnvBool(%q) = %v, want %v", in, got, want)
		}
	}
	// Should return default when unset
	t.Setenv("OAH_TEST_BOOL", "")
	if getEnvBool("OAH_TEST_BOOL", true) != true {
		t.Fatal("empty value should fall back to default true")
	}
}
