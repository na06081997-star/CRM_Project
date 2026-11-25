package opensign_test

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/na06081997-star/CRM_Project/backend/internal/opensign"
	"github.com/na06081997-star/CRM_Project/backend/internal/opensign/opensigntest"
)

func TestDisabledWithoutConfigOrKey(t *testing.T) {
	if (*opensign.Client)(nil).Enabled() {
		t.Fatal("nil client must be disabled")
	}
	if opensign.New(opensign.Config{}).Enabled() {
		t.Fatal("empty config must be disabled")
	}
	c := opensign.New(opensign.Config{BaseURL: "http://x", PublicURL: "https://y", Email: "a@b.c", MasterKeyFile: "/nonexistent/key"})
	if c.Enabled() {
		t.Fatal("missing key file must disable OpenSign")
	}
}

func TestFullSigningRoundTrip(t *testing.T) {
	ctx := context.Background()
	f := opensigntest.NewFake(t)
	c := opensign.New(f.Config())
	if !c.Enabled() {
		t.Fatal("client should be enabled")
	}

	// The account is created on first use, then reused.
	pdfURL, err := c.ConvertDocx(ctx, "contract.docx", []byte("PK\x03\x04 fake docx"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(pdfURL, "token=") || !strings.HasPrefix(pdfURL, "https://sign.example.com/api/app/files/") {
		t.Fatalf("stored URL must be the bare public file URL, got %s", pdfURL)
	}
	pdf, err := c.DownloadStored(ctx, pdfURL)
	if err != nil {
		t.Fatal(err)
	}
	if opensign.PageCount(pdf) != 2 {
		t.Fatalf("pages = %d, want 2", opensign.PageCount(pdf))
	}

	exp := time.Now().Add(14 * 24 * time.Hour)
	sent, err := c.CreateDocument(ctx, opensign.DocumentInput{
		Name: "הסכם", PDFURL: pdfURL, Signer: opensign.Signer{Name: "נועה", Email: " Noa@Example.com "},
		Signature: opensign.Box{Page: 2, X: 323, Y: 240, Width: 200, Height: 70}, ExpiresAt: exp,
		RedirectURL: "https://crm.example.com/sign/tok?done=1",
	})
	if err != nil {
		t.Fatal(err)
	}
	doc := f.Docs[sent.DocumentID]
	if doc["IsSendMail"] != false || doc["SentToOthers"] != true || doc["RedirectUrl"] != "https://crm.example.com/sign/tok?done=1" {
		t.Fatalf("document flags: %+v", doc)
	}
	if doc["ExpiryDate"] == nil {
		t.Fatal("expiry date must be set")
	}
	ph := doc["Placeholders"].([]any)[0].(map[string]any)
	pos := ph["placeHolder"].([]any)[0].(map[string]any)
	if pos["pageNumber"].(float64) != 2 || ph["signerObjId"] != sent.ContactID {
		t.Fatalf("placeholder: %+v", ph)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(sent.SignURL, "https://sign.example.com/login/"))
	if err != nil || string(raw) != sent.DocumentID+"/noa@example.com/"+sent.ContactID+"/false" {
		t.Fatalf("sign url %s → %q %v", sent.SignURL, raw, err)
	}

	// A second document for the same person reuses the contact.
	again, err := c.CreateDocument(ctx, opensign.DocumentInput{Name: "נספח", PDFURL: pdfURL, Signer: opensign.Signer{Name: "נועה", Email: "noa@example.com"},
		Signature: opensign.Box{Page: 1, X: 1, Y: 1, Width: 1, Height: 1}, ExpiresAt: exp})
	if err != nil || again.ContactID != sent.ContactID {
		t.Fatalf("contact reuse: %+v %v", again, err)
	}

	st, err := c.Document(ctx, sent.DocumentID)
	if err != nil || st.Completed {
		t.Fatalf("pending status: %+v %v", st, err)
	}
	f.Complete(sent.DocumentID, "5.6.7.8")
	st, err = c.Document(ctx, sent.DocumentID)
	if err != nil || !st.Completed || st.SignerIP != "5.6.7.8" || st.SignedAt == nil {
		t.Fatalf("completed status: %+v %v", st, err)
	}
	signed, err := c.DownloadStored(ctx, st.SignedURL)
	if err != nil || !strings.Contains(string(signed), "signed "+sent.DocumentID) {
		t.Fatalf("signed pdf: %q %v", signed, err)
	}
}

func TestSessionIsRenewedWhenRejected(t *testing.T) {
	ctx := context.Background()
	f := opensigntest.NewFake(t)
	c := opensign.New(f.Config())
	if _, err := c.ConvertDocx(ctx, "a.docx", []byte("PK..")); err != nil {
		t.Fatal(err)
	}
	c.SetSessionForTest("expired")
	if _, err := c.ConvertDocx(ctx, "b.docx", []byte("PK..")); err != nil {
		t.Fatalf("expired session should be renewed: %v", err)
	}
}
