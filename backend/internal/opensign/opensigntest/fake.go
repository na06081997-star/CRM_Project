// Package opensigntest provides an in-memory OpenSign for tests.
package opensigntest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/na06081997-star/CRM_Project/backend/internal/opensign"
)

// Fake is an in-memory OpenSign for tests. It checks the headers and payloads
// the real server needs and records what the client sent.
type Fake struct {
	Server  *httptest.Server
	Key     string
	KeyFile string

	mu        sync.Mutex
	users     map[string]string // username → objectId
	contacts  map[string]string // email → objectId
	Docs      map[string]map[string]any
	Files     map[string][]byte // path → content
	Converted int
	// PDF returned by the converter (defaults to a 2-page PDF).
	PDF []byte
}

// NewFake starts a fake OpenSign and writes its master key to a temp file.
func NewFake(t testing.TB) *Fake {
	t.Helper()
	f := &Fake{Key: "fake-master-key-0123456789", users: map[string]string{}, contacts: map[string]string{},
		Docs: map[string]map[string]any{}, Files: map[string][]byte{},
		PDF: []byte("%PDF-1.4\n1 0 obj << /Type /Pages /Count 2 >>\n2 0 obj << /Type /Page >>\n3 0 obj << /Type /Page >>\n%%EOF")}
	f.KeyFile = filepath.Join(t.TempDir(), "master_key")
	if err := os.WriteFile(f.KeyFile, []byte(f.Key+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Server.Close)
	return f
}

// Config returns a client configuration pointing at the fake.
func (f *Fake) Config() opensign.Config {
	return opensign.Config{BaseURL: f.Server.URL, PublicURL: "https://sign.example.com", MasterKeyFile: f.KeyFile,
		Email: "crm@example.com", Name: "CRM", Company: "Test"}
}

// Complete marks a document signed, as OpenSign does after the signature.
func (f *Fake) Complete(docID, ip string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.Docs[docID]
	d["IsCompleted"] = true
	d["SignedUrl"] = "https://sign.example.com/api/app/files/opensign/signed_" + docID + ".pdf"
	d["CertificateUrl"] = "https://sign.example.com/api/app/files/opensign/cert_" + docID + ".pdf"
	d["AuditTrail"] = []any{map[string]any{"Activity": "Signed", "ipAddress": ip,
		"SignedOn": map[string]any{"__type": "Date", "iso": "2026-11-26T10:00:00.000Z"}}}
	f.Files["/api/app/files/opensign/signed_"+docID+".pdf"] = []byte("%PDF-1.7 signed " + docID)
	f.Files["/api/app/files/opensign/cert_"+docID+".pdf"] = []byte("%PDF-1.7 certificate " + docID)
}

// Decline marks a document declined by its signer.
func (f *Fake) Decline(docID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Docs[docID]["IsDeclined"] = true
}

func (f *Fake) json(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := r.URL.Path
	if r.Method == http.MethodGet && strings.HasPrefix(p, "/api/app/files/") {
		if r.URL.Query().Get("token") == "" {
			f.json(w, 400, map[string]string{"message": "unauthorized"})
			return
		}
		b, ok := f.Files[p]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b) //nolint:errcheck
		return
	}
	master := r.Header.Get("X-Parse-Master-Key") == f.Key
	session := r.Header.Get("X-Parse-Session-Token") == "session-1" || r.Header.Get("sessiontoken") == "session-1"
	if p != "/api/docxtopdf" && r.Header.Get("X-Parse-Application-Id") != opensign.AppID {
		f.json(w, 403, map[string]any{"code": 0, "error": "unauthorized"})
		return
	}
	var body map[string]any
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
	}
	needMaster := func() bool {
		if !master {
			f.json(w, 403, map[string]any{"code": 119, "error": "master key required"})
		}
		return master
	}
	needSession := func() bool {
		if !session {
			f.json(w, 400, map[string]any{"code": 209, "error": "Invalid session token"})
		}
		return session
	}
	result := func(v any) { f.json(w, 200, map[string]any{"result": v}) }
	switch {
	case p == "/api/app/users" && r.Method == http.MethodGet:
		if !needMaster() {
			return
		}
		var q map[string]any
		json.Unmarshal([]byte(r.URL.Query().Get("where")), &q) //nolint:errcheck
		res := []any{}
		if id, ok := f.users[fmt.Sprint(q["username"])]; ok {
			res = append(res, map[string]string{"objectId": id})
		}
		f.json(w, 200, map[string]any{"results": res})
	case p == "/api/app/functions/addadmin":
		d, _ := body["userDetails"].(map[string]any)
		if d["password"] == "" || d["email"] == "" || d["role"] != "contracts_Admin" {
			f.json(w, 400, map[string]any{"code": 141, "error": "bad user details"})
			return
		}
		f.users[fmt.Sprint(d["email"])] = "user-1"
		result(map[string]string{"message": "User sign up"})
	case p == "/api/app/loginAs":
		if !needMaster() {
			return
		}
		if r.URL.Query().Get("userId") != "user-1" {
			f.json(w, 404, map[string]any{"code": 101, "error": "user not found"})
			return
		}
		f.json(w, 200, map[string]any{"objectId": "user-1", "sessionToken": "session-1"})
	case p == "/api/app/classes/contracts_Users":
		if !needMaster() {
			return
		}
		f.json(w, 200, map[string]any{"results": []any{map[string]any{"objectId": "ext-1", "TenantId": map[string]string{"objectId": "tenant-1"}}}})
	case p == "/api/docxtopdf":
		if !needSession() {
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			f.json(w, 400, map[string]string{"error": "No file uploaded."})
			return
		}
		b, _ := io.ReadAll(file)
		if len(b) < 4 || string(b[:2]) != "PK" {
			f.json(w, 400, map[string]string{"error": "not a docx"})
			return
		}
		f.Converted++
		path := fmt.Sprintf("/api/app/files/opensign/contract%d.pdf", f.Converted)
		f.Files[path] = f.PDF
		f.json(w, 200, map[string]string{"message": "success.", "url": "https://sign.example.com" + path + "?token=t"})
	case p == "/api/app/functions/fileupload":
		if !needMaster() {
			return
		}
		result(map[string]string{"url": fmt.Sprint(body["url"]) + "?token=signed"})
	case p == "/api/app/functions/savecontact":
		if !needSession() {
			return
		}
		email := fmt.Sprint(body["email"])
		if _, ok := f.contacts[email]; ok {
			f.json(w, 400, map[string]any{"code": 137, "error": "Contact already exists."})
			return
		}
		id := fmt.Sprintf("contact-%d", len(f.contacts)+1)
		f.contacts[email] = id
		result(map[string]any{"objectId": id, "Email": email, "Name": body["name"]})
	case p == "/api/app/classes/contracts_Contactbook":
		if !needMaster() {
			return
		}
		var q map[string]any
		json.Unmarshal([]byte(r.URL.Query().Get("where")), &q) //nolint:errcheck
		res := []any{}
		if id, ok := f.contacts[fmt.Sprint(q["Email"])]; ok {
			res = append(res, map[string]string{"objectId": id})
		}
		f.json(w, 200, map[string]any{"results": res})
	case p == "/api/app/functions/createdocumentfromapp":
		if !needSession() {
			return
		}
		doc, _ := body["document"].(map[string]any)
		if doc == nil || doc["URL"] == nil || doc["Signers"] == nil || doc["Placeholders"] == nil {
			f.json(w, 400, map[string]any{"code": 107, "error": "Missing document payload."})
			return
		}
		id := fmt.Sprintf("doc-%d", len(f.Docs)+1)
		doc["objectId"] = id
		f.Docs[id] = doc
		result(map[string]any{"objectId": id, "className": "contracts_Document"})
	case strings.HasPrefix(p, "/api/app/classes/contracts_Document/"):
		if !needMaster() {
			return
		}
		id := strings.TrimPrefix(p, "/api/app/classes/contracts_Document/")
		d, ok := f.Docs[id]
		if !ok {
			f.json(w, 404, map[string]any{"code": 101, "error": "Object not found."})
			return
		}
		if r.Method == http.MethodPut {
			for k, v := range body {
				d[k] = v
			}
			f.json(w, 200, map[string]string{"updatedAt": "2026-11-26T00:00:00.000Z"})
			return
		}
		f.json(w, 200, d)
	default:
		f.json(w, 404, map[string]any{"code": 141, "error": "unknown route " + r.Method + " " + p})
	}
}
