// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestDo_SuccessDecodesBodyAndSetsHeaders(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization header = %q, want %q", got, "Bearer secret")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type header = %q, want application/json", got)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var in payload
		if err := json.Unmarshal(body, &in); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(payload{Name: in.Name + "-created"})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	var out payload
	if err := c.Do(context.Background(), http.MethodPost, "/api/0/organizations/", payload{Name: "acme"}, &out); err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if out.Name != "acme-created" {
		t.Errorf("out.Name = %q, want acme-created", out.Name)
	}
}

func TestDo_NotFoundReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Not found"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	err := c.Do(context.Background(), http.MethodGet, "/api/0/organizations/missing/", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsNotFound(err) {
		t.Errorf("IsNotFound(%v) = false, want true", err)
	}
}

func TestDo_ServerErrorIsNotNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	err := c.Do(context.Background(), http.MethodGet, "/api/0/", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if IsNotFound(err) {
		t.Errorf("IsNotFound(%v) = true, want false", err)
	}
}

func TestNew_TrimsTrailingSlashAndDefaults(t *testing.T) {
	c := New("https://example.com/", "tok")
	if c.baseURL != "https://example.com" {
		t.Errorf("baseURL = %q, want https://example.com", c.baseURL)
	}
	d := New("", "tok")
	if d.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %q, want %q", d.baseURL, DefaultBaseURL)
	}
}

func TestNextPagePath(t *testing.T) {
	// Exact Link header captured from a live GlitchTip 6 container (wrapped in
	// {'...'}, with results="true" on a non-final page).
	hdr := `{'<http://localhost:8123/api/0/organizations/demo-org/projects/?limit=1>; rel="previous"; results="false", <http://localhost:8123/api/0/organizations/demo-org/projects/?cursor=cD1hcGk%3D&limit=1>; rel="next"; results="true"; cursor="cD1hcGk="'}`
	got := nextPagePath(hdr)
	want := "/api/0/organizations/demo-org/projects/?cursor=cD1hcGk%3D&limit=1"
	if got != want {
		t.Fatalf("nextPagePath = %q, want %q", got, want)
	}

	// On the final page the next link has results="false" -> stop.
	last := `{'<http://localhost:8123/api/0/organizations/demo-org/projects/?cursor=cD1hcGk%3D&limit=1>; rel="next"; results="false"'}`
	if got := nextPagePath(last); got != "" {
		t.Fatalf("nextPagePath(final) = %q, want empty", got)
	}

	if got := nextPagePath(""); got != "" {
		t.Fatalf("nextPagePath(empty header) = %q, want empty", got)
	}
}

func TestListFollowsCursorPagination(t *testing.T) {
	var base string
	mux := http.NewServeMux()
	mux.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("cursor") {
		case "":
			w.Header().Set("Link", fmt.Sprintf(`{'<%[1]s/list>; rel="previous"; results="false", <%[1]s/list?cursor=c1>; rel="next"; results="true"; cursor="c1"'}`, base))
			_, _ = w.Write([]byte(`["a"]`))
		case "c1":
			w.Header().Set("Link", fmt.Sprintf(`{'<%[1]s/list?cursor=c0>; rel="previous"; results="true", <%[1]s/list?cursor=c2>; rel="next"; results="true"; cursor="c2"'}`, base))
			_, _ = w.Write([]byte(`["b"]`))
		case "c2":
			w.Header().Set("Link", fmt.Sprintf(`{'<%[1]s/list?cursor=c1>; rel="previous"; results="true", <%[1]s/list?cursor=c2>; rel="next"; results="false"'}`, base))
			_, _ = w.Write([]byte(`["c"]`))
		default:
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base = srv.URL

	got, err := List[string](context.Background(), New(srv.URL, "tok"), "/list")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("List = %v, want %v", got, want)
	}
}

func TestListEmptyReturnsNonNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Link", `{'<x>; rel="previous"; results="false", <x>; rel="next"; results="false"'}`)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	got, err := List[string](context.Background(), New(srv.URL, "tok"), "/list")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("List(empty) = %v, want non-nil empty slice", got)
	}
}
