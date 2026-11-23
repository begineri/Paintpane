package nettest

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReplayAnswersFromFixturesWhateverTheHost(t *testing.T) {
	client := Replay(t, map[string]string{
		"api.example.org/v1/items?id=7": "testdata/hello.json",
	})

	resp, err := client.Get("https://api.example.org/v1/items?id=7")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	// Compare without the line ending: the checkout may turn the fixture's LF into CRLF.
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != `{"ok":true}` {
		t.Fatalf("got %d %q, want the fixture", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if resp.Request.URL.Host != "api.example.org" {
		t.Errorf("the caller should still see the original URL, got %s", resp.Request.URL)
	}
}

func TestReplayAnswers404WithoutAFixture(t *testing.T) {
	client := Replay(t, map[string]string{})
	for _, u := range []string{"https://api.example.org/v1/items?id=8", "http://other.example/"} {
		resp, err := client.Get(u)
		if err != nil {
			t.Fatalf("Get %s: %v", u, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", u, resp.StatusCode)
		}
	}
}
