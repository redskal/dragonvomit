// mostly pulled from Black Hat Go
package metadataplus

import (
	"encoding/xml"
	"strings"
)

type Node struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:"-"`
	Content []byte     `xml:",innerxml"`
	Nodes   []Node     `xml:",any"`
}

func (n *Node) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	n.Attrs = start.Attr
	type node Node

	return d.DecodeElement((*node)(n), &start)
}

type MetaData struct {
	CoreProperties OfficeCoreProperties
	AppProperties  OfficeAppProperty
	ExternalLinks  []string
	ImageLinks     []string
	FilePaths      []string
	Printers       []string
	Hostnames      []string
	Emails         []string
	Names          []string
	Usernames      []string
	GrepKeywords   []string
	HiddenSheets   []string
	LastSavedPath  []string
	EmbeddedDocs   bool
	EmbeddedMedia  bool
}

type OfficeCoreProperties struct {
	XMLName        xml.Name `xml:"coreProperties"`
	Creator        string   `xml:"creator"`
	LastModifiedBy string   `xml:"lastModifiedBy"`
}

type OfficeAppProperty struct {
	XMLName     xml.Name `xml:"Properties"`
	Application string   `xml:"Application"`
	Company     string   `xml:"Company"`
	Version     string   `xml:"AppVersion"`
}

var OfficeVersions = map[string]string{
	"16": "2016",
	"15": "2013",
	"14": "2010",
	"12": "2007",
	"11": "2003",
}

func (a *OfficeAppProperty) GetMajorVersion() string {
	tokens := strings.Split(a.Version, ".")

	if len(tokens) < 2 {
		return "(Version unknown)"
	}
	v, ok := OfficeVersions[tokens[0]]
	if !ok {
		return "(Version unknown)"
	}
	return v
}
