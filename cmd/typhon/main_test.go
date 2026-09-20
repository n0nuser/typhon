package main

import "testing"

func TestEnvHelpers(t *testing.T) {
	t.Run("string falls back when unset or empty", func(t *testing.T) {
		t.Setenv("TYPHON_COLOR", "")
		if got := env("TYPHON_COLOR", "#8A0303"); got != "#8A0303" {
			t.Errorf("env = %q, want the fallback", got)
		}
		t.Setenv("TYPHON_COLOR", "#123456")
		if got := env("TYPHON_COLOR", "#8A0303"); got != "#123456" {
			t.Errorf("env = %q, want the value", got)
		}
	})

	t.Run("int falls back when unparseable", func(t *testing.T) {
		t.Setenv("TYPHON_OPPONENTS", "not a number")
		if got := envInt("TYPHON_OPPONENTS", 2); got != 2 {
			t.Errorf("envInt = %d, want the fallback", got)
		}
		t.Setenv("TYPHON_OPPONENTS", "3")
		if got := envInt("TYPHON_OPPONENTS", 2); got != 3 {
			t.Errorf("envInt = %d, want 3", got)
		}
	})

	t.Run("bool falls back when unparseable", func(t *testing.T) {
		t.Setenv("TYPHON_NO_TABLE", "")
		if got := envBool("TYPHON_NO_TABLE", false); got != false {
			t.Errorf("envBool = %v, want the fallback", got)
		}
		t.Setenv("TYPHON_NO_TABLE", "true")
		if got := envBool("TYPHON_NO_TABLE", false); got != true {
			t.Errorf("envBool = %v, want true", got)
		}
	})
}
