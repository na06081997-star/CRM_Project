//go:build integration

package service

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/opensign"
	"github.com/na06081997-star/CRM_Project/backend/internal/opensign/opensigntest"
)

func withOpenSign(t *testing.T, svc *Service) *opensigntest.Fake {
	f := opensigntest.NewFake(t)
	svc.SetOpenSign(opensign.New(f.Config()))
	svc.files = &memFiles{objects: map[string][]byte{}}
	svc.cfg.CORSOrigins = []string{"https://maaleramot.example"}
	return f
}

func TestOpenSignContractIsSignedAndArchived(t *testing.T) {
	svc, st := setup(t)
	f := withOpenSign(t, svc)
	ctx := context.Background()
	admin := actor(t, st, "shiri@matnas.local")

	res, err := svc.CreateCoordinator(ctx, admin, CoordinatorInput{FullName: "נועה רכזת", Email: "noa@example.com", Contract: "digital", Title: "הסכם רכז/ת", Body: "סעיף 1\nסעיף 2"})
	if err != nil {
		t.Fatal(err)
	}
	// The CRM link does not change; the contract now lives in OpenSign.
	if !strings.HasPrefix(res.SignURL, "https://maaleramot.example/sign/") {
		t.Fatalf("CRM link expected, got %s", res.SignURL)
	}
	token := strings.TrimPrefix(res.SignURL, "https://maaleramot.example/sign/")
	if res.Contract.Provider != "opensign" {
		t.Fatalf("the answer must show the OpenSign contract, got %s", res.Contract.Provider)
	}
	c, _ := st.ContractByID(ctx, res.Contract.ID)
	if c.Provider != "opensign" || c.OpenSignDocID == "" {
		t.Fatalf("contract should be in OpenSign: %+v", c)
	}
	doc := f.Docs[c.OpenSignDocID]
	if doc["Name"] != "הסכם רכז/ת" || doc["RedirectUrl"] != res.SignURL+"?done=1" {
		t.Fatalf("OpenSign document: %+v", doc)
	}
	// The signature goes on the last page (the fake converter returns 2 pages).
	ph := doc["Placeholders"].([]any)[0].(map[string]any)["placeHolder"].([]any)[0].(map[string]any)
	if ph["pageNumber"].(float64) != 2 {
		t.Fatalf("signature page = %v, want 2", ph["pageNumber"])
	}

	pub, err := svc.PublicContract(ctx, token)
	if err != nil || pub.Status != "pending" || !strings.HasPrefix(pub.SignURL, "https://sign.example.com/login/") || pub.Body != "סעיף 1\nסעיף 2" {
		t.Fatalf("public contract: %+v %v", pub, err)
	}
	if _, err := svc.SignContract(ctx, token, SignInput{Name: "נועה רכזת", Agree: true, Signature: signaturePNG(t)}, ""); err == nil {
		t.Fatal("an OpenSign contract cannot be signed with the built-in pad")
	}

	// Nothing happens while the document is unsigned.
	svc.SyncOpenSign(ctx)
	if c, _ = st.ContractByID(ctx, c.ID); c.Status != domain.ContractPending {
		t.Fatalf("still pending expected, got %s", c.Status)
	}

	// The signer signs in OpenSign and comes back to the CRM page.
	f.Complete(c.OpenSignDocID, "5.6.7.8")
	pub, err = svc.PublicContract(ctx, token)
	if err != nil || pub.Status != "signed" || pub.SignURL != "" {
		t.Fatalf("after signing: %+v %v", pub, err)
	}
	c, _ = st.ContractByID(ctx, c.ID)
	if c.SignedIP != "5.6.7.8" || c.SignedName != "נועה רכזת" {
		t.Fatalf("signature details: %+v", c)
	}
	if u, _ := st.UserByID(ctx, res.User.ID); !u.IsActive {
		t.Fatal("signing must activate the account")
	}
	files, _ := st.ListAttachments(ctx, "contract", c.ID)
	if len(files) != 2 {
		t.Fatalf("signed PDF and certificate expected, got %+v", files)
	}
	for _, a := range files {
		if a.ContentType != "application/pdf" {
			t.Fatalf("archived file %+v", a)
		}
	}
	// A later sync does not archive the files twice.
	svc.SyncOpenSign(ctx)
	if again, _ := st.ListAttachments(ctx, "contract", c.ID); len(again) != 2 {
		t.Fatalf("files archived twice: %d", len(again))
	}
}

func TestOpenSignContractWordFileIsHebrewRTL(t *testing.T) {
	svc, st := setup(t)
	ctx := context.Background()
	u := actor(t, st, "yosef@matnas.local").User
	b, err := svc.contractDocument(ctx, domain.Contract{Title: "הסכם", Body: "נוסח"}, u.FullName, u.Email)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	for _, zf := range zr.File {
		if zf.Name != "word/document.xml" {
			continue
		}
		rc, _ := zf.Open()
		x, _ := io.ReadAll(rc)
		rc.Close()
		for _, want := range []string{"הסכם", "אישור וחתימה", u.FullName, u.Email, "<w:bidi/>"} {
			if !strings.Contains(string(x), want) {
				t.Errorf("document lacks %q", want)
			}
		}
		return
	}
	t.Fatal("document.xml missing")
}

func TestOpenSignDeclinedContractIsCancelled(t *testing.T) {
	svc, st := setup(t)
	f := withOpenSign(t, svc)
	ctx := context.Background()
	admin := actor(t, st, "shiri@matnas.local")
	res, err := svc.CreateCoordinator(ctx, admin, CoordinatorInput{FullName: "דן", Email: "dan@example.com", Contract: "digital", Title: "הסכם", Body: "נוסח"})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := st.ContractByID(ctx, res.Contract.ID)
	f.Decline(c.OpenSignDocID)
	svc.SyncOpenSign(ctx)
	if c, _ = st.ContractByID(ctx, c.ID); c.Status != domain.ContractCancelled {
		t.Fatalf("declined contract must be cancelled, got %s", c.Status)
	}
	if u, _ := st.UserByID(ctx, res.User.ID); u.IsActive {
		t.Fatal("a declined contract must not open the account")
	}
}

func TestOpenSignDownFallsBackToBuiltInSignature(t *testing.T) {
	svc, st := setup(t)
	f := withOpenSign(t, svc)
	f.Server.Close() // OpenSign unreachable
	ctx := context.Background()
	admin := actor(t, st, "shiri@matnas.local")
	res, err := svc.CreateCoordinator(ctx, admin, CoordinatorInput{FullName: "רות", Email: "ruth@example.com", Contract: "digital", Title: "הסכם", Body: "נוסח"})
	if err != nil {
		t.Fatalf("sending must not fail when OpenSign is down: %v", err)
	}
	c, _ := st.ContractByID(ctx, res.Contract.ID)
	if c.Provider != "builtin" {
		t.Fatalf("fallback expected, got %s", c.Provider)
	}
	token := strings.TrimPrefix(res.SignURL, "https://maaleramot.example/sign/")
	pub, err := svc.SignContract(ctx, token, SignInput{Name: "רות", Agree: true, Signature: signaturePNG(t)}, "")
	if err != nil || pub.Status != "signed" {
		t.Fatalf("built-in signature: %+v %v", pub, err)
	}
}

func TestOpenSignResendRenewsExpiry(t *testing.T) {
	svc, st := setup(t)
	f := withOpenSign(t, svc)
	ctx := context.Background()
	admin := actor(t, st, "shiri@matnas.local")
	res, err := svc.CreateCoordinator(ctx, admin, CoordinatorInput{FullName: "מיכל", Email: "michal@example.com", Contract: "digital", Title: "הסכם", Body: "נוסח"})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := st.ContractByID(ctx, res.Contract.ID)
	first := f.Docs[c.OpenSignDocID]["ExpiryDate"]
	f.Docs[c.OpenSignDocID]["ExpiryDate"] = nil
	if _, err := svc.ResendContract(ctx, admin, c.ID); err != nil {
		t.Fatal(err)
	}
	if f.Docs[c.OpenSignDocID]["ExpiryDate"] == nil || first == nil {
		t.Fatal("resending must renew the OpenSign expiry")
	}
	if len(f.Docs) != 1 {
		t.Fatalf("resending must reuse the document, got %d", len(f.Docs))
	}
}

func TestOpenSignAccountIsClaimedAtStartup(t *testing.T) {
	// A fresh OpenSign offers its admin set-up page to anyone: the CRM takes
	// the account on its first sync, without waiting for a contract.
	svc, _ := setup(t)
	f := withOpenSign(t, svc)
	svc.SyncOpenSign(context.Background())
	if !f.HasAccount() {
		t.Fatal("the CRM account must be created on the first sync")
	}
}
