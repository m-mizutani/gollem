package main

import (
	"fmt"
	"os"
)

func main() {
	// Create a minimal valid PDF with a unique secret code.
	// The code "GOLLEM-PDF-7X9K2" is embedded inside PDF stream objects,
	// which can only be correctly read when the data is processed as a PDF document.
	// This ensures the LLM actually parses the PDF rather than reading raw bytes.
	content := "The secret code is: GOLLEM-PDF-7X9K2"

	// Build PDF objects
	streamContent := fmt.Sprintf("BT /F1 12 Tf 72 720 Td (%s) Tj ET", content)
	streamLength := len(streamContent)

	pdf := fmt.Sprintf(`%%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj

2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj

3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]
   /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>
endobj

4 0 obj
<< /Length %d >>
stream
%s
endstream
endobj

5 0 obj
<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>
endobj

xref
0 6
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
0000000266 00000 n
0000000000 00000 n

trailer
<< /Size 6 /Root 1 0 R >>
startxref
0
%%%%EOF
`, streamLength, streamContent)

	if err := os.WriteFile("test_document.pdf", []byte(pdf), 0600); err != nil {
		panic(err)
	}

	fmt.Println("Created test_document.pdf")
}
