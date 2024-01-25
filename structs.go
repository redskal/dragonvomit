package main

type empty struct{}

type dorkResult struct {
	searchEngine string
	url          string
}

type analysisResult struct {
	url           string
	fileType      string
	ExternalLinks []string
	ImageLinks    []string
	FilePaths     []string
	Printers      []string
	Hostnames     []string
	Emails        []string
	Names         []string
	Usernames     []string
	GrepKeywords  []string
	HiddenSheets  []string
	LastSavedPath []string
	Software      []string
	EmbeddedDocs  bool
	EmbeddedMedia bool
}

type FinalResult struct {
	ExternalLinks  []ExternalLink  `json:"external_links,omitempty"`
	ImageLinks     []ImageLink     `json:"image_links,omitempty"`
	FilePaths      []FilePath      `json:"file_paths,omitempty"`
	Printers       []Printer       `json:"printers,omitempty"`
	Hostnames      []Hostname      `json:"hostnames,omitempty"`
	Emails         []Email         `json:"emails,omitempty"`
	Names          []Name          `json:"names,omitempty"`
	Usernames      []Username      `json:"usernames,omitempty"`
	GreppedValues  []Grepped       `json:"grepped_values,omitempty"`
	HiddenSheets   []HiddenSheet   `json:"hidden_sheets,omitempty"`
	LastSavedPaths []LastSavedPath `json:"last_saved_paths,omitempty"`
	Softwares      []Software      `json:"software,omitempty"`
	EmbeddedDocs   []EmbeddedDoc   `json:"embedded_docs,omitempty"`
	EmbeddedMedias []EmbeddedMedia `json:"embedded_media,omitempty"`
}

type ExternalLink struct {
	ExternalLink string `json:"external_link,omitempty"`
	FileName     string `json:"file_name,omitempty"`
	FileUrl      string `json:"file_url,omitempty"`
}

type ImageLink struct {
	ImageLink string `json:"image_link,omitempty"`
	FileName  string `json:"file_name,omitempty"`
	FileUrl   string `json:"file_url,omitempty"`
}

type FilePath struct {
	FilePath string `json:"file_path,omitempty"`
	FileName string `json:"file_name,omitempty"`
	FileUrl  string `json:"file_url,omitempty"`
}

type Printer struct {
	Printer  string `json:"printer,omitempty"`
	FileName string `json:"file_name,omitempty"`
	FileUrl  string `json:"file_url,omitempty"`
}

type Hostname struct {
	Hostname string `json:"hostname,omitempty"`
	FileName string `json:"file_name,omitempty"`
	FileUrl  string `json:"file_url,omitempty"`
}

type Email struct {
	EmailAddr string `json:"email_address,omitempty"`
	FileName  string `json:"file_name,omitempty"`
	FileUrl   string `json:"file_url,omitempty"`
}

type Name struct {
	Name     string `json:"name,omitempty"`
	FileName string `json:"file_name,omitempty"`
	FileUrl  string `json:"file_url,omitempty"`
}

type Username struct {
	UserName string `json:"username,omitempty"`
	FileName string `json:"file_name,omitempty"`
	FileUrl  string `json:"file_url,omitempty"`
}

type Grepped struct {
	Value    string `json:"value,omitempty"`
	FileName string `json:"file_name,omitempty"`
	FileUrl  string `json:"file_url,omitempty"`
}

type HiddenSheet struct {
	SheetName string `json:"sheet_name,omitempty"`
	FileName  string `json:"file_name,omitempty"`
	FileUrl   string `json:"file_url,omitempty"`
}

type LastSavedPath struct {
	Path     string `json:"path,omitempty"`
	FileName string `json:"file_name,omitempty"`
	FileUrl  string `json:"file_url,omitempty"`
}

type Software struct {
	Value    string `json:"software,omitempty"`
	FileName string `json:"file_name,omitempty"`
	FileUrl  string `json:"file_url,omitempty"`
}

type EmbeddedDoc struct {
	FileName string `json:"file_name,omitempty"`
	FileUrl  string `json:"file_url,omitempty"`
}

type EmbeddedMedia struct {
	FileName string `json:"file_name,omitempty"`
	FileUrl  string `json:"file_url,omitempty"`
}
