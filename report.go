package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ReportGenConfig menyimpan parameter untuk generate laporan DOCX.
type ReportGenConfig struct {
	TemplatePath string
	OutputPath   string
	IsMonthly    bool
	SIDs         []string
	ImageDir     string
	StartDate    time.Time
	EndDate      time.Time
	TicketData   string // raw ticket text untuk Laporan Harian (opsional)
}

// TicketEntry menyimpan satu record tiket gangguan layanan.
type TicketEntry struct {
	ID        string
	OpenDate  time.Time
	CloseDate time.Time
	Customer  string
}

// ── Ticket parser ────────────────────────────────────────────────────────────

var (
	// Mendeteksi awal record: TICKET_ID (huruf besar + angka, 6–12 char) diikuti SID 12 digit
	ticketStartRe = regexp.MustCompile(`(?:^|\s)([A-Z][A-Z0-9]{5,11})\s+\d{12}\s`)

	// Mem-parse satu record tiket yang sudah diisolasi
	oneTicketRe = regexp.MustCompile(
		`^([A-Z][A-Z0-9]{5,11})\s+` + // 1: ticket ID
			`\d{10,13}\s+` + // SID (abaikan)
			`.+?\s+` + // tipe layanan (abaikan, lazy)
			`(\d{4}-\d{2}-\d{2})\s+` + // 2: open date
			`\d{2}:\d{2}\s+` + // open time (abaikan)
			`[\d.,]+\s+` + // durasi (abaikan)
			`.+?\s+` + // deskripsi masalah (abaikan, lazy)
			`(\d{4}-\d{2}-\d{2})\s+` + // 3: close date
			`\d{2}:\d{2}\s+` + // close time (abaikan)
			`(.+)$`, // 4: nama customer
	)
)

// looksLikeCSV mengembalikan true jika input terlihat seperti format CSV
// (baris pertama non-kosong mengandung ≥9 koma).
func looksLikeCSV(raw string) bool {
	for _, line := range strings.SplitN(raw, "\n", 5) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return strings.Count(line, ",") >= 9
	}
	return false
}

// parseCSVTickets mem-parse data tiket format CSV (dengan atau tanpa header).
// Kolom: TICKET_ID,SID,TIPE,TGL_BUKA,JAM_BUKA,DURASI,DESKRIPSI,TGL_TUTUP,JAM_TUTUP,NAMA_CUSTOMER
func parseCSVTickets(raw string) []TicketEntry {
	r := csv.NewReader(strings.NewReader(raw))
	r.FieldsPerRecord = -1 // toleran jumlah kolom berbeda
	r.TrimLeadingSpace = true

	var entries []TicketEntry
	headerSkipped := false
	for {
		fields, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(fields) < 10 {
			continue
		}
		// Lewati baris header
		if !headerSkipped && strings.EqualFold(strings.TrimSpace(fields[0]), "TICKET_ID") {
			headerSkipped = true
			continue
		}
		headerSkipped = true
		openDate, err1 := time.Parse("2006-01-02", strings.TrimSpace(fields[3]))
		closeDate, err2 := time.Parse("2006-01-02", strings.TrimSpace(fields[7]))
		if err1 != nil || err2 != nil {
			continue
		}
		entries = append(entries, TicketEntry{
			ID:        strings.TrimSpace(fields[0]),
			OpenDate:  openDate,
			CloseDate: closeDate,
			Customer:  strings.TrimSpace(fields[9]),
		})
	}
	return entries
}

func parseTicketData(raw string) []TicketEntry {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	// Deteksi otomatis: CSV atau raw text
	if looksLikeCSV(raw) {
		return parseCSVTickets(raw)
	}
	// Raw text: normalisasi whitespace lalu parse per record
	raw = regexp.MustCompile(`\s+`).ReplaceAllString(raw, " ")

	locs := ticketStartRe.FindAllStringIndex(raw, -1)
	if len(locs) == 0 {
		return nil
	}
	var entries []TicketEntry
	for i, loc := range locs {
		end := len(raw)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		record := strings.TrimSpace(raw[loc[0]:end])
		if e, ok := parseOneTicket(record); ok {
			entries = append(entries, e)
		}
	}
	return entries
}

func parseOneTicket(record string) (TicketEntry, bool) {
	m := oneTicketRe.FindStringSubmatch(record)
	if m == nil {
		return TicketEntry{}, false
	}
	openDate, err1 := time.Parse("2006-01-02", m[2])
	closeDate, err2 := time.Parse("2006-01-02", m[3])
	if err1 != nil || err2 != nil {
		return TicketEntry{}, false
	}
	return TicketEntry{
		ID:        m[1],
		OpenDate:  openDate,
		CloseDate: closeDate,
		Customer:  strings.TrimSpace(m[4]),
	}, true
}

// groupTicketsByDate mengelompokkan tiket berdasarkan tanggal buka (byOpen=true)
// atau tanggal tutup (byOpen=false). Key map: "2006-01-02".
func groupTicketsByDate(tickets []TicketEntry, byOpen bool) map[string][]TicketEntry {
	m := make(map[string][]TicketEntry)
	for _, t := range tickets {
		var d time.Time
		if byOpen {
			d = t.OpenDate
		} else {
			d = t.CloseDate
		}
		key := d.Format("2006-01-02")
		m[key] = append(m[key], t)
	}
	return m
}

// ── Harian XML builders ──────────────────────────────────────────────────────

func buildDayHeadingXML(date time.Time) string {
	label := fmt.Sprintf("Capture Layanan Tanggal %d %s", date.Day(), bulanPanjang[date.Month()-1])
	return fmt.Sprintf(
		`<w:p>`+
			`<w:pPr><w:spacing w:before="200" w:after="100"/></w:pPr>`+
			`<w:r><w:rPr><w:b/><w:sz w:val="22"/><w:szCs w:val="22"/></w:rPr>`+
			`<w:t xml:space="preserve">%s</w:t>`+
			`</w:r></w:p>`,
		xmlEscape(label))
}

func buildTicketGroupXML(label string, tickets []TicketEntry) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		`<w:p><w:pPr><w:spacing w:before="140" w:after="40"/></w:pPr>`+
			`<w:r><w:rPr><w:b/><w:sz w:val="22"/><w:szCs w:val="22"/></w:rPr>`+
			`<w:t xml:space="preserve">%s</w:t></w:r></w:p>`,
		xmlEscape(label)))
	if len(tickets) == 0 {
		sb.WriteString(
			`<w:p><w:pPr><w:spacing w:after="0"/></w:pPr>` +
				`<w:r><w:t>-</w:t></w:r></w:p>`)
	} else {
		for _, t := range tickets {
			sb.WriteString(fmt.Sprintf(
				`<w:p><w:pPr><w:spacing w:after="0"/></w:pPr>`+
					`<w:r><w:t xml:space="preserve">- %s | %s</w:t></w:r></w:p>`,
				xmlEscape(t.ID), xmlEscape(t.Customer)))
		}
	}
	return sb.String()
}

func buildTicketSectionXML(openTickets, closeTickets []TicketEntry) string {
	var sb strings.Builder
	sb.WriteString(buildTicketGroupXML("Tiket Open :", openTickets))
	sb.WriteString(`<w:p><w:pPr><w:spacing w:after="100"/></w:pPr></w:p>`)
	sb.WriteString(buildTicketGroupXML("Tiket Close :", closeTickets))
	return sb.String()
}

// readSIDMeta membaca capture_meta.json dari folder imageDir.
// Mengembalikan map SID→tipe ("Backhaul"/"METRONET"/"INTERNET").
func readSIDMeta(imageDir string) map[string]string {
	data, err := os.ReadFile(filepath.Join(imageDir, "capture_meta.json"))
	if err != nil {
		return make(map[string]string)
	}
	var meta map[string]string
	if err := json.Unmarshal(data, &meta); err != nil {
		return make(map[string]string)
	}
	return meta
}

// sidTypeOrder mengembalikan urutan tampil: Backhaul(0) < INTERNET(1) < METRONET(2).
// Tipe kosong / tidak dikenal diperlakukan seperti INTERNET agar urutan asli terjaga.
func sidTypeOrder(t string) int {
	switch t {
	case "Backhaul":
		return 0
	case "METRONET":
		return 2
	default: // "INTERNET" atau kosong
		return 1
	}
}

// replaceTextPlaceholder mengganti placeholder teks sederhana (mis. {{TAHUN}}).
// Fast path: ganti semua kemunculan literal terlebih dahulu.
// Slow path: lanjutkan paragraf per paragraf untuk menangkap placeholder
// yang terpecah antar run XML oleh Word (kedua jalur selalu dijalankan).
func replaceTextPlaceholder(docXML, placeholder, value string) string {
	// Fast path — ganti semua kemunculan literal dulu
	docXML = strings.ReplaceAll(docXML, placeholder, value)

	// Slow path — tangkap sisa yang terpecah antar run
	var out strings.Builder
	rest := docXML
	for {
		pIdx := findParaStart(rest)
		if pIdx < 0 {
			out.WriteString(rest)
			break
		}
		out.WriteString(rest[:pIdx])
		rest = rest[pIdx:]
		eIdx := strings.Index(rest, "</w:p>")
		if eIdx < 0 {
			out.WriteString(rest)
			break
		}
		eIdx += len("</w:p>")
		para := rest[:eIdx]
		rest = rest[eIdx:]
		if strings.Contains(extractParaText(para), placeholder) {
			out.WriteString(consolidateParaText(para, placeholder, value))
		} else {
			out.WriteString(para)
		}
	}
	return out.String()
}

// consolidateParaText menggabungkan semua run dalam satu paragraf, mengganti placeholder,
// dan menghasilkan kembali paragraf dengan satu run tunggal (mempertahankan format pertama).
func consolidateParaText(para, placeholder, value string) string {
	// Opening tag <w:p ...>
	openEnd := strings.Index(para, ">")
	if openEnd < 0 {
		return para
	}
	openTag := para[:openEnd+1]

	// Paragraph properties
	var pPrContent string
	pPrS := strings.Index(para, "<w:pPr")
	pPrE := strings.Index(para, "</w:pPr>")
	if pPrS >= 0 && pPrE >= 0 {
		pPrContent = para[pPrS : pPrE+len("</w:pPr>")]
	}

	// Run properties dari run pertama
	var rPrContent string
	rPrS := strings.Index(para, "<w:rPr")
	rPrE := strings.Index(para, "</w:rPr>")
	if rPrS >= 0 && rPrE >= 0 {
		rPrContent = para[rPrS : rPrE+len("</w:rPr>")]
	}

	newText := strings.ReplaceAll(extractParaText(para), placeholder, xmlEscape(value))

	var sb strings.Builder
	sb.WriteString(openTag)
	if pPrContent != "" {
		sb.WriteString(pPrContent)
	}
	if newText != "" {
		sb.WriteString("<w:r>")
		if rPrContent != "" {
			sb.WriteString(rPrContent)
		}
		sb.WriteString(`<w:t xml:space="preserve">`)
		sb.WriteString(newText)
		sb.WriteString("</w:t></w:r>")
	}
	sb.WriteString("</w:p>")
	return sb.String()
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// GenerateReport membuat DOCX laporan dari template + hasil capture.
//
// Template DOCX harus berisi placeholder teks:
//   - {{BULAN}}         → nama bulan (misal: MEI)     — untuk Bulanan
//   - {{TAHUN}}         → tahun (misal: 2026)
//   - {{TANGGAL_MULAI}} → tanggal mulai (DD/MM/YYYY)  — untuk Harian
//   - {{TANGGAL_AKHIR}} → tanggal akhir (DD/MM/YYYY)  — untuk Harian
//   - {{SID_CONTENT}}   → posisi sisipan section chart SID (wajib)
//
// Laporan Bulanan: 1 gambar per SID, 2 SID per halaman.
// Laporan Harian:  1 gambar per SID per hari, dikelompokkan per SID.
func GenerateReport(cfg *ReportGenConfig) error {
	zr, err := zip.OpenReader(cfg.TemplatePath)
	if err != nil {
		return fmt.Errorf("gagal buka template DOCX: %w", err)
	}
	defer zr.Close()

	// Baca semua file dari template ZIP
	fileMap := make(map[string][]byte)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("gagal buka %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("gagal baca %s: %w", f.Name, err)
		}
		fileMap[f.Name] = data
	}

	// ── Ganti placeholder teks (tahan terhadap run terpecah oleh Word) ────────
	bulanStr := strings.ToUpper(bulanPanjang[cfg.StartDate.Month()-1])
	tahunStr := fmt.Sprintf("%d", cfg.StartDate.Year())

	docXML := string(fileMap["word/document.xml"])
	docXML = replaceTextPlaceholder(docXML, "{{BULAN}}", bulanStr)
	docXML = replaceTextPlaceholder(docXML, "{{TAHUN}}", tahunStr)
	docXML = replaceTextPlaceholder(docXML, "{{TANGGAL_MULAI}}", cfg.StartDate.Format("02/01/2006"))
	docXML = replaceTextPlaceholder(docXML, "{{TANGGAL_AKHIR}}", cfg.EndDate.Format("02/01/2006"))

	// ── Baca metadata tipe SID & urutkan ──────────────────────────────────────
	sidMeta := readSIDMeta(cfg.ImageDir)
	sortedSIDs := make([]string, len(cfg.SIDs))
	copy(sortedSIDs, cfg.SIDs)
	sort.SliceStable(sortedSIDs, func(i, j int) bool {
		return sidTypeOrder(sidMeta[sortedSIDs[i]]) < sidTypeOrder(sidMeta[sortedSIDs[j]])
	})

	// ── Bangun XML section SID ────────────────────────────────────────────────
	type relEntry struct {
		rID     string
		zipName string
		data    []byte
	}
	var rels []relEntry
	imgCounter := 0

	addImage := func(imgPath string) (rID string, cx, cy int64, ok bool) {
		imgData, err := os.ReadFile(imgPath)
		if err != nil || len(imgData) == 0 {
			return "", 0, 0, false
		}
		imgCfg, _, decErr := image.DecodeConfig(bytes.NewReader(imgData))
		const targetW = int64(5080000) // ~13.6cm dalam EMU
		cx = targetW
		cy = int64(3048000)
		if decErr == nil && imgCfg.Width > 0 {
			cy = targetW * int64(imgCfg.Height) / int64(imgCfg.Width)
		}
		imgCounter++
		rID = fmt.Sprintf("rId%d", 200+imgCounter)
		zipName := fmt.Sprintf("word/media/capture%d.jpeg", imgCounter)
		rels = append(rels, relEntry{rID: rID, zipName: zipName, data: imgData})
		return rID, cx, cy, true
	}

	var sidParts []string
	docID := 1000

	if cfg.IsMonthly {
		// ── Laporan Bulanan: satu gambar per SID ──────────────────────────────
		for i, sid := range sortedSIDs {
			imgPath := filepath.Join(cfg.ImageDir,
				fmt.Sprintf("%s_%s.jpg", sid, cfg.StartDate.Format("01-2006")))

			sidParts = append(sidParts, buildSIDHeadingXML(i+1, sid, sidMeta[sid]))
			rID, cx, cy, ok := addImage(imgPath)
			if ok {
				sidParts = append(sidParts, buildSIDImageXML(rID, cx, cy, docID))
				docID++
			} else {
				sidParts = append(sidParts,
					`<w:p><w:r><w:t xml:space="preserve">[Gambar tidak ditemukan: `+imgPath+`]</w:t></w:r></w:p>`)
			}

			// Page break setiap 2 SID
			if (i+1)%2 == 0 && i < len(sortedSIDs)-1 {
				sidParts = append(sidParts,
					`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
			} else {
				sidParts = append(sidParts,
					`<w:p><w:pPr><w:spacing w:after="280"/></w:pPr></w:p>`)
			}
		}
	} else {
		// ── Laporan Harian: per hari → semua SID + tiket open/close ──────────
		tickets := parseTicketData(cfg.TicketData)
		openByDate := groupTicketsByDate(tickets, true)
		closeByDate := groupTicketsByDate(tickets, false)

		dates := generateDateRange(cfg.StartDate, cfg.EndDate)
		for i, date := range dates {
			// Heading tanggal: "Capture Layanan Tanggal D Bulan"
			sidParts = append(sidParts, buildDayHeadingXML(date))

			// Gambar semua SID untuk hari ini
			for _, sid := range sortedSIDs {
				imgPath := filepath.Join(cfg.ImageDir,
					fmt.Sprintf("%s_%s.jpg", sid, date.Format("02-01-2006")))

				rID, cx, cy, ok := addImage(imgPath)
				if ok {
					sidParts = append(sidParts, buildSIDImageXML(rID, cx, cy, docID))
					docID++
				} else {
					sidParts = append(sidParts,
						`<w:p><w:r><w:t xml:space="preserve">[Gambar tidak ditemukan: `+
							xmlEscape(filepath.Base(imgPath))+`]</w:t></w:r></w:p>`)
				}
				sidParts = append(sidParts,
					`<w:p><w:pPr><w:spacing w:after="80"/></w:pPr></w:p>`)
			}

			// Tiket Open & Close untuk hari ini
			dateKey := date.Format("2006-01-02")
			sidParts = append(sidParts, buildTicketSectionXML(openByDate[dateKey], closeByDate[dateKey]))

			// Page break antar hari (kecuali hari terakhir)
			if i < len(dates)-1 {
				sidParts = append(sidParts,
					`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
			}
		}
	}

	sidContentXML := strings.Join(sidParts, "\n")

	// ── Sisipkan SID content ke dokumen ──────────────────────────────────────
	docXML = replacePlaceholderParagraph(docXML, "{{SID_CONTENT}}", sidContentXML)
	fileMap["word/document.xml"] = []byte(docXML)

	// ── Update relationship untuk gambar ─────────────────────────────────────
	relsXML := string(fileMap["word/_rels/document.xml.rels"])
	for _, rel := range rels {
		entry := fmt.Sprintf(
			`<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/%s"/>`,
			rel.rID, filepath.Base(rel.zipName))
		relsXML = strings.Replace(relsXML, "</Relationships>", entry+"\n</Relationships>", 1)
	}
	fileMap["word/_rels/document.xml.rels"] = []byte(relsXML)

	// ── Pastikan tipe konten JPEG terdaftar ──────────────────────────────────
	ctXML := string(fileMap["[Content_Types].xml"])
	if !strings.Contains(ctXML, `Extension="jpeg"`) && !strings.Contains(ctXML, `Extension="jpg"`) {
		ctXML = strings.Replace(ctXML, "</Types>",
			`<Default Extension="jpeg" ContentType="image/jpeg"/>`+"\n</Types>", 1)
	}
	fileMap["[Content_Types].xml"] = []byte(ctXML)

	// ── Tambahkan file gambar ke ZIP ─────────────────────────────────────────
	for _, rel := range rels {
		fileMap[rel.zipName] = rel.data
	}

	return writeDocxZIP(cfg.OutputPath, fileMap)
}

// buildSIDHeadingXML menghasilkan paragraf heading bernomor untuk satu SID.
// sidType diisi dengan "Backhaul", "METRONET", "INTERNET", atau "" (tanpa label).
func buildSIDHeadingXML(num int, sid, sidType string) string {
	label := "SID " + sid
	if sidType != "" {
		label += "  " + sidType
	}
	return fmt.Sprintf(
		`<w:p>`+
			`<w:pPr><w:jc w:val="center"/><w:spacing w:before="240" w:after="80"/></w:pPr>`+
			`<w:r>`+
			`<w:rPr><w:b/><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr>`+
			`<w:t xml:space="preserve">%d.  %s</w:t>`+
			`</w:r>`+
			`</w:p>`,
		num, label)
}

// buildDateSubHeadingXML menghasilkan paragraf sub-heading tanggal (untuk Harian).
func buildDateSubHeadingXML(dateLbl string) string {
	return fmt.Sprintf(
		`<w:p>`+
			`<w:pPr><w:spacing w:before="120" w:after="60"/></w:pPr>`+
			`<w:r>`+
			`<w:rPr><w:b/><w:color w:val="336699"/><w:sz w:val="22"/></w:rPr>`+
			`<w:t xml:space="preserve">📅  %s</w:t>`+
			`</w:r>`+
			`</w:p>`,
		dateLbl)
}

// buildSIDImageXML menghasilkan paragraf berisi gambar inline DOCX.
func buildSIDImageXML(rID string, cx, cy int64, imgID int) string {
	return fmt.Sprintf(
		`<w:p>`+
			`<w:pPr><w:jc w:val="center"/><w:spacing w:after="120"/></w:pPr>`+
			`<w:r><w:drawing>`+
			`<wp:inline distT="0" distB="0" distL="0" distR="0"`+
			` xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing">`+
			`<wp:extent cx="%d" cy="%d"/>`+
			`<wp:effectExtent l="0" t="0" r="0" b="0"/>`+
			`<wp:docPr id="%d" name="Capture%d"/>`+
			`<wp:cNvGraphicFramePr>`+
			`<a:graphicFrameLocks xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" noChangeAspect="1"/>`+
			`</wp:cNvGraphicFramePr>`+
			`<a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">`+
			`<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">`+
			`<pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">`+
			`<pic:nvPicPr>`+
			`<pic:cNvPr id="%d" name="Capture%d"/>`+
			`<pic:cNvPicPr/>`+
			`</pic:nvPicPr>`+
			`<pic:blipFill>`+
			`<a:blip r:embed="%s" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/>`+
			`<a:stretch><a:fillRect/></a:stretch>`+
			`</pic:blipFill>`+
			`<pic:spPr>`+
			`<a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm>`+
			`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`+
			`</pic:spPr>`+
			`</pic:pic>`+
			`</a:graphicData>`+
			`</a:graphic>`+
			`</wp:inline>`+
			`</w:drawing></w:r>`+
			`</w:p>`,
		cx, cy,
		imgID, imgID,
		imgID, imgID,
		rID,
		cx, cy,
	)
}

// lastParaStart returns the position of the last <w:p> or <w:p (space) before pos.
// It avoids false matches on <w:pPr>, <w:pStyle>, etc.
func lastParaStart(s string, before int) int {
	sub := s[:before]
	p1 := strings.LastIndex(sub, "<w:p>")
	p2 := strings.LastIndex(sub, "<w:p ")
	if p1 > p2 {
		return p1
	}
	return p2
}

// replacePlaceholderParagraph mengganti seluruh paragraf yang mengandung
// placeholder dengan replacement XML. Fallback ke sisip sebelum </w:body>.
func replacePlaceholderParagraph(docXML, placeholder, replacement string) string {
	// Coba penggantian langsung (placeholder utuh dalam satu XML run)
	if strings.Contains(docXML, placeholder) {
		idx := strings.Index(docXML, placeholder)
		pStart := lastParaStart(docXML, idx)
		if pStart >= 0 {
			pEnd := strings.Index(docXML[idx:], "</w:p>")
			if pEnd >= 0 {
				pEnd = idx + pEnd + len("</w:p>")
				return docXML[:pStart] + replacement + docXML[pEnd:]
			}
		}
		// Placeholder found but no enclosing <w:p> — replace inline (edge case)
		return strings.Replace(docXML, placeholder, replacement, 1)
	}

	// Cari di text content (placeholder mungkin tersebar antar run Word)
	remaining := docXML
	var out strings.Builder
	found := false
	for len(remaining) > 0 {
		pStart := findParaStart(remaining)
		if pStart < 0 {
			out.WriteString(remaining)
			break
		}
		out.WriteString(remaining[:pStart])
		remaining = remaining[pStart:]

		pEnd := strings.Index(remaining, "</w:p>")
		if pEnd < 0 {
			out.WriteString(remaining)
			break
		}
		pEnd += len("</w:p>")
		para := remaining[:pEnd]
		remaining = remaining[pEnd:]

		if !found && strings.Contains(extractParaText(para), placeholder) {
			out.WriteString(replacement)
			found = true
		} else {
			out.WriteString(para)
		}
	}
	if found {
		return out.String()
	}

	// Placeholder tidak ditemukan → sisipkan sebelum </w:body>
	return strings.Replace(docXML, "</w:body>", replacement+"</w:body>", 1)
}

func findParaStart(s string) int {
	i := strings.Index(s, "<w:p>")
	j := strings.Index(s, "<w:p ")
	if i < 0 {
		return j
	}
	if j < 0 {
		return i
	}
	if i < j {
		return i
	}
	return j
}

// extractParaText mengambil teks dari semua elemen <w:t> dalam satu paragraf.
func extractParaText(para string) string {
	var sb strings.Builder
	rest := para
	for {
		start := strings.Index(rest, "<w:t")
		if start < 0 {
			break
		}
		tagEnd := strings.Index(rest[start:], ">")
		if tagEnd < 0 {
			break
		}
		tagEnd += start + 1
		closeTag := strings.Index(rest[tagEnd:], "</w:t>")
		if closeTag < 0 {
			break
		}
		sb.WriteString(rest[tagEnd : tagEnd+closeTag])
		rest = rest[tagEnd+closeTag+len("</w:t>"):]
	}
	return sb.String()
}

// writeDocxZIP menulis fileMap ke file ZIP (format DOCX).
func writeDocxZIP(outputPath string, fileMap map[string][]byte) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("gagal buat file output: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	// Tulis [Content_Types].xml dan _rels/.rels lebih dahulu (konvensi OOXML)
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels"} {
		if data, ok := fileMap[name]; ok {
			w, err := zw.Create(name)
			if err != nil {
				return err
			}
			if _, err := w.Write(data); err != nil {
				return err
			}
		}
	}
	for name, data := range fileMap {
		if name == "[Content_Types].xml" || name == "_rels/.rels" {
			continue
		}
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return nil
}
