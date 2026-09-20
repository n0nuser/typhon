package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInfoWebhook(t *testing.T) {
	t.Parallel()

	want := info{
		APIVersion: "1",
		Author:     "n0nuser",
		Color:      "#8A0303",
		Head:       "fang",
		Tail:       "sharp",
		Version:    "0.1.0",
	}

	srv := httptest.NewServer(routes(want, slog.New(slog.DiscardHandler)))
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET / = %d, want 200 (body %q)", resp.StatusCode, body)
	}

	var got info
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode info: %v", err)
	}
	if got != want {
		t.Errorf("info = %+v, want %+v", got, want)
	}
}

func TestEnvFallsBackWhenUnset(t *testing.T) {
	t.Setenv("TYPHON_COLOR", "")
	if got := env("TYPHON_COLOR", "#8A0303"); got != "#8A0303" {
		t.Errorf("env with empty value = %q, want the fallback", got)
	}

	t.Setenv("TYPHON_COLOR", "#123456")
	if got := env("TYPHON_COLOR", "#8A0303"); got != "#123456" {
		t.Errorf("env with a value = %q, want the value", got)
	}
}
