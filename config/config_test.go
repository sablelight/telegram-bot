package config

import "testing"

func TestEnvNameDerivation(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"proxy.mode", "PROXYHTTP_MODE"},
		{"http.timeout", "PROXYHTTP_TIMEOUT"},
		{"a.b.c.d", "PROXYHTTP_D"},
		{"noDots", "PROXYHTTP_NODOTS"},
		{"dashed-key", "PROXYHTTP_DASHED_KEY"},
	}
	for _, tt := range tests {
		o := &Option{key: tt.key}
		if got := o.EnvName(); got != tt.want {
			t.Errorf("EnvName(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestGetStringPrefersEnv(t *testing.T) {
	o := &Option{key: "x.mode", deflt: "tor"}
	if got := o.GetString(); got != "tor" {
		t.Errorf("with no env, GetString() = %q, want default %q", got, "tor")
	}

	t.Setenv("PROXYHTTP_MODE", "direct")
	if got := o.GetString(); got != "direct" {
		t.Errorf("with env set, GetString() = %q, want %q", got, "direct")
	}
}

func TestGetStringEmptyEnvFallsBackToDefault(t *testing.T) {
	o := &Option{key: "x.mode", deflt: "tor"}
	t.Setenv("PROXYHTTP_MODE", "")
	if got := o.GetString(); got != "tor" {
		t.Errorf("empty env should fall back to default, got %q", got)
	}
}

func TestRegisterOptionIsIdempotent(t *testing.T) {
	a := RegisterOption("test.dup", "first", "one")
	b := RegisterOption("test.dup", "second", "two")
	if a != b {
		t.Fatal("RegisterOption returned a different *Option for the same key")
	}
	if a.deflt != "one" {
		t.Errorf("re-registering overwrote the default: got %q, want %q", a.deflt, "one")
	}
}

func TestTypedAccessors(t *testing.T) {
	o := &Option{key: "x.flag", deflt: "true"}
	if !o.GetBool() {
		t.Error("GetBool() = false, want true from default")
	}
	t.Setenv("PROXYHTTP_FLAG", "false")
	if o.GetBool() {
		t.Error("GetBool() = true, want false after env override")
	}

	// Unparseable values degrade rather than panic.
	t.Setenv("PROXYHTTP_FLAG", "not-a-bool")
	if o.GetBool() {
		t.Error("GetBool() should be false for an unparseable value")
	}

	n := &Option{key: "x.num", deflt: "42"}
	if got := n.GetInt(); got != 42 {
		t.Errorf("GetInt() = %d, want 42", got)
	}
	t.Setenv("PROXYHTTP_NUM", "abc")
	if got := n.GetInt(); got != 0 {
		t.Errorf("GetInt() = %d for unparseable value, want 0", got)
	}
}

func TestKeysIsSorted(t *testing.T) {
	RegisterOption("zzz.last", "", "")
	RegisterOption("aaa.first", "", "")

	keys := Keys()
	for i := 1; i < len(keys); i++ {
		if keys[i-1] > keys[i] {
			t.Fatalf("Keys() not sorted: %q before %q", keys[i-1], keys[i])
		}
	}
}
