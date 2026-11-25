package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/na06081997-star/CRM_Project/backend/internal/docx"
	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/opensign"
)

// Contracts signed through OpenSign (docs/opensign.md).
//
// The CRM keeps its own link (/sign/<token>), so e-mails, "copy link" and
// "send again" do not change. When OpenSign is on, sending a digital contract
// turns it into a PDF in OpenSign with a signature box on a last "approval
// and signature" page; the CRM page shows the text and sends the signer to
// OpenSign. Once OpenSign reports the document completed, the signed PDF and
// OpenSign's signature certificate are archived with the contract (Garage)
// and the account opens, exactly as with the built-in signature.
//
// If OpenSign is not configured or fails while sending, the contract falls
// back to the built-in signature, so a contract can always be signed.

// signatureBox is where the signer signs on the last page (PDF points from
// the top-left corner): under the lines of the approval page, on the right
// (the start of a Hebrew line).
func signatureBox(pages int) opensign.Box {
	const w, h = 200.0, 70.0
	return opensign.Box{Page: pages, X: docx.PageWidthPt - docx.MarginPt - w, Y: 280, Width: w, Height: h}
}

func (s *Service) openSignOn() bool { return s.sign != nil && s.sign.Enabled() }

// OpenSignEnabled reports whether digital contracts go through OpenSign.
func (s *Service) OpenSignEnabled() bool { return s.openSignOn() }

// contractDocument is the contract as a Word file, with its approval page.
func (s *Service) contractDocument(ctx context.Context, c domain.Contract, name, email string) ([]byte, error) {
	org, _ := s.store.Organization(ctx)
	doc := docx.Document{Title: c.Title, Body: c.Body, Pages: []docx.Page{{
		Heading: "אישור וחתימה",
		Lines: []string{
			"הצדדים: " + org.Name + " ו" + name,
			"שם החותם/ת: " + name,
			"דוא״ל: " + email,
			"נשלח לחתימה: " + s.now().In(s.loc).Format("02/01/2006"),
			"בחתימתי אני מאשר/ת שקראתי את החוזה ואני מסכים/ה לתנאיו.",
			"חתימה:",
		},
	}}}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sendToOpenSign creates the contract's OpenSign document.
func (s *Service) sendToOpenSign(ctx context.Context, c domain.Contract, u domain.User, expires time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	file, err := s.contractDocument(ctx, c, u.FullName, u.Email)
	if err != nil {
		return err
	}
	pdfURL, err := s.sign.ConvertDocx(ctx, "contract.docx", file)
	if err != nil {
		return err
	}
	pdf, err := s.sign.DownloadStored(ctx, pdfURL)
	if err != nil {
		return err
	}
	pages := opensign.PageCount(pdf)
	if pages < 1 {
		return errors.New("opensign: converted contract has no page")
	}
	phone := ""
	if u.Phone != nil {
		phone = *u.Phone
	}
	sent, err := s.sign.CreateDocument(ctx, opensign.DocumentInput{
		Name: c.Title, PDFURL: pdfURL,
		Signer:      opensign.Signer{Name: u.FullName, Email: u.Email, Phone: phone},
		Signature:   signatureBox(pages),
		ExpiresAt:   expires,
		RedirectURL: s.signURL(c.Token) + "?done=1",
	})
	if err != nil {
		return err
	}
	return s.store.SetContractOpenSign(ctx, c.ID, sent.DocumentID, sent.SignURL)
}

// prepareSignature sends (or renews) the OpenSign side of a digital contract.
// Errors are logged: the built-in signature stays available.
func (s *Service) prepareSignature(ctx context.Context, c domain.Contract, u domain.User, expires time.Time) {
	if !s.openSignOn() || c.Method != "digital" {
		return
	}
	if c.OpenSignDocID != "" {
		if err := s.sign.SetExpiry(ctx, c.OpenSignDocID, expires); err != nil {
			slog.Warn("opensign: renew expiry", "contract", c.ID, "err", err)
		}
		return
	}
	if err := s.sendToOpenSign(ctx, c, u, expires); err != nil {
		slog.Error("opensign: send failed, built-in signature kept", "contract", c.ID, "err", err)
	}
}

// SyncOpenSign checks every contract waiting in OpenSign (scheduler).
func (s *Service) SyncOpenSign(ctx context.Context) {
	if !s.openSignOn() {
		return
	}
	if err := s.sign.Ready(ctx); err != nil {
		slog.Warn("opensign: not ready", "err", err)
		return
	}
	list, err := s.store.PendingOpenSignContracts(ctx)
	if err != nil {
		slog.Error("opensign: pending contracts", "err", err)
		return
	}
	for _, c := range list {
		if err := s.syncContract(ctx, c); err != nil {
			slog.Warn("opensign: sync", "contract", c.ID, "err", err)
		}
	}
}

// syncContract applies OpenSign's state to a pending contract: signed →
// archive the PDFs and open the account; declined → cancel and tell managers.
func (s *Service) syncContract(ctx context.Context, c domain.Contract) error {
	if !s.openSignOn() || c.OpenSignDocID == "" || c.Status != domain.ContractPending {
		return nil
	}
	s.signSync.Lock()
	defer s.signSync.Unlock()
	c, err := s.store.ContractByID(ctx, c.ID) // another sync may have finished it
	if err != nil || c.Status != domain.ContractPending {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	st, err := s.sign.Document(ctx, c.OpenSignDocID)
	if err != nil {
		return err
	}
	switch {
	case st.Completed:
		files := []struct{ url, name string }{
			{st.SignedURL, c.Title + " - חתום.pdf"},
			{st.CertificateURL, "אישור חתימה דיגיטלית.pdf"},
		}
		for _, f := range files {
			if f.url == "" {
				continue
			}
			b, err := s.sign.DownloadStored(ctx, f.url)
			if err != nil {
				return fmt.Errorf("download %s: %w", f.name, err)
			}
			if _, err := s.storeFile(ctx, c.UserID, "contract", c.ID, f.name, "application/pdf", bytes.NewReader(b), int64(len(b)), nil); err != nil {
				return fmt.Errorf("archive %s: %w", f.name, err)
			}
		}
		if err := s.completeContract(ctx, c, c.UserName, st.SignerIP); err != nil {
			return err
		}
		s.systemAudit(ctx, c, "opensign_signed", map[string]any{"document": c.OpenSignDocID})
	case st.Declined:
		if err := s.store.CancelContract(ctx, c.ID); err != nil {
			return err
		}
		s.systemAudit(ctx, c, "opensign_declined", map[string]any{"document": c.OpenSignDocID})
		s.notifyManagers(ctx, "חוזה נדחה", fmt.Sprintf("%s סירב/ה לחתום על \"%s\". החוזה בוטל; אפשר לשלוח חוזה חדש ממסך המשתמש.", c.UserName, c.Title))
	}
	return nil
}

// systemAudit records an action done by the system (no logged-in actor).
func (s *Service) systemAudit(ctx context.Context, c domain.Contract, action string, data any) {
	if err := s.store.Audit(ctx, nil, "contract", &c.ID, action, data); err != nil {
		slog.Error("audit write failed", "entity", "contract", "action", action, "err", err)
	}
}
