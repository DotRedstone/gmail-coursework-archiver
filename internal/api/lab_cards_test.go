package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DotRedstone/gmail-coursework-archiver/internal/config"
	"github.com/DotRedstone/gmail-coursework-archiver/internal/db"
	"github.com/DotRedstone/gmail-coursework-archiver/internal/rule"
	"github.com/DotRedstone/gmail-coursework-archiver/internal/storage"
)

func TestLabCardDraftAndManualPublication(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "rosters"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "rosters", "course.csv"), []byte("学号,姓名,班级\n990000000001,示例甲,示例班A\n"), 0600); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	files, err := storage.New(root)
	if err != nil {
		t.Fatal(err)
	}
	rules := rule.NewEngine(root)
	handler := NewServer(&config.Config{APIKey: "synthetic-secret", DataDir: root}, database, files, nil, rules).Routes()
	content := []byte("%PDF-1.4\nsynthetic lab card\n")
	upload := func(key string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		file, err := writer.CreateFormFile("file", "实验卡1.pdf")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(content); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v3/lab-cards", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		if key != "" {
			req.Header.Set("X-API-Key", key)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	request := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("X-API-Key", "synthetic-secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	if rec := upload(""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated upload: %d", rec.Code)
	}
	if rec := upload("synthetic-secret"); rec.Code != http.StatusCreated {
		t.Fatalf("draft upload: %d: %s", rec.Code, rec.Body.String())
	}
	if rec := request(http.MethodGet, "/api/v3/lab-cards/1/download"); rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), content) {
		t.Fatalf("card download: %d", rec.Code)
	}
	conflictPath := filepath.Join(root, "rosters", "conflict.csv")
	if err := os.WriteFile(conflictPath, []byte("学号,姓名,班级\n990000000001,示例乙,示例班A\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if rec := request(http.MethodPost, "/api/v3/lab-cards/1/confirm"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("conflicting roster allowed publication: %d", rec.Code)
	}
	if err := os.WriteFile(conflictPath, []byte("学号,姓名,班级\n990000000001,示例甲,示例班A\n"), 0600); err != nil {
		t.Fatal(err)
	}
	confirmed := request(http.MethodPost, "/api/v3/lab-cards/1/confirm")
	if confirmed.Code != http.StatusOK {
		t.Fatalf("confirmation: %d: %s", confirmed.Code, confirmed.Body.String())
	}
	var outcome struct {
		AssignmentID   string `json:"assignment_id"`
		DeliveryStatus string `json:"delivery_status"`
	}
	if err := json.Unmarshal(confirmed.Body.Bytes(), &outcome); err != nil || outcome.AssignmentID != "experiment_1" || outcome.DeliveryStatus != "manual_pending" {
		t.Fatalf("wrong confirmation outcome: %s", confirmed.Body.String())
	}
	if _, ok := rules.GetRule("experiment_1"); !ok {
		t.Fatal("confirmed card did not create assignment")
	}
	if rec := request(http.MethodGet, "/api/v3/lab-cards/1"); rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"delivery_status":"manual_pending"`)) {
		t.Fatalf("manual delivery state was not persisted: %d: %s", rec.Code, rec.Body.String())
	}
	if rec := request(http.MethodPost, "/api/v3/lab-cards/1/confirm"); rec.Code != http.StatusOK {
		t.Fatalf("repeat confirmation: %d", rec.Code)
	}
	if rec := upload("synthetic-secret"); rec.Code != http.StatusConflict {
		t.Fatalf("confirmed card was overwritten: %d", rec.Code)
	}
}

func TestLabCardFormatValidation(t *testing.T) {
	if !validLabCard("实验卡1.pdf", []byte("%PDF-1.4\nsynthetic")) || validLabCard("实验卡1.pdf", []byte("not a pdf")) {
		t.Fatal("PDF signature validation failed")
	}
	if !validLabCard("实验卡1.doc", []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) {
		t.Fatal("Word signature validation failed")
	}
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	for _, name := range []string{"[Content_Types].xml", "word/document.xml"} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte("synthetic")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if !validLabCard("实验卡1.docx", data.Bytes()) || validLabCard("实验卡1.docx", []byte("not a zip")) {
		t.Fatal("DOCX container validation failed")
	}
}

func TestSuggestedLabDeadline(t *testing.T) {
	zone := time.FixedZone("Asia/Shanghai", 8*60*60)
	uploaded := time.Date(2030, 10, 17, 10, 0, 0, 0, zone)
	deadline := suggestedLabDeadline(uploaded)
	if deadline.In(zone).Weekday() != time.Wednesday || deadline.In(zone).Hour() != 18 || deadline.Sub(uploaded) != 6*24*time.Hour+8*time.Hour {
		t.Fatalf("unexpected deadline: %s", deadline)
	}
}
