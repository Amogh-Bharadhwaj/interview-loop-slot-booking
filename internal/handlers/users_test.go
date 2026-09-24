package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestUserCreateAndList(t *testing.T) {
	srv, _ := newTestServer(t, 90*time.Second, 3)

	body, _ := json.Marshal(map[string]string{"name": "Amogh", "email": "amogh@example.com"})
	resp, err := http.Post(srv.URL+"/users", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /users: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	listResp, err := http.Get(srv.URL + "/users")
	if err != nil {
		t.Fatalf("GET /users: %v", err)
	}
	defer listResp.Body.Close()
	var users []map[string]any
	json.NewDecoder(listResp.Body).Decode(&users)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
}

func TestUserCreateDuplicateEmailConflict(t *testing.T) {
	srv, _ := newTestServer(t, 90*time.Second, 3)

	body, _ := json.Marshal(map[string]string{"name": "Amogh", "email": "dup@example.com"})
	first, err := http.Post(srv.URL+"/users", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /users (first): %v", err)
	}
	first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("expected first create to succeed with 201, got %d", first.StatusCode)
	}

	second, err := http.Post(srv.URL+"/users", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /users (second): %v", err)
	}
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 for duplicate email, got %d", second.StatusCode)
	}
}

func TestUserCreateInvalidEmail(t *testing.T) {
	srv, _ := newTestServer(t, 90*time.Second, 3)

	body, _ := json.Marshal(map[string]string{"name": "Amogh", "email": "not-an-email"})
	resp, err := http.Post(srv.URL+"/users", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /users: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid email, got %d", resp.StatusCode)
	}
}
