// Utility: generate sample DOCX template for Laporan Bulanan & Harian.
// Run: go run ./gen_template
package main

import (
	"archive/zip"
	"os"
)

func main() {
	genBulanan("Template_Laporan_Bulanan_SAMPLE.docx")
	genHarian("Template_Laporan_Harian_SAMPLE.docx")
}

// ── shared XML parts ──────────────────────────────────────────────────────────

const contentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml"  ContentType="application/xml"/>
  <Override PartName="/word/document.xml"
    ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml"
    ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
  <Override PartName="/word/settings.xml"
    ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
</Types>`

const dotRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1"
    Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
    Target="word/document.xml"/>
</Relationships>`

const docRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1"
    Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"
    Target="styles.xml"/>
  <Relationship Id="rId2"
    Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings"
    Target="settings.xml"/>
</Relationships>`

const settings = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:defaultTabStop w:val="720"/>
</w:settings>`

const styles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
          xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml">
  <w:docDefaults>
    <w:rPrDefault>
      <w:rPr>
        <w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/>
        <w:sz w:val="22"/>
        <w:szCs w:val="22"/>
      </w:rPr>
    </w:rPrDefault>
    <w:pPrDefault>
      <w:pPr><w:spacing w:after="160" w:line="259" w:lineRule="auto"/></w:pPr>
    </w:pPrDefault>
  </w:docDefaults>

  <w:style w:type="paragraph" w:styleId="Normal" w:default="1">
    <w:name w:val="Normal"/>
  </w:style>

  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:pPr><w:spacing w:before="240" w:after="60"/></w:pPr>
    <w:rPr>
      <w:b/>
      <w:sz w:val="32"/><w:szCs w:val="32"/>
    </w:rPr>
  </w:style>

  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="heading 2"/>
    <w:pPr><w:spacing w:before="200" w:after="60"/></w:pPr>
    <w:rPr>
      <w:b/>
      <w:sz w:val="28"/><w:szCs w:val="28"/>
    </w:rPr>
  </w:style>
</w:styles>`

// ── page margin helper ────────────────────────────────────────────────────────

const pageSectPr = `<w:sectPr>
    <w:pgSz w:w="11906" w:h="16838"/>
    <w:pgMar w:top="1134" w:right="1134" w:bottom="1134" w:left="1701"
             w:header="709" w:footer="709" w:gutter="0"/>
  </w:sectPr>`

// ── paragraph helpers ─────────────────────────────────────────────────────────

func bold(text, size string) string {
	return `<w:p><w:pPr><w:jc w:val="center"/></w:pPr>` +
		`<w:r><w:rPr><w:b/><w:sz w:val="` + size + `"/><w:szCs w:val="` + size + `"/></w:rPr>` +
		`<w:t>` + text + `</w:t></w:r></w:p>`
}

func normal(text string) string {
	return `<w:p><w:r><w:t xml:space="preserve">` + text + `</w:t></w:r></w:p>`
}

func centered(text string) string {
	return `<w:p><w:pPr><w:jc w:val="center"/></w:pPr>` +
		`<w:r><w:t>` + text + `</w:t></w:r></w:p>`
}

func centeredBig(text, size, color string) string {
	colorTag := ""
	if color != "" {
		colorTag = `<w:color w:val="` + color + `"/>`
	}
	return `<w:p><w:pPr><w:jc w:val="center"/><w:spacing w:before="480" w:after="480"/></w:pPr>` +
		`<w:r><w:rPr><w:b/>` + colorTag + `<w:sz w:val="` + size + `"/><w:szCs w:val="` + size + `"/></w:rPr>` +
		`<w:t>` + text + `</w:t></w:r></w:p>`
}

func pageBreak() string {
	return `<w:p><w:r><w:br w:type="page"/></w:r></w:p>`
}

func emptyLine() string {
	return `<w:p><w:pPr><w:spacing w:after="0"/></w:pPr></w:p>`
}

func hLine() string {
	return `<w:p><w:pPr>` +
		`<w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="AAAAAA"/></w:pBdr>` +
		`<w:spacing w:before="120" w:after="120"/>` +
		`</w:pPr></w:p>`
}

// ── Laporan Bulanan ───────────────────────────────────────────────────────────

func genBulanan(outPath string) {
	doc := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
            xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<w:body>

  <!-- ═══════════════════════════════════════════════════════
       HALAMAN 1 — COVER
       ═══════════════════════════════════════════════════════ -->
` +
		emptyLine() + emptyLine() + emptyLine() +
		bold("LAPORAN", "52") +
		bold("LAYANAN INTERNET", "36") +
		emptyLine() +
		bold("DINAS KOMUNIKASI, INFORMATIKA, DAN PERSANDIAN", "24") +
		bold("KABUPATEN ACEH BARAT DAYA", "24") +
		emptyLine() + emptyLine() + emptyLine() + emptyLine() +
		centeredBig("{{BULAN}} {{TAHUN}}", "52", "1F4E79") +
		pageBreak() +

		// ── HALAMAN 2 — DOKUMEN ──────────────────────────────────────────────
		`  <!-- HALAMAN 2 — DOKUMEN -->
` +
		bold("Dokumen", "32") +
		hLine() +
		emptyLine() +
		bold("Disiapkan Oleh :", "24") +
		emptyLine() + emptyLine() +
		centered("PT PLN ICON PLUS") +
		emptyLine() + emptyLine() + emptyLine() + emptyLine() + emptyLine() + emptyLine() +

		`<w:p><w:pPr><w:jc w:val="center"/><w:spacing w:before="600"/></w:pPr>` +
		`<w:r><w:rPr><w:b/><w:sz w:val="28"/><w:szCs w:val="28"/></w:rPr>` +
		`<w:t>PT PLN ICON PLUS</w:t></w:r></w:p>` +
		centered("{{TAHUN}}") +
		pageBreak() +

		// ── HALAMAN 3 — KONTEN SID ───────────────────────────────────────────
		`  <!-- HALAMAN 3 — KONTEN SID
       Sistem akan mengganti paragraf {{SID_CONTENT}} dengan
       semua section SID + gambar capture MRTG secara otomatis.
       JANGAN hapus atau ubah teks {{SID_CONTENT}} ini.
  -->
` +
		`<w:p><w:r><w:t>{{SID_CONTENT}}</w:t></w:r></w:p>` +

		`  <w:sectPr>
    <w:pgSz w:w="11906" w:h="16838"/>
    <w:pgMar w:top="1134" w:right="1134" w:bottom="1134" w:left="1701"
             w:header="709" w:footer="709" w:gutter="0"/>
  </w:sectPr>
</w:body>
</w:document>`

	writeDocx(outPath, doc)
}

// ── Laporan Harian ────────────────────────────────────────────────────────────

func genHarian(outPath string) {
	doc := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
            xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<w:body>

  <!-- ═══════════════════════════════════════════════════════
       HALAMAN 1 — COVER
       ═══════════════════════════════════════════════════════ -->
` +
		emptyLine() + emptyLine() + emptyLine() +
		bold("LAPORAN HARIAN", "52") +
		bold("LAYANAN INTERNET", "36") +
		emptyLine() +
		bold("DINAS KOMUNIKASI, INFORMATIKA, DAN PERSANDIAN", "24") +
		bold("KABUPATEN ACEH BARAT DAYA", "24") +
		emptyLine() + emptyLine() + emptyLine() + emptyLine() +
		centeredBig("{{BULAN}} {{TAHUN}}", "52", "1F4E79") +
		pageBreak() +

		// ── HALAMAN 2 — DOKUMEN ──────────────────────────────────────────────
		bold("Dokumen", "32") +
		hLine() +
		emptyLine() +
		bold("Disiapkan Oleh :", "24") +
		emptyLine() + emptyLine() + emptyLine() + emptyLine() + emptyLine() + emptyLine() +
		`<w:p><w:pPr><w:jc w:val="center"/><w:spacing w:before="600"/></w:pPr>` +
		`<w:r><w:rPr><w:b/><w:sz w:val="28"/><w:szCs w:val="28"/></w:rPr>` +
		`<w:t>PT PLN ICON PLUS</w:t></w:r></w:p>` +
		centered("{{TAHUN}}") +
		pageBreak() +

		// ── HALAMAN 3 — KONTEN SID ───────────────────────────────────────────
		`<w:p><w:r><w:t>{{SID_CONTENT}}</w:t></w:r></w:p>` +

		`  <w:sectPr>
    <w:pgSz w:w="11906" w:h="16838"/>
    <w:pgMar w:top="1134" w:right="1134" w:bottom="1134" w:left="1701"
             w:header="709" w:footer="709" w:gutter="0"/>
  </w:sectPr>
</w:body>
</w:document>`

	writeDocx(outPath, doc)
}

// ── ZIP writer ────────────────────────────────────────────────────────────────

func writeDocx(path, documentXML string) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	files := map[string]string{
		"[Content_Types].xml":          contentTypes,
		"_rels/.rels":                  dotRels,
		"word/document.xml":            documentXML,
		"word/_rels/document.xml.rels": docRels,
		"word/styles.xml":              styles,
		"word/settings.xml":            settings,
	}

	// Write in deterministic order (Content_Types first per OOXML convention)
	ordered := []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"word/_rels/document.xml.rels",
		"word/styles.xml",
		"word/settings.xml",
		"word/document.xml",
	}
	for _, name := range ordered {
		w, err := zw.Create(name)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write([]byte(files[name])); err != nil {
			panic(err)
		}
	}
}
