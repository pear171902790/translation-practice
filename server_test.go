package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddAndPracticeFlow(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/practice", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "还没有句子") {
		t.Fatalf("empty practice: %d %s", rr.Code, rr.Body.String())
	}

	form := url.Values{"chinese": {"  今天天气很好。  "}, "english": {"  The weather is nice today.  "}}
	req = httptest.NewRequest(http.MethodPost, "/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/add?saved=1" {
		t.Fatalf("save redirect: %d %s", rr.Code, rr.Header().Get("Location"))
	}

	req = httptest.NewRequest(http.MethodGet, "/add?saved=1", nil)
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "已保存这句。") || !strings.Contains(body, "今天天气很好。") {
		t.Fatalf("saved page missing sentence: %s", body)
	}
	if strings.Contains(body, "<script>") {
		t.Fatal("unexpected script")
	}

	req = httptest.NewRequest(http.MethodGet, "/practice", nil)
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	body = rr.Body.String()
	if rr.Code != http.StatusOK || !strings.Contains(body, "今天天气很好。") {
		t.Fatalf("practice form: %d %s", rr.Code, body)
	}
	if strings.Contains(body, "The weather is nice today.") {
		t.Fatal("practice form leaked the reference translation")
	}

	form = url.Values{"id": {"1"}, "attempt": {"the   WEATHER is nice today."}}
	req = httptest.NewRequest(http.MethodPost, "/practice", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	body = rr.Body.String()
	if rr.Code != http.StatusOK {
		t.Fatalf("check status %d", rr.Code)
	}
	if !strings.Contains(body, "the   WEATHER is nice today.") || !strings.Contains(body, "The weather is nice today.") {
		t.Fatalf("comparison missing answers: %s", body)
	}
	if !strings.Contains(body, "和参考译文一致") {
		t.Fatalf("expected match badge: %s", body)
	}

	form = url.Values{"id": {"1"}, "attempt": {"It is sunny."}}
	req = httptest.NewRequest(http.MethodPost, "/practice", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	body = rr.Body.String()
	if !strings.Contains(body, "It is sunny.") || !strings.Contains(body, "The weather is nice today.") {
		t.Fatalf("mismatch comparison: %s", body)
	}
	if strings.Contains(body, "和参考译文一致") {
		t.Fatal("different answer was treated as a match")
	}
}

func TestAddRejectsBlankAndEscapesHTML(t *testing.T) {
	srv := newTestServer(t)

	form := url.Values{"chinese": {"<script>alert(1)</script>"}, "english": {"Hi"}}
	req := httptest.NewRequest(http.MethodPost, "/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatal(rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/practice", nil)
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if strings.Contains(body, "<script>alert") {
		t.Fatal("script was not escaped")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("escaped chinese missing: %s", body)
	}

	form = url.Values{"chinese": {"  "}, "english": {"Hi"}}
	req = httptest.NewRequest(http.MethodPost, "/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "请把中文和英文都填上。") {
		t.Fatalf("blank add: %d %s", rr.Code, rr.Body.String())
	}
}

func TestBlankAttemptRevealsReference(t *testing.T) {
	srv := newTestServer(t)

	form := url.Values{"chinese": {"今天天气很好。"}, "english": {"The weather is nice today."}}
	req := httptest.NewRequest(http.MethodPost, "/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("save: %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/practice", nil)
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if strings.Contains(rr.Body.String(), "required") {
		t.Fatal("empty answer is still blocked before submit")
	}

	for _, attempt := range []string{"", " \n\t "} {
		form = url.Values{"id": {"1"}, "attempt": {attempt}}
		req = httptest.NewRequest(http.MethodPost, "/practice", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr = httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		body := rr.Body.String()
		if rr.Code != http.StatusOK {
			t.Fatalf("blank check %q: %d %s", attempt, rr.Code, body)
		}
		if !strings.Contains(body, "The weather is nice today.") {
			t.Fatalf("reference hidden for %q: %s", attempt, body)
		}
		if strings.Contains(body, "请先写出你的英文译文。") {
			t.Fatalf("blank attempt still rejected: %s", body)
		}
		if strings.Contains(body, "和参考译文一致") || strings.Contains(body, "对照一下用词和语序") {
			t.Fatalf("blank attempt was scored like a written answer: %s", body)
		}
		if !strings.Contains(body, "还不会，先看参考译文。") {
			t.Fatalf("missing skip note for %q: %s", attempt, body)
		}
	}
}

func TestNormalizeAnswer(t *testing.T) {
	if normalizeAnswer("  Hello,   WORLD ") != "hello, world" {
		t.Fatal(normalizeAnswer("  Hello,   WORLD "))
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "sentences.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	srv, err := NewServer(store)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}
