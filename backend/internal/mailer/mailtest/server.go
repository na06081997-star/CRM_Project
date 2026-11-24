// Package mailtest is a tiny SMTP server for tests: it captures every
// message it accepts and can be told to refuse deliveries.
package mailtest

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type Message struct {
	From    string
	To      []string
	Raw     []byte
	Subject string
	Text    string
	HTML    string
}

type Server struct {
	ln net.Listener

	mu       sync.Mutex
	msgs     []Message
	rejectTo int    // SMTP code returned to RCPT TO (0 = accept)
	rejectMs string // its text
}

// Start listens on a random local port until the test ends.
func Start(t testing.TB) *Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{ln: ln}
	go s.serve()
	t.Cleanup(func() { ln.Close() })
	return s
}

func (s *Server) Port() int { return s.ln.Addr().(*net.TCPAddr).Port }

// Reject makes every following RCPT TO fail with this SMTP reply (e.g. 550 or 451). Code 0 accepts again.
func (s *Server) Reject(code int, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rejectTo, s.rejectMs = code, text
}

func (s *Server) Messages() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Message(nil), s.msgs...)
}

// Last returns the last message sent to addr.
func (s *Server) Last(t testing.TB, addr string) Message {
	t.Helper()
	msgs := s.Messages()
	for i := len(msgs) - 1; i >= 0; i-- {
		for _, to := range msgs[i].To {
			if strings.EqualFold(to, addr) {
				return msgs[i]
			}
		}
	}
	t.Fatalf("no e-mail captured for %s (%d messages)", addr, len(msgs))
	return Message{}
}

// Count returns how many messages were sent to addr.
func (s *Server) Count(addr string) int {
	n := 0
	for _, m := range s.Messages() {
		for _, to := range m.To {
			if strings.EqualFold(to, addr) {
				n++
			}
		}
	}
	return n
}

func (s *Server) serve() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.session(c)
	}
}

func (s *Server) session(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	w := func(line string) { fmt.Fprintf(c, "%s\r\n", line) }
	w("220 mailtest ESMTP ready")
	var cur Message
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		cmd := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(cmd, "EHLO"):
			w("250-mailtest")
			w("250-8BITMIME")
			w("250 SMTPUTF8")
		case strings.HasPrefix(cmd, "HELO"):
			w("250 mailtest")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			cur = Message{From: addr(line[10:])}
			w("250 OK")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			s.mu.Lock()
			code, text := s.rejectTo, s.rejectMs
			s.mu.Unlock()
			if code != 0 {
				w(strconv.Itoa(code) + " " + text)
				continue
			}
			cur.To = append(cur.To, addr(line[8:]))
			w("250 OK")
		case cmd == "DATA":
			w("354 end with <CRLF>.<CRLF>")
			var buf bytes.Buffer
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" || l == ".\n" {
					break
				}
				buf.WriteString(strings.TrimPrefix(l, "."))
			}
			cur.Raw = buf.Bytes()
			parse(&cur)
			s.mu.Lock()
			s.msgs = append(s.msgs, cur)
			s.mu.Unlock()
			w("250 OK queued")
		case cmd == "RSET" || cmd == "NOOP":
			w("250 OK")
		case cmd == "QUIT":
			w("221 bye")
			return
		default:
			w("502 command not implemented")
		}
	}
}

func addr(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, " "); i > 0 {
		s = s[:i]
	}
	return strings.Trim(s, "<>")
}

func parse(m *Message) {
	msg, err := mail.ReadMessage(bytes.NewReader(m.Raw))
	if err != nil {
		return
	}
	dec := new(mime.WordDecoder)
	m.Subject, _ = dec.DecodeHeader(msg.Header.Get("Subject"))
	walk(m, msg.Header.Get("Content-Type"), msg.Header.Get("Content-Transfer-Encoding"), msg.Body)
}

func walk(m *Message, ctype, enc string, body io.Reader) {
	mt, params, err := mime.ParseMediaType(ctype)
	if err != nil {
		mt = "text/plain"
	}
	if strings.HasPrefix(mt, "multipart/") {
		mr := multipart.NewReader(body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err != nil {
				return
			}
			walk(m, part.Header.Get("Content-Type"), part.Header.Get("Content-Transfer-Encoding"), part)
		}
	}
	var r io.Reader = body
	switch strings.ToLower(enc) {
	case "quoted-printable":
		r = quotedprintable.NewReader(body)
	case "base64":
		r = base64.NewDecoder(base64.StdEncoding, body)
	}
	b, _ := io.ReadAll(r)
	switch mt {
	case "text/plain":
		m.Text = string(b)
	case "text/html":
		m.HTML = string(b)
	}
}
