// Package opensign drives a self-hosted OpenSign instance (Parse Server) for
// the CRM's contract signatures.
//
// OpenSign's free edition has no public API, so this client uses the same
// internal endpoints as OpenSign's own web app (Parse REST + cloud functions),
// as checked against OpenSign main of 2026-11-20. The owner accepted that
// risk (decision of 25/11/2026); docs/opensign.md explains how to roll back.
//
// Authentication: the OpenSign stack generates its master key on first start
// into a volume the CRM backend reads (no secret is typed by a person). With
// it the client creates its own OpenSign account once (password derived from
// the key) and opens sessions with Parse's /loginAs.
package opensign

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// AppID is OpenSign's fixed Parse application id.
const AppID = "opensign"

// ErrDisabled means OpenSign is not configured for this instance.
var ErrDisabled = errors.New("opensign: not configured")

// Config locates the OpenSign stack.
type Config struct {
	// BaseURL is how the backend reaches OpenSign's proxy, e.g. http://opensign
	// (private network). Its /api/app is Parse, /api/docxtopdf the converter.
	BaseURL string
	// PublicURL is the address signers open, e.g. https://sign.demo.example.com.
	PublicURL string
	// MasterKeyFile holds the master key written by the opensign-init service.
	MasterKeyFile string
	// Account the CRM uses inside OpenSign (created on first use).
	Email   string
	Name    string
	Company string
}

type Client struct {
	cfg  Config
	http *http.Client

	mu        sync.Mutex
	session   string
	userID    string
	extUserID string
	tenantID  string
}

func New(cfg Config) *Client {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	cfg.PublicURL = strings.TrimRight(cfg.PublicURL, "/")
	cfg.Email = strings.ToLower(strings.TrimSpace(cfg.Email))
	return &Client{cfg: cfg, http: &http.Client{Timeout: 3 * time.Minute}}
}

// Enabled reports whether the instance is configured and its key is readable.
func (c *Client) Enabled() bool {
	if c == nil || c.cfg.BaseURL == "" || c.cfg.PublicURL == "" || c.cfg.Email == "" {
		return false
	}
	_, err := c.masterKey()
	return err == nil
}

func (c *Client) masterKey() (string, error) {
	if c == nil || c.cfg.MasterKeyFile == "" {
		return "", ErrDisabled
	}
	b, err := os.ReadFile(c.cfg.MasterKeyFile)
	if err != nil {
		return "", fmt.Errorf("opensign: master key: %w", err)
	}
	k := strings.TrimSpace(string(b))
	if len(k) < 16 {
		return "", errors.New("opensign: master key too short")
	}
	return k, nil
}

// accountPassword is derived from the master key, so nothing else is stored.
func accountPassword(masterKey string) string {
	m := hmac.New(sha256.New, []byte(masterKey))
	m.Write([]byte("crm-opensign-account"))
	return hex.EncodeToString(m.Sum(nil))[:40]
}

// ── HTTP plumbing ───────────────────────────────────────────

// Error is an error answered by Parse ({"code":…, "error":"…"}).
type Error struct {
	Status  int
	Code    int    `json:"code"`
	Message string `json:"error"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("opensign: HTTP %d code %d: %s", e.Status, e.Code, e.Message)
}

// Parse error code for a duplicate value ("Contact already exists.").
const codeDuplicate = 137

type auth int

const (
	asMaster auth = iota
	asSession
)

func (c *Client) do(ctx context.Context, method, path string, as auth, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+"/api/app"+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("X-Parse-Application-Id", AppID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch as {
	case asMaster:
		k, err := c.masterKey()
		if err != nil {
			return err
		}
		req.Header.Set("X-Parse-Master-Key", k)
	case asSession:
		s, err := c.sessionToken(ctx)
		if err != nil {
			return err
		}
		req.Header.Set("X-Parse-Session-Token", s)
	}
	return c.send(req, out)
}

func (c *Client) send(req *http.Request, out any) error {
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("opensign: %w", err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 64<<20))
	if err != nil {
		return fmt.Errorf("opensign: %w", err)
	}
	if res.StatusCode >= 300 {
		e := &Error{Status: res.StatusCode}
		if json.Unmarshal(data, e) != nil || e.Message == "" {
			e.Message = strings.TrimSpace(string(data))
			if len(e.Message) > 300 {
				e.Message = e.Message[:300]
			}
		}
		return e
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("opensign: decode %s: %w", req.URL.Path, err)
	}
	return nil
}

// call runs a cloud function and decodes its "result".
func (c *Client) call(ctx context.Context, fn string, as auth, params, result any) error {
	var env struct {
		Result json.RawMessage `json:"result"`
	}
	if err := c.do(ctx, http.MethodPost, "/functions/"+fn, as, params, &env); err != nil {
		return err
	}
	if result == nil || len(env.Result) == 0 {
		return nil
	}
	return json.Unmarshal(env.Result, result)
}

func pointer(class, id string) map[string]string {
	return map[string]string{"__type": "Pointer", "className": class, "objectId": id}
}

func where(q map[string]any) string {
	b, _ := json.Marshal(q)
	return url.QueryEscape(string(b))
}

// ── Account & session ───────────────────────────────────────

func (c *Client) sessionToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != "" {
		return c.session, nil
	}
	if err := c.loginLocked(ctx); err != nil {
		return "", err
	}
	return c.session, nil
}

// Ready creates the CRM's OpenSign account if needed and opens a session.
// A fresh OpenSign offers its admin set-up page to anyone, so the backend
// claims it as soon as it starts.
func (c *Client) Ready(ctx context.Context) error {
	_, err := c.sessionToken(ctx)
	return err
}

// SetSessionForTest replaces the cached session (tests of session renewal).
func (c *Client) SetSessionForTest(s string) {
	c.mu.Lock()
	c.session = s
	c.mu.Unlock()
}

// resetSession forgets the cached session (expired or revoked).
func (c *Client) resetSession() {
	c.mu.Lock()
	c.session = ""
	c.mu.Unlock()
}

// loginLocked finds or creates the CRM's OpenSign account, then opens a session.
func (c *Client) loginLocked(ctx context.Context) error {
	key, err := c.masterKey()
	if err != nil {
		return err
	}
	find := func() (string, error) {
		var users struct {
			Results []struct {
				ObjectID string `json:"objectId"`
			} `json:"results"`
		}
		err := c.do(ctx, http.MethodGet, "/users?limit=1&where="+where(map[string]any{"username": c.cfg.Email}), asMaster, nil, &users)
		if err != nil || len(users.Results) == 0 {
			return "", err
		}
		return users.Results[0].ObjectID, nil
	}
	userID, err := find()
	if err != nil {
		return err
	}
	if userID == "" {
		details := map[string]any{"userDetails": map[string]any{
			"name": c.cfg.Name, "email": c.cfg.Email, "password": accountPassword(key),
			"company": c.cfg.Company, "role": "contracts_Admin", "timezone": "Asia/Jerusalem",
		}}
		if err := c.do(ctx, http.MethodPost, "/functions/addadmin", asNone, details, nil); err != nil {
			return fmt.Errorf("opensign: create account: %w", err)
		}
		if userID, err = find(); err != nil {
			return err
		}
		if userID == "" {
			return errors.New("opensign: account not found after creation")
		}
	}
	var login struct {
		SessionToken string `json:"sessionToken"`
	}
	if err := c.do(ctx, http.MethodPost, "/loginAs?userId="+url.QueryEscape(userID), asMaster, nil, &login); err != nil {
		return fmt.Errorf("opensign: session: %w", err)
	}
	if login.SessionToken == "" {
		return errors.New("opensign: empty session token")
	}
	var ext struct {
		Results []struct {
			ObjectID string `json:"objectId"`
			TenantID *struct {
				ObjectID string `json:"objectId"`
			} `json:"TenantId"`
		} `json:"results"`
	}
	q := where(map[string]any{"UserId": pointer("_User", userID)})
	if err := c.do(ctx, http.MethodGet, "/classes/contracts_Users?limit=1&where="+q, asMaster, nil, &ext); err != nil {
		return err
	}
	if len(ext.Results) == 0 {
		return errors.New("opensign: account has no OpenSign profile")
	}
	c.session, c.userID, c.extUserID = login.SessionToken, userID, ext.Results[0].ObjectID
	if t := ext.Results[0].TenantID; t != nil {
		c.tenantID = t.ObjectID
	}
	return nil
}

// asNone sends neither key nor session (public cloud functions).
const asNone auth = -1

// withSession retries once with a fresh session when OpenSign rejects it.
func (c *Client) withSession(ctx context.Context, f func() error) error {
	err := f()
	var pe *Error
	if errors.As(err, &pe) && pe.Code == 209 { // INVALID_SESSION_TOKEN
		c.resetSession()
		err = f()
	}
	return err
}

// ── Documents ───────────────────────────────────────────────

// ConvertDocx turns a .docx into a PDF stored in OpenSign and returns its URL
// (without the short-lived access token).
func (c *Client) ConvertDocx(ctx context.Context, filename string, docx []byte) (string, error) {
	var out struct {
		URL string `json:"url"`
	}
	err := c.withSession(ctx, func() error {
		s, err := c.sessionToken(ctx)
		if err != nil {
			return err
		}
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		fw, err := mw.CreateFormFile("file", filename)
		if err != nil {
			return err
		}
		if _, err := fw.Write(docx); err != nil {
			return err
		}
		if err := mw.Close(); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/api/docxtopdf", &body)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("sessiontoken", s)
		return c.send(req, &out)
	})
	if err != nil {
		return "", err
	}
	if out.URL == "" {
		return "", errors.New("opensign: conversion returned no file")
	}
	return stripToken(out.URL), nil
}

func stripToken(u string) string {
	if i := strings.IndexByte(u, '?'); i >= 0 {
		return u[:i]
	}
	return u
}

// Download fetches a file served by OpenSign. URLs carry the public origin;
// the request goes through the private BaseURL with the same path and token.
func (c *Client) Download(ctx context.Context, fileURL string) ([]byte, error) {
	u, err := url.Parse(fileURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+u.RequestURI(), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opensign: download: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opensign: download: HTTP %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 64<<20))
}

// signedFileURL adds OpenSign's short-lived access token to a stored file URL.
func (c *Client) signedFileURL(ctx context.Context, fileURL string) (string, error) {
	var out struct {
		URL string `json:"url"`
	}
	if err := c.call(ctx, "fileupload", asMaster, map[string]string{"url": stripToken(fileURL)}, &out); err != nil {
		return "", err
	}
	return out.URL, nil
}

// DownloadStored fetches a stored file (document, signed PDF, certificate).
func (c *Client) DownloadStored(ctx context.Context, fileURL string) ([]byte, error) {
	if strings.Contains(fileURL, "token=") {
		if b, err := c.Download(ctx, fileURL); err == nil {
			return b, nil
		}
	}
	u, err := c.signedFileURL(ctx, fileURL)
	if err != nil {
		return nil, err
	}
	return c.Download(ctx, u)
}

var pageRe = regexp.MustCompile(`/Type\s*/Page[^s]`)

// PageCount counts the pages of a PDF (good enough for LibreOffice output).
func PageCount(pdf []byte) int { return len(pageRe.FindAllIndex(pdf, -1)) }

// Signer is the person who signs.
type Signer struct {
	Name  string
	Email string
	Phone string
}

// Box is a widget's place on a page, in PDF points from the top-left corner.
type Box struct {
	Page                int
	X, Y, Width, Height float64
}

// DocumentInput creates a document to sign.
type DocumentInput struct {
	Name        string
	PDFURL      string // from ConvertDocx
	Signer      Signer
	Signature   Box
	ExpiresAt   time.Time
	RedirectURL string // where the signer lands after signing
}

// Sent is a document waiting for its signer.
type Sent struct {
	DocumentID string
	ContactID  string
	SignURL    string // the signer's personal link
}

// contact returns the signer's OpenSign contact, creating it when needed.
func (c *Client) contact(ctx context.Context, s Signer) (string, error) {
	email := strings.ToLower(strings.TrimSpace(s.Email))
	var created struct {
		ObjectID string `json:"objectId"`
	}
	params := map[string]any{"name": s.Name, "email": email, "phone": s.Phone}
	err := c.withSession(ctx, func() error {
		c.mu.Lock()
		if c.tenantID != "" {
			params["tenantId"] = c.tenantID
		}
		c.mu.Unlock()
		return c.call(ctx, "savecontact", asSession, params, &created)
	})
	var pe *Error
	if errors.As(err, &pe) && pe.Code == codeDuplicate {
		c.mu.Lock()
		owner := c.userID
		c.mu.Unlock()
		var found struct {
			Results []struct {
				ObjectID string `json:"objectId"`
			} `json:"results"`
		}
		q := where(map[string]any{"Email": email, "CreatedBy": pointer("_User", owner), "IsDeleted": map[string]any{"$ne": true}})
		if err := c.do(ctx, http.MethodGet, "/classes/contracts_Contactbook?limit=1&where="+q, asMaster, nil, &found); err != nil {
			return "", err
		}
		if len(found.Results) == 0 {
			return "", errors.New("opensign: existing contact not found")
		}
		return found.Results[0].ObjectID, nil
	}
	if err != nil {
		return "", fmt.Errorf("opensign: contact: %w", err)
	}
	if created.ObjectID == "" {
		return "", errors.New("opensign: contact not created")
	}
	return created.ObjectID, nil
}

// CreateDocument registers the document with one signer and a signature
// widget. OpenSign sends no e-mail: the CRM sends the link itself.
func (c *Client) CreateDocument(ctx context.Context, in DocumentInput) (Sent, error) {
	contactID, err := c.contact(ctx, in.Signer)
	if err != nil {
		return Sent{}, err
	}
	email := strings.ToLower(strings.TrimSpace(in.Signer.Email))
	signer := pointer("contracts_Contactbook", contactID)
	widget := map[string]any{
		"xPosition": in.Signature.X, "yPosition": in.Signature.Y,
		"Width": in.Signature.Width, "Height": in.Signature.Height,
		"isStamp": false, "key": time.Now().UnixMilli(), "scale": 1, "zIndex": 1,
		"type": "signature", "options": map[string]string{"name": "signature-crm-1", "status": "required"},
	}
	days := int(time.Until(in.ExpiresAt).Hours()/24 + 0.5)
	if days < 1 {
		days = 1
	}
	doc := map[string]any{
		"Name": in.Name, "URL": in.PDFURL, "SignedUrl": in.PDFURL,
		"Note": "Sent by the CRM", "Description": in.Name,
		"Signers": []any{signer},
		"Placeholders": []any{map[string]any{
			"signerPtr": signer, "signerObjId": contactID, "Role": "חותם/ת", "Id": "crm-signer-1",
			"blockColor": "#93a3db", "email": email,
			"placeHolder": []any{map[string]any{"pageNumber": in.Signature.Page, "pos": []any{widget}}},
		}},
		"SignatureType": []any{
			map[string]any{"name": "draw", "enabled": true},
			map[string]any{"name": "typed", "enabled": true},
			map[string]any{"name": "upload", "enabled": true},
		},
		"SentToOthers": true, "SendinOrder": false, "IsSendMail": false, "AutomaticReminders": false,
		"TimeToCompleteDays": days, "DocSentAt": map[string]any{"__type": "Date", "iso": time.Now().UTC().Format(time.RFC3339)},
	}
	if in.RedirectURL != "" {
		doc["RedirectUrl"] = in.RedirectURL
	}
	var created struct {
		ObjectID string `json:"objectId"`
	}
	if err := c.withSession(ctx, func() error {
		return c.call(ctx, "createdocumentfromapp", asSession, map[string]any{"document": doc}, &created)
	}); err != nil {
		return Sent{}, fmt.Errorf("opensign: create document: %w", err)
	}
	if created.ObjectID == "" {
		return Sent{}, errors.New("opensign: document not created")
	}
	if err := c.SetExpiry(ctx, created.ObjectID, in.ExpiresAt); err != nil {
		return Sent{}, err
	}
	return Sent{DocumentID: created.ObjectID, ContactID: contactID, SignURL: c.SignURL(created.ObjectID, email, contactID)}, nil
}

// SetExpiry moves the date after which the link stops working.
func (c *Client) SetExpiry(ctx context.Context, docID string, at time.Time) error {
	body := map[string]any{"ExpiryDate": map[string]any{"__type": "Date", "iso": at.UTC().Format(time.RFC3339)}}
	return c.do(ctx, http.MethodPut, "/classes/contracts_Document/"+url.PathEscape(docID), asMaster, body, nil)
}

// SignURL is the signer's link, built like OpenSign's "copy link" button.
func (c *Client) SignURL(docID, email, contactID string) string {
	raw := docID + "/" + email + "/" + contactID + "/false"
	return c.cfg.PublicURL + "/login/" + base64.StdEncoding.EncodeToString([]byte(raw))
}

// Status is the state of a document.
type Status struct {
	Completed      bool
	Declined       bool
	SignedURL      string
	CertificateURL string
	SignerIP       string
	SignedAt       *time.Time
}

// Document reads a document's state.
func (c *Client) Document(ctx context.Context, docID string) (Status, error) {
	var d struct {
		IsCompleted    bool   `json:"IsCompleted"`
		IsDeclined     bool   `json:"IsDeclined"`
		SignedURL      string `json:"SignedUrl"`
		CertificateURL string `json:"CertificateUrl"`
		AuditTrail     []struct {
			Activity  string          `json:"Activity"`
			IPAddress string          `json:"ipAddress"`
			SignedOn  json.RawMessage `json:"SignedOn"`
		} `json:"AuditTrail"`
	}
	if err := c.do(ctx, http.MethodGet, "/classes/contracts_Document/"+url.PathEscape(docID), asMaster, nil, &d); err != nil {
		return Status{}, err
	}
	st := Status{Completed: d.IsCompleted, Declined: d.IsDeclined, SignedURL: d.SignedURL, CertificateURL: d.CertificateURL}
	for _, a := range d.AuditTrail {
		if a.Activity == "Signed" {
			st.SignerIP = a.IPAddress
			if t, ok := parseDate(a.SignedOn); ok {
				st.SignedAt = &t
			}
		}
	}
	return st, nil
}

// parseDate reads a Parse date ({"__type":"Date","iso":…}) or a plain string.
func parseDate(raw json.RawMessage) (time.Time, bool) {
	var d struct {
		ISO string `json:"iso"`
	}
	var str string
	if json.Unmarshal(raw, &d) == nil && d.ISO != "" {
		str = d.ISO
	} else if json.Unmarshal(raw, &str) != nil {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, str)
	return t, err == nil
}
