/*
 * This package is a combination of code from Black Hat Go
 * and my attempt at porting functionality from MetadataPlus
 * by Chris Nevin of NCC Group.
 */
package metadataplus

import (
	"archive/zip"
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

func GetMetadata(r *zip.Reader) (*MetaData, error) {
	var metadata MetaData

	// process all files for metadata
	for _, f := range r.File {
		// checks for embedded media and docs are just boolean
		// flags. we could add an extraction option, but I don't
		// really want that.
		// we could extract images and check EXIF data, but then
		// where do we stop?!

		// embedded docs?
		if strings.Contains(f.Name, "embed") {
			metadata.EmbeddedDocs = true
		}

		// embedded media?
		if strings.Contains(f.Name, "media") {
			metadata.EmbeddedMedia = true
		}

		// OLE files containing file locations and printer details
		if strings.HasSuffix(f.Name, ".bin") {
			// check for file names
			checkFileNames, err := extractFileNamesFromOle(f)
			if err != nil {
				continue
			}
			if len(checkFileNames) > 0 {
				for _, filePath := range checkFileNames {
					// can be shorter, but this inherently de-duplicates
					if !slices.Contains(metadata.FilePaths, filePath) {
						metadata.FilePaths = append(metadata.FilePaths, filePath)
					}
				}

				// use this opportunity to attempt to extract usernames and
				// hostnames from the extracted filenames of the OLE files
				for _, filename := range checkFileNames {
					// usernames
					names, usernames := lookupUsernames(filename)
					for _, name := range names {
						if !slices.Contains(metadata.Names, name) && name != "" {
							metadata.Names = append(metadata.Names, name)
						}
					}
					for _, username := range usernames {
						if !slices.Contains(metadata.Usernames, username) {
							metadata.Usernames = append(metadata.Usernames, username)
						}
					}

					// hostnames
					foundHostnames := lookupHostnames(filename)
					for _, host := range foundHostnames {
						if !slices.Contains(metadata.Hostnames, host) {
							metadata.Hostnames = append(metadata.Hostnames, host)
						}
					}
				}
			}

			// check for printers
			// TODO: I need a valid sample for this.
		}

		// checks for any XML or .rels file
		if strings.Contains(f.Name, "xml") || strings.Contains(f.Name, ".rels") {
			// open the file and decode the XML because we're going
			// to be grepping it...many times
			rc, err := f.Open()
			if err != nil {
				continue
			}
			defer rc.Close()

			// perform some overall grepping. usernames, hostnames, emails, etc.
			// put it in here to cover all bases.
			fileBytes, err := io.ReadAll(rc)
			if err != nil {
				continue
			}

			fileString := string(fileBytes)

			// grep for hostnames
			foundHostnames := lookupHostnames(fileString)
			for _, host := range foundHostnames {
				if !slices.Contains(metadata.Hostnames, host) {
					metadata.Hostnames = append(metadata.Hostnames, host)
				}
			}

			// grep for users
			names, users := lookupUsernames(fileString)
			for _, name := range names {
				if !slices.Contains(metadata.Names, name) && name != "" {
					metadata.Names = append(metadata.Names, name)
				}
			}
			for _, user := range users {
				if !slices.Contains(metadata.Usernames, user) {
					metadata.Usernames = append(metadata.Usernames, user)
				}
			}

			// grep for emails - this should cover occurences like
			// mailto:info@example.com?subject=email%20subject&cc=another@example.com
			// https://go.dev/play/p/5oNBKo3LGvw
			emails, _ := grepStringForRegex(fileString, `([a-zA-Z0-9+._-]+@[a-zA-Z0-9._-]+\.[a-zA-Z0-9_-]+)`)
			for _, email := range emails {
				if !slices.Contains(metadata.Emails, email) {
					metadata.Emails = append(metadata.Emails, email)
				}
			}

			var n Node
			if err := xml.NewDecoder(rc).Decode(&n); err != nil {
				continue
			}

			// process specific XML files - originally taken from Black Hat Go
			switch f.Name {
			case "docProps/core.xml":
				if err := processXml(f, &metadata.CoreProperties); err != nil {
					continue
				}
			case "docProps/app.xml":
				if err := processXml(f, &metadata.AppProperties); err != nil {
					continue
				}
			case "xl/workbook.xml":
				// this one checks for VeryHidden sheets, too.
				findHiddenSheets([]Node{n}, &metadata.HiddenSheets)

				// Grab the last saved path from Excel workbook.
				// May contain usernames or hostnames, too.
				searchForTagAttrValue([]Node{n}, "absPath", "url", &metadata.LastSavedPath)

			}

			// a @sam_phisher original check...
			// found this one during testing. seems to be present
			// if the user adds an object to a Word document, but
			// covering all eventualities.
			if strings.Contains(f.Name, "people") {
				// lists the author of the document
				searchForTagAttrValue([]Node{n}, "person", "author", &metadata.Names)

				// listed an email during testing, but in format of "S::sam.phisher@example.com"
				// could create a custom function for this as "providerId" attr is also
				// interesting.
				searchForTagAttrValue([]Node{n}, "presenceInfo", "userId", &metadata.Emails)
			}

			// another found by myself during testing with PowerPoint files
			if strings.Contains(f.Name, "changes") {
				// under ppt/changeInfos there's XML files which keep
				// note of users that have made changes.
				searchForTagAttrValue([]Node{n}, "chgData", "name", &metadata.Names)
			}

			if strings.Contains(f.Name, "document") {
				// search for image links which may contain file locations.
				// can be used to extract usernames and/or hostnames. we
				// won't necessarily display these to the user unless we can
				// grep users/hosts out.
				searchAllTagsAttrValue([]Node{n}, "descr", &metadata.ImageLinks)
			}

			// search for comment authors
			if strings.Contains(f.Name, "comments") {
				searchForTagContent([]Node{n}, "author", &metadata.Names)
			}

			// run the grepping checks from MetadataPlus that are
			// suitable for any XML or .rels file
			err = grepXmlOrRels(f, &metadata)
			if err != nil {
				continue
			}

		}
	}

	// add these if they're not found already
	if !slices.Contains(metadata.Names, metadata.CoreProperties.Creator) && metadata.CoreProperties.Creator != "" {
		metadata.Names = append(metadata.Names, metadata.CoreProperties.Creator)
	}
	if !slices.Contains(metadata.Names, metadata.CoreProperties.LastModifiedBy) && metadata.CoreProperties.LastModifiedBy != "" {
		metadata.Names = append(metadata.Names, metadata.CoreProperties.LastModifiedBy)
	}

	// last pass over .Names and .Filepaths to dig out usernames
	// and hostnames in case we missed anything
	for _, name := range metadata.Names {
		_, usernames := lookupUsernames(name)
		for _, user := range usernames {
			if !slices.Contains(metadata.Usernames, user) {
				metadata.Usernames = append(metadata.Usernames, user)
			}
		}
	}
	for _, filePath := range metadata.FilePaths {
		_, usernames := lookupUsernames(filePath)
		for _, user := range usernames {
			if !slices.Contains(metadata.Usernames, user) && user != "" {
				metadata.Usernames = append(metadata.Usernames, user)
			}
		}
		hosts := lookupHostnames(filePath)
		for _, host := range hosts {
			if !slices.Contains(metadata.Hostnames, host) {
				metadata.Hostnames = append(metadata.Hostnames, host)
			}
		}
	}

	return &metadata, nil
}

// lookupHostnames will pull hostnames from any UNC paths
// in s
func lookupHostnames(s string) (r []string) {
	re := regexp.MustCompile(`\\\\([a-zA-Z0-9\.-]+)+\\`)
	matches := re.FindAllStringSubmatch(s, -1)
	for _, match := range matches {
		// shouldn't need this length check, but belt and braces...
		if len(match) > 0 {
			r = append(r, match[1])
		}
	}

	return
}

// lookupUsernames will pull any matches to known patterns
// in s
func lookupUsernames(s string) (names, usernames []string) {
	regexes := []string{
		`documents%20and%20settings\\(.*?)\\`,
		`DOCUME~1/(.*?)/`,
		`Users\\(.*?)\\`,
		`Users/(.*?)/`,
	}
	errorStrings := []string{
		"",
		".",
		"<cp:keywords></cp:keywords><dc:description>",
	}

	// herein lies wofty logic
	for _, regex := range regexes {
		re := regexp.MustCompile(regex)
		matches := re.FindAllStringSubmatch(s, -1)
		for _, match := range matches {
			if len(match) > 0 {
				// checks for first regex care of Chris
				// not bothering with the Vista checks because I'm no masochist
				if !slices.Contains(errorStrings, match[1]) {
					// contains a space?
					if strings.Contains(match[1], " ") {
						// starts with space?
						if strings.HasPrefix(match[1], " ") {
							// never seen a username start with a space, but whatever
							usernames = append(usernames, match[1])
						} else {
							names = append(names, match[1])
						}
					} else {
						usernames = append(usernames, match[1])
					}
				}
			}

		}
	}

	return
}

// extractStringsFromFile is a modified version of
// https://github.com/robpike/strings/blob/master/strings.go#L62.
// Now returns a []string of strings found in f.
func extractStringsFromFile(f io.ReadCloser) ([]string, error) {
	min := 6
	max := 256
	var returnSlice []string

	in := bufio.NewReader(f)
	str := make([]rune, 0, 256)
	filePos := int64(0)

	addToReturnSlice := func() {
		if len(str) >= min {
			s := string(str)
			returnSlice = append(returnSlice, s)
		}
		str = str[0:0]
	}

	for {
		var (
			r   rune
			wid int
			err error
		)
		// One string per loop.
		for ; ; filePos += int64(wid) {
			r, wid, err = in.ReadRune()
			if err == io.EOF {
				return returnSlice, nil
			} else if err != nil {
				return nil, err
			}
			if !strconv.IsPrint(r) && r >= 0xFF {
				addToReturnSlice()
				continue
			}
			// It's printable. Keep it.
			if len(str) >= max {
				addToReturnSlice()
			}
			str = append(str, r)
		}
	}
}

func extractFileNamesFromOle(f *zip.File) ([]string, error) {
	var fileNames []string
	regexes := []string{
		// local files
		`[a-zA-Z]:[\\\/](?:[a-zA-Z0-9]+[\\\/])*([a-zA-Z0-9-]+\.[a-zA-Z0-9]+)`,
		// UNC path...hopefully
		`(\\\\)+([a-zA-Z0-9\.-]+[\\])*([a-zA-Z0-9-\.\$\\)*([a-zA-Z0-9-]+\.[a-zA-Z0-9]+)`,
	}

	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	stringsInFile, err := extractStringsFromFile(rc)
	if err != nil {
		return nil, err
	}

	// for each string we got, test against each regex
	for _, s := range stringsInFile {
		for _, regex := range regexes {
			re := regexp.MustCompile(regex)
			matches := re.FindStringSubmatch(s)
			if len(matches) > 0 {
				fileNames = append(fileNames, matches[0])
			}
		}
	}

	return fileNames, nil
}

// processXml grabs basic data from an XML file in an Office document.
func processXml(f *zip.File, prop interface{}) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	if err := xml.NewDecoder(rc).Decode(&prop); err != nil {
		return err
	}
	return nil
}

// fileContainsWord returns true if f contains grepWord, otherwise false
func fileContainsWord(f *zip.File, grepWord string) (bool, error) {
	rc, err := f.Open()
	if err != nil {
		return false, err
	}
	defer rc.Close()

	fileBytes, err := io.ReadAll(rc)
	if err != nil {
		return false, err
	}

	fileString := string(fileBytes)
	if strings.Contains(fileString, grepWord) {
		return true, nil
	}
	return false, nil
}

// grepStringForRegex extracts data from regex matches in f
func grepStringForRegex(s string, regEx string) ([]string, error) {
	var r []string
	re := regexp.MustCompile(regEx)
	matches := re.FindAllStringSubmatch(s, -1)
	for _, match := range matches {
		if !slices.Contains(r, match[1]) {
			r = append(r, match[1])
		}
	}

	return r, nil
}

// grepXmlForKeyword recurses through XML for the first instance of
// either an XML tag like <keyword> or a field containing keyword
// Note: this is a modified version of the top answer here:
// https://stackoverflow.com/questions/30256729/how-to-traverse-through-xml-data-in-golang
func grepXmlForKeyword(nodes []Node, keyword string, s *string) {
	// this could probably be vastly improved by making s a []string,
	// and appending findings to it. But that requires way more
	// logic than my puny mind can handle.
	for _, n := range nodes {
		if strings.Contains(n.XMLName.Local, keyword) {
			// grab the XML entry
			*s = fmt.Sprintf("<%s>%s</%s>", n.XMLName.Local, string(n.Content), n.XMLName.Local)
			break
		} else if strings.Contains(string(n.Content), keyword) {
			// grab the content of the XML entry, but keep looking
			// incase we're not inside the last tag level
			*s = fmt.Sprintf("<%s>%s</%s>", n.XMLName.Local, string(n.Content), n.XMLName.Local)
			grepXmlForKeyword(n.Nodes, keyword, s)
			// the break stops us from being like Elsa in Ahtohallan
			// and going too deep, which causes us to return nothing
			break
		} /* else {
			// keep recursing
			grepXmlForKeyword(n.Nodes, keyword, s)
		}*/
	}
}

// findHiddenSheets extracts the name of any hidden or very hidden
// sheets. Right-click sheets -> Unhide for hidden sheets. Go to Visual
// Basic Editor in Developer ribbon and change Visible property for
// VeryHidden sheets.
func findHiddenSheets(nodes []Node, s *[]string) {
	for _, n := range nodes {
		if n.XMLName.Local == "sheet" && len(n.Attrs) > 0 {
			// can't think of a good way of pulling all this in one sweep
			var hidden bool
			var state string
			for _, v := range n.Attrs {
				if v.Name.Local == "state" && (v.Value == "hidden" || v.Value == "veryHidden") {
					hidden = true
					state = v.Value
				}
			}
			if hidden {
				for _, v := range n.Attrs {
					if v.Name.Local == "name" {
						*s = append(*s, fmt.Sprintf("%s - (%s)", v.Value, state))
					}
				}
			}
		} else {
			findHiddenSheets(n.Nodes, s)
		}
	}
}

// searchExternalLinks finds any Target attr value where TargetMode
// attr is set to External.
func searchExternalLinks(nodes []Node, s *[]string) {
	for _, n := range nodes {
		if len(n.Attrs) > 0 {
			var external bool
			var link string
			// extract Target and whether it's external
			for _, v := range n.Attrs {
				if v.Name.Local == "Target" {
					link = v.Value
				}
				if v.Name.Local == "TargetMode" && v.Value == "External" {
					external = true
				}
			}
			// if we have a link, it's external and not already found...
			if link != "" && external && !slices.Contains(*s, link) {
				*s = append(*s, link)
			}
			searchExternalLinks(n.Nodes, s)
		} else {
			searchExternalLinks(n.Nodes, s)
		}
	}
}

// searchForTagContent searches XML for the content of any tag named
// tagTitle. Eg tagTitle = "Test" pulls "testing" from "<Test>testing</Test>"
func searchForTagContent(nodes []Node, tagTitle string, s *[]string) {
	for _, n := range nodes {
		if n.XMLName.Local == tagTitle {
			if !slices.Contains(*s, string(n.Content)) {
				*s = append(*s, string(n.Content))
			}
		} else {
			searchForTagContent(n.Nodes, tagTitle, s)
		}
	}
}

// searchForTagAttrValue grabs the value of attribute named attrName from
// any tag named tagTitle.
func searchForTagAttrValue(nodes []Node, tagTitle, attrName string, s *[]string) {
	for _, n := range nodes {
		if n.XMLName.Local == tagTitle {
			if len(n.Attrs) > 0 {
				for _, v := range n.Attrs {
					if v.Name.Local == attrName && !slices.Contains(*s, v.Value) && v.Value != "" {
						*s = append(*s, v.Value)
					}
				}
			}
		} else {
			searchForTagAttrValue(n.Nodes, tagTitle, attrName, s)
		}
	}
}

// searchAllTagsAttrValue searches through all tags for any of the specified
// attributes named attrName and pulls values.
func searchAllTagsAttrValue(nodes []Node, attrName string, s *[]string) {
	for _, n := range nodes {
		if len(n.Attrs) > 0 {
			for _, v := range n.Attrs {
				if v.Name.Local == attrName && !slices.Contains(*s, v.Value) {
					*s = append(*s, v.Value)
				}
			}
		} else {
			searchAllTagsAttrValue(n.Nodes, attrName, s)
		}
	}
}

// grepXmlOrRels runs checks suitable for all .xml or .rels files
func grepXmlOrRels(f *zip.File, metadata *MetaData) error {
	// open the file ready to grep for goodies
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("unable to open file")
	}
	defer rc.Close()

	// decode XML to Node
	var n Node
	if err := xml.NewDecoder(rc).Decode(&n); err != nil {
		return fmt.Errorf("could not decode XML")
	}

	// we could expand on this in the future
	keywords := []string{
		"password",
	}

	// grep the file for keywords, and add findings to MetaData struct
	for _, keyword := range keywords {
		// check if keyword is in file to avoid unnecessary overheads
		// of the recursive grep
		if containsKeyword, _ := fileContainsWord(f, keyword); containsKeyword {
			var s string
			grepXmlForKeyword([]Node{n}, keyword, &s)
			if s != "" {
				metadata.GrepKeywords = append(metadata.GrepKeywords, s)
			}
		}
	}

	// file paths
	searchForTagAttrValue([]Node{n}, "absPath", "url", &metadata.FilePaths)

	// external links - search all tags, not just <Relationships>
	// Note: I've tried to refine Chris' approach by finding only
	//       targets with TargetMode=External
	//       I've also removed the file name check because of the
	//       extra check, which should solve the same issue.
	searchExternalLinks([]Node{n}, &metadata.ExternalLinks)

	return nil
}
