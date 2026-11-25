package api

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/na06081997-star/CRM_Project/backend/internal/xlsx"
)

const xlsxContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// exportName builds "<prefix>_<from>_<to>.xlsx" (or with today's date when
// the period is open).
func exportName(prefix string, from, to time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	if from.IsZero() || to.IsZero() {
		return fmt.Sprintf("%s_%s.xlsx", prefix, time.Now().In(loc).Format("2006-01-02"))
	}
	// "to" is exclusive in the filters: the last day shown is the day before.
	last := to.In(loc).Add(-time.Nanosecond)
	return fmt.Sprintf("%s_%s_%s.xlsx", prefix, from.In(loc).Format("2006-01-02"), last.Format("2006-01-02"))
}

func writeXLSX(w http.ResponseWriter, r *http.Request, wb xlsx.Workbook, filename, asciiName string) {
	data, err := wb.Bytes()
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", xlsxContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, asciiName, url.PathEscape(filename)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	_, _ = w.Write(data)
}

func (s *Server) exportBookings(w http.ResponseWriter, r *http.Request) {
	f := bookingFilter(r)
	wb, err := s.svc.ExportBookings(r.Context(), mustActor(r), f)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeXLSX(w, r, wb, exportName("הזמנות", f.From, f.To, wb.Location), exportName("bookings", f.From, f.To, wb.Location))
}

func (s *Server) exportPayments(w http.ResponseWriter, r *http.Request) {
	f := paymentFilter(r)
	wb, err := s.svc.ExportPayments(r.Context(), mustActor(r), f)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeXLSX(w, r, wb, exportName("תשלומים", f.From, f.To, wb.Location), exportName("payments", f.From, f.To, wb.Location))
}
