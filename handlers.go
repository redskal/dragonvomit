package main

import (
	"archive/zip"
	"bytes"

	"github.com/redskal/dragonvomit/pkg/metadataplus"
	"seehuhn.de/go/pdf"
)

// pdfParse will grab metadata from a PDF to extract the
// author as a potential user, and the software used to
// create the PDF for intel.
func pdfParse(pdfFile []byte) (string, []string, error) {
	// pdf.Read expects an io.ReadSeeker interface
	// so we need to create a reader
	r := bytes.NewReader(pdfFile)

	// load the PDF
	pdfData, err := pdf.Read(r, nil)
	if err != nil {
		return "", []string{}, err
	}

	// load the metadata and check the author is there
	meta := pdfData.GetMeta()
	var author string
	if meta.Info.Author != "" {
		author = meta.Info.Author
	}

	// record the software if it's available
	var software []string
	if meta.Info.Creator != "" {
		software = append(software, meta.Info.Creator)
	}
	if meta.Info.Producer != "" {
		software = append(software, meta.Info.Producer)
	}

	return author, software, nil
}

func officeParse(officeFile []byte) (*metadataplus.MetaData, error) {
	r, err := zip.NewReader(bytes.NewReader(officeFile), int64(len(officeFile)))
	if err != nil {
		return nil, err
	}

	metadata, err := metadataplus.GetMetadata(r)
	if err != nil {
		return nil, err
	}

	return metadata, nil
}
