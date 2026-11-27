package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/pkg/browser"
)

// ===== Exportação de resposta como PDF =====

var (
	reMDImage    = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	reMDLink     = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reMDBold     = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reMDItalic   = regexp.MustCompile(`(^|[\s(])\*([^*\n]+)\*`)
	reMDCode     = regexp.MustCompile("`([^`]+)`")
	reListBullet = regexp.MustCompile(`^\s*[-*+]\s+(.*)$`)
	reListNum    = regexp.MustCompile(`^\s*(\d+)[.)]\s+(.*)$`)
	reHeading    = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
)

// stripInlineMarkdown remove marcadores inline de uma linha markdown
func stripInlineMarkdown(s string) string {
	s = reMDImage.ReplaceAllString(s, "$1")
	s = reMDLink.ReplaceAllString(s, "$1 ($2)")
	s = reMDBold.ReplaceAllString(s, "$1")
	s = reMDItalic.ReplaceAllString(s, "${1}$2")
	s = reMDCode.ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

const (
	// fpdf faz path.Join(fontpath, file) INCONDICIONALMENTE e path.Join("",
	// "/abs") estraga paths absolutos — por isso o diretório vai no New e os
	// fontes por nome simples.
	pdfFontDir = "/usr/share/fonts/truetype/dejavu/"
	pdfMargin  = 18.0
)

// pdfFontDirReady retorna o fontdir se as fontes DejaVu existirem, ou "" para
// usar as fontes core (latin-1: acentos PT ok, emoji não).
func pdfFontDirReady() string {
	if existsFile(filepath.Join(pdfFontDir, "DejaVuSans.ttf")) {
		return pdfFontDir
	}
	return ""
}

// loadPDFFonts embute as fontes UTF-8 no pdf (fontDir já validado)
func loadPDFFonts(pdf *fpdf.Fpdf, fontDir string) {
	pdf.AddUTF8Font("dv", "", "DejaVuSans.ttf")
	if existsFile(filepath.Join(fontDir, "DejaVuSans-Bold.ttf")) {
		pdf.AddUTF8Font("dvb", "", "DejaVuSans-Bold.ttf")
	}
	if existsFile(filepath.Join(fontDir, "DejaVuSansMono.ttf")) {
		pdf.AddUTF8Font("dvm", "", "DejaVuSansMono.ttf")
	}
}

func existsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// renderMarkdownPDF gera o PDF da resposta em dest; retorna erro se falhar.
func RenderMarkdownPDF(dest, title, markdown string) error {
	fontDir := pdfFontDirReady()
	pdf := fpdf.New("P", "mm", "A4", fontDir)
	pdf.SetMargins(pdfMargin, 16, pdfMargin)
	pdf.SetAutoPageBreak(true, 16)
	utf8OK := fontDir != ""
	if utf8OK {
		loadPDFFonts(pdf, fontDir)
	}

	family := func(bold, mono bool) string {
		if !utf8OK {
			if mono {
				return "courier"
			}
			if bold {
				return "helveticaB"
			}
			return "helvetica"
		}
		if mono {
			return "dvm"
		}
		if bold {
			return "dvb"
		}
		return "dv"
	}

	// Cabeçalho do documento
	pdf.AddPage()
	pdf.SetFont(family(true, false), "", 16)
	pdf.MultiCell(0, 8, strings.TrimSpace(title), "", "", false)
	pdf.SetFont(family(false, false), "", 9)
	pdf.SetTextColor(120, 120, 120)
	pdf.MultiCell(0, 5, "Gemini Desktop · "+time.Now().Format("02/01/2006 15:04"), "", "", false)
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(2)

	type style struct {
		size   float64
		bold   bool
		mono   bool
		gray   bool
		indent float64
		bullet string
		lh     float64
	}
	writeLine := func(text string, st style) {
		pdf.SetFont(family(st.bold, st.mono), "", st.size)
		if st.gray {
			pdf.SetTextColor(90, 90, 90)
		} else {
			pdf.SetTextColor(0, 0, 0)
		}
		x := pdfMargin + st.indent
		if st.bullet != "" {
			pdf.SetX(x)
			pdf.CellFormat(0, st.lh, st.bullet, "", 1, "", false, 0, "")
			x += 4
		}
		pageW, _ := pdf.GetPageSize()
		pdf.SetX(x)
		pdf.MultiCell(pageW-x-pdfMargin, st.lh, text, "", "", false)
		pdf.SetTextColor(0, 0, 0)
	}

	var codeBuf []string
	flushCode := func() {
		if len(codeBuf) == 0 {
			return
		}
		pdf.SetFont(family(false, true), "", 9)
		pdf.SetFillColor(242, 242, 242)
		pageW, _ := pdf.GetPageSize()
		pdf.SetX(pdfMargin + 2)
		pdf.MultiCell(pageW-2*pdfMargin-4, 4.6, strings.Join(codeBuf, "\n"), "", "L", true)
		pdf.Ln(1.5)
		codeBuf = nil
	}

	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	var para []string
	flushPara := func() {
		if len(para) == 0 {
			return
		}
		writeLine(strings.Join(para, " "), style{size: 11, lh: 5.6})
		pdf.Ln(1.2)
		para = nil
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			flushPara()
			if len(codeBuf) > 0 {
				flushCode()
			} else {
				pdf.Ln(0.5)
			}
			continue
		}
		if len(codeBuf) > 0 {
			codeBuf = append(codeBuf, raw)
			continue
		}

		if trimmed == "" {
			flushPara()
			continue
		}

		if m := reHeading.FindStringSubmatch(trimmed); m != nil {
			flushPara()
			level := len(m[1])
			size := 13.0
			if level == 1 {
				size = 16
			} else if level == 2 {
				size = 14
			}
			writeLine(stripInlineMarkdown(m[2]), style{size: size, bold: true, lh: size * 0.55})
			pdf.Ln(1)
			continue
		}

		if trimmed == "---" || trimmed == "***" {
			flushPara()
			pdf.Ln(1)
			pdf.SetDrawColor(200, 200, 200)
			y := pdf.GetY()
			pageW, _ := pdf.GetPageSize()
			pdf.Line(pdfMargin, y, pageW-pdfMargin, y)
			pdf.Ln(2)
			continue
		}

		if strings.HasPrefix(trimmed, ">") {
			flushPara()
			writeLine(stripInlineMarkdown(strings.TrimPrefix(trimmed, ">")), style{size: 10.5, gray: true, indent: 5, lh: 5.4})
			continue
		}

		if m := reListBullet.FindStringSubmatch(trimmed); m != nil {
			flushPara()
			writeLine(stripInlineMarkdown(m[1]), style{size: 11, lh: 5.6, indent: 4, bullet: "•"})
			continue
		}
		if m := reListNum.FindStringSubmatch(trimmed); m != nil {
			flushPara()
			writeLine(stripInlineMarkdown(m[2]), style{size: 11, lh: 5.6, indent: 4, bullet: m[1] + "."})
			continue
		}

		para = append(para, stripInlineMarkdown(trimmed))
	}
	flushPara()
	flushCode()

	return pdf.OutputFileAndClose(dest)
}

// POST /api/export/pdf — exporta uma resposta markdown como PDF em ~/Downloads
func (s *Server) handleExportPDF(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title    string `json:"title"`
		Markdown string `json:"markdown"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Markdown) == "" {
		http.Error(w, "markdown obrigatório", http.StatusBadRequest)
		return
	}
	const maxMD = 2 << 20 // 2 MB de texto
	if len(req.Markdown) > maxMD {
		http.Error(w, "resposta muito longa para exportar", http.StatusRequestEntityTooLarge)
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Resposta do Gemini"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "falha ao resolver diretório do usuário", http.StatusInternalServerError)
		return
	}
	dir := filepath.Join(home, "Downloads", "Gemini Desktop")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, "falha ao criar diretório de exportação", http.StatusInternalServerError)
		return
	}

	base := SanitizeFilename(title)
	if base == "" {
		base = "resposta"
	}
	dest := filepath.Join(dir, fmt.Sprintf("%s-%s.pdf", base, time.Now().Format("20060102-150405")))
	if err := RenderMarkdownPDF(dest, title, req.Markdown); err != nil {
		http.Error(w, "falha ao gerar PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}

	display := "~/Downloads/Gemini Desktop/" + filepath.Base(dest)
	if s.notifications {
		go s.desktopNotify("Gemini Desktop", "📄 PDF salvo em "+display)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"path": dest, "displayPath": display})
}

// POST /api/open-url — abre uma URL http/https no navegador padrão do sistema
func (s *Server) handleOpenURL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	trimmed := strings.TrimSpace(req.URL)

	// URL http/https: abre no navegador padrão via browser.OpenURL (sem execução de caminhos locais)
	u, err := url.Parse(trimmed)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		http.Error(w, "URL inválida (apenas links http/https são permitidos)", http.StatusBadRequest)
		return
	}

	if err := browser.OpenURL(u.String()); err != nil {
		http.Error(w, "falha ao abrir no navegador: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
