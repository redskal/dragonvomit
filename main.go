/*
 * Dragon Vomit v0.1
 * @sam_phisher
 *
 * Automatically dorks files on Bing and/or Google, grabs the file and
 * inspects it for interesting metadata that may be useful for social
 * engineering and/or red team engagements.
 *
 * Everything is done in-memory, so you'll need to download more RAM for
 * the more bountiful targets.
 */
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/redskal/dragonvomit/pkg/settings"
)

var (
	banner string = `┏━━┓
┗┓┓┣┳┳━┓┏━┳━┳━┳┓
┏┻┛┃┏┫╋┗┫╋┃╋┃┃┃┃
┗━━┻┛┗━━╋┓┣━┻┻━┛
╋╋╋╋ ╋╋╋┗━┛ ╋╋ ╋
╋ ╋╋┏┓╋┏┓╋╋╋╋┏┳┓
╋╋ ╋┃┗┳┛┣━┳━━╋┫┗┓
 ╋╋╋┗┓┃┏┫╋┃┃┃┃┃┏┫
╋╋ ╋╋┗━┛┗━┻┻┻┻┻━┛
    @sam_phisher

`
	usage string = `Usage:
    First, configure your API keys.
        dragonvomit -config "bing=111111,googleKey=222222,googleId=333333"

    Search for files and wait.
        dragonvomit -search "example.com" -extensions "docx,xlsx,pptx"

    Pray, and it might return names, usernames, emails, etc.

    Options:
        -silent             Only show results. No banner or progress updates.
        -config <string>    Set your API keys, etc. "bing" = Bing key, "googleKey" = Google API key,
                            "googleId" = Google Custom Search Engine ID
        -search <domain>    The domain to dork against
        -extensions <list>  Comma-separated list of file types to dork for
                            Currently supports (and dorks by default):
                            xlsx, xlsm, xltx, xltm, docx, docm, dotm, dotx, ppt, pptx, potm, potx, pdf
        -threads <int>      Number of threads to use for downloading and analysing documents. [default = 50]
        -json <filename>    Export findings to the named file in JSON format.

    WARNING: using the default extension list will deplete your Google API limit and rack up your Bing bill
             pretty quickly.
`
	silent bool = false
)

func main() {
	quietPtr := flag.Bool("silent", false, "Only show results")
	configPtr := flag.String("config", "", "Set API keys")
	searchPtr := flag.String("search", "", "Domain to search for")
	extensionsPtr := flag.String("extensions", "xlsx,xlsm,xltx,xltm,docx,docm,dotm,dotx,ppt,pptx,potm,potx,pdf", "Comma-separated list of file extensions to dork")
	jsonExportPtr := flag.String("json", "", "Export to the named JSON file.")
	threadCount := flag.Int("threads", 50, "Amount of threads to use for pulling and analysing documents")
	flag.Usage = func() {
		fmt.Print(usage)
		os.Exit(0)
	}
	flag.Parse()

	if *quietPtr {
		silent = true
	}

	// obligitory ascii art
	if !silent {
		fmt.Print(banner)
	}

	if *configPtr == "" && *searchPtr == "" {
		flag.Usage()
		os.Exit(1)
	}

	// grab user's home directory to use for settings and temporary files
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("unable to determine user home directory")
	}

	// setup the Dragon Vomit directory, etc.
	var dragonVomitDir string
	if runtime.GOOS == "windows" {
		dragonVomitDir = filepath.Join(homeDir, "DragonVomit")
	} else {
		// i hate a cluttered home directory in Linux
		dragonVomitDir = filepath.Join(homeDir, ".DragonVomit")
	}
	err = os.MkdirAll(dragonVomitDir, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	settingsFile := filepath.Join(dragonVomitDir, "settings.json")

	// do we want to configure keys?
	if *configPtr != "" {
		if err := settings.SetUserSettings(*configPtr, settingsFile); err != nil {
			log.Fatal(err)
		}
		if *searchPtr == "" {
			// no point going any further...
			os.Exit(0)
		}
	}

	// does the config file exist?
	if _, err := os.Stat(settingsFile); err != nil {
		log.Fatal("no configuration present. run with -config to generate one.")
	}

	// read the current settings
	settings, err := settings.ReadUserSettings(settingsFile)
	if err != nil {
		log.Fatal(err)
	}

	// split file extensions into slice and dork each one
	returnedUrls := make(chan dorkResult)
	tracker := make(chan empty)

	// a nice, ugly way of calculating the tracker count
	// so we can wait for threads and not get race conditions.
	multiplier := 0
	if settings.BingKey != "" {
		multiplier++
	}
	if settings.GoogleKey != "" && settings.GoogleId != "" {
		multiplier++
	}

	// TODO: implement a sync.Map for recording results. We can use the
	// keys to add unqiue items, and record their source as the value.
	dorkTrackerCount := len(strings.Split(*extensionsPtr, ",")) * multiplier
	for _, currentExtension := range strings.Split(*extensionsPtr, ",") {
		// only run the dorks that have been configured
		// disable Bing during testing to avoid bankruptcy
		if settings.BingKey != "" {
			go bingDork(settings.BingKey, *searchPtr, currentExtension, returnedUrls, tracker)
		}
		if settings.GoogleKey != "" && settings.GoogleId != "" {
			go googleDork(settings.GoogleKey, settings.GoogleId, *searchPtr, currentExtension, returnedUrls, tracker)
		}
	}

	// get a de-duplicated list of URLs to investigate
	var dedupedReturnedUrls []string
	go func() {
		for r := range returnedUrls {
			if !slices.Contains(dedupedReturnedUrls, r.url) {
				dedupedReturnedUrls = append(dedupedReturnedUrls, r.url)
				if !silent {
					fmt.Printf("[%s] %s\n", r.searchEngine, r.url)
				}
			}
		}
		var e empty
		tracker <- e
	}()

	// make sure dork routines have finished
	for i := 0; i < dorkTrackerCount; i++ {
		<-tracker
	}

	// clean up and make sure de-duplicate routine is done
	close(returnedUrls)
	<-tracker
	if !silent {
		fmt.Println("[i] Total documents identified:", len(dedupedReturnedUrls))
	}

	var results []analysisResult
	gather := make(chan analysisResult)
	docUrls := make(chan string, *threadCount)

	// start our file processing worker threads
	for i := 0; i < *threadCount; i++ {
		go worker(tracker, gather, docUrls)
	}

	// thread to gather results
	go func() {
		for r := range gather {
			results = append(results, r)
		}
		var e empty
		tracker <- e
	}()

	// add document URLs to channel for workers to process
	for _, r := range dedupedReturnedUrls {
		docUrls <- r
	}

	// clean up the threads
	close(docUrls)
	for i := 0; i < *threadCount; i++ {
		<-tracker
	}
	close(gather)
	<-tracker

	// output pulls unique loot for display.
	if !silent {
		fmt.Println("[i] Printing results:\n")
	}
	finalResults := processResultsToFinal(results)
	// export to JSON?
	if *jsonExportPtr != "" {
		jsonContent, err := json.Marshal(finalResults)
		if err != nil {
			fmt.Println("[!] Unable to marshal results to JSON")
		} else {
			err = os.WriteFile(*jsonExportPtr, jsonContent, 0644)
			if err != nil {
				fmt.Println("[!] Error writing JSON to file")
			}
		}
	}

	_ = printResults(finalResults)

}

// processResultsToFinal creates one large struct from all
// results for easier output formatting through tabwriter
func processResultsToFinal(results []analysisResult) (r FinalResult) {
	// a long, ugly process but makes it easier to output clean tables
	for _, result := range results {
		// get the file name
		urlParts := strings.Split(result.url, "/")
		dorkedFileName := urlParts[len(urlParts)-1]
		dorkedFileName, _ = url.QueryUnescape(dorkedFileName)

		// process External links
		if len(result.ExternalLinks) > 0 {
			for _, externalLink := range result.ExternalLinks {
				addition := ExternalLink{
					ExternalLink: externalLink,
					FileName:     dorkedFileName,
					FileUrl:      result.url,
				}
				r.ExternalLinks = append(r.ExternalLinks, addition)
			}
		}

		// process Image links
		if len(result.ImageLinks) > 0 {
			for _, imageLink := range result.ImageLinks {
				addition := ImageLink{
					ImageLink: imageLink,
					FileName:  dorkedFileName,
					FileUrl:   result.url,
				}
				r.ImageLinks = append(r.ImageLinks, addition)
			}
		}

		// process file paths
		if len(result.FilePaths) > 0 {
			for _, filePath := range result.FilePaths {
				addition := FilePath{
					FilePath: filePath,
					FileName: dorkedFileName,
					FileUrl:  result.url,
				}
				r.FilePaths = append(r.FilePaths, addition)
			}
		}

		// process printers
		if len(result.Printers) > 0 {
			for _, printer := range result.Printers {
				addition := Printer{
					Printer:  printer,
					FileName: dorkedFileName,
					FileUrl:  result.url,
				}
				r.Printers = append(r.Printers, addition)
			}
		}

		// process hostnames
		if len(result.Hostnames) > 0 {
			for _, hostname := range result.Hostnames {
				addition := Hostname{
					Hostname: hostname,
					FileName: dorkedFileName,
					FileUrl:  result.url,
				}
				r.Hostnames = append(r.Hostnames, addition)
			}
		}

		// process email addresses
		if len(result.Emails) > 0 {
			for _, email := range result.Emails {
				addition := Email{
					EmailAddr: email,
					FileName:  dorkedFileName,
					FileUrl:   result.url,
				}
				r.Emails = append(r.Emails, addition)
			}
		}

		// process user's names
		if len(result.Names) > 0 {
			for _, name := range result.Names {
				addition := Name{
					Name:     name,
					FileName: dorkedFileName,
					FileUrl:  result.url,
				}
				r.Names = append(r.Names, addition)
			}
		}

		// process usernames
		if len(result.Usernames) > 0 {
			for _, username := range result.Usernames {
				addition := Username{
					UserName: username,
					FileName: dorkedFileName,
					FileUrl:  result.url,
				}
				r.Usernames = append(r.Usernames, addition)
			}
		}

		// process grepped keywords
		if len(result.GrepKeywords) > 0 {
			for _, grep := range result.GrepKeywords {
				addition := Grepped{
					Value:    grep,
					FileName: dorkedFileName,
					FileUrl:  result.url,
				}
				r.GreppedValues = append(r.GreppedValues, addition)
			}
		}

		// process hidden sheets
		if len(result.HiddenSheets) > 0 {
			for _, hiddensheet := range result.HiddenSheets {
				addition := HiddenSheet{
					SheetName: hiddensheet,
					FileName:  dorkedFileName,
					FileUrl:   result.url,
				}
				r.HiddenSheets = append(r.HiddenSheets, addition)
			}
		}

		// process last saved paths
		if len(result.LastSavedPath) > 0 {
			for _, lsp := range result.LastSavedPath {
				addition := LastSavedPath{
					Path:     lsp,
					FileName: dorkedFileName,
					FileUrl:  result.url,
				}
				r.LastSavedPaths = append(r.LastSavedPaths, addition)
			}
		}

		// process software entries
		if len(result.Software) > 0 {
			for _, software := range result.Software {
				addition := Software{
					Value:    software,
					FileName: dorkedFileName,
					FileUrl:  result.url,
				}
				r.Softwares = append(r.Softwares, addition)
			}
		}

		// process embedded docs flags
		if result.EmbeddedDocs {
			addition := EmbeddedDoc{
				FileName: dorkedFileName,
				FileUrl:  result.url,
			}
			r.EmbeddedDocs = append(r.EmbeddedDocs, addition)
		}

		// process embedded media flags
		if result.EmbeddedMedia {
			addition := EmbeddedMedia{
				FileName: dorkedFileName,
				FileUrl:  result.url,
			}
			r.EmbeddedMedias = append(r.EmbeddedMedias, addition)
		}
	}

	return
}

// printResults prints final result struct to stdout
func printResults(results FinalResult) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 6, 8, ' ', 0)

	// print files with embedded docs
	if len(results.EmbeddedDocs) > 0 {
		fmt.Fprintf(w, "%s\t%s\t%s\n", "Containinng Embedded Docs", "Dorked File Name", "Dorked URL")
		fmt.Fprintf(w, "%s\t%s\t%s\n", "=========================", "================", "==========")
		for _, embedocs := range results.EmbeddedDocs {
			fmt.Fprintf(w, "%s\t\"%s\"\t%.45s...\n", "", embedocs.FileName, embedocs.FileUrl)
		}
		w.Flush()
		fmt.Println()
	}

	// print files with embedded media
	if len(results.EmbeddedMedias) > 0 {
		fmt.Fprintf(w, "%s\t%s\t%s\n", "Containing Embedded Media", "Dorked File Name", "Dorked URL")
		fmt.Fprintf(w, "%s\t%s\t%s\n", "=========================", "================", "==========")
		for _, embedmedia := range results.EmbeddedMedias {
			fmt.Fprintf(w, "%s\t\"%s\"\t%.45s...\n", "", embedmedia.FileName, embedmedia.FileUrl)
		}
		w.Flush()
		fmt.Println()
	}

	// print external links table
	if len(results.ExternalLinks) > 0 {
		fmt.Fprintf(w, "%s\t%s\t%s\n", "External Link", "Dorked File Name", "Dorked URL")
		fmt.Fprintf(w, "%s\t%s\t%s\n", "=============", "================", "==========")
		for _, externalLink := range results.ExternalLinks {
			fmt.Fprintf(w, "%s\t\"%s\"\t%.45s...\n", externalLink.ExternalLink, externalLink.FileName, externalLink.FileUrl)
		}
		w.Flush()
		fmt.Println() // space things a bit
	}

	// print image links table
	if len(results.ImageLinks) > 0 {
		fmt.Fprintf(w, "%s\t%s\t%s\n", "Image Link", "Dorked File Name", "Dorked URL")
		fmt.Fprintf(w, "%s\t%s\t%s\n", "==========", "================", "==========")
		for _, imageLink := range results.ImageLinks {
			fmt.Fprintf(w, "%s\t\"%s\"\t%.45s...\n", imageLink.ImageLink, imageLink.FileName, imageLink.FileUrl)
		}
		w.Flush()
		fmt.Println()
	}

	// print file paths
	if len(results.FilePaths) > 0 {
		fmt.Fprintf(w, "%s\t%s\t%s\n", "File Paths", "Dorked File Name", "Dorked URL")
		fmt.Fprintf(w, "%s\t%s\t%s\n", "==========", "================", "==========")
		for _, filePath := range results.FilePaths {
			fmt.Fprintf(w, "\"%s\"\t\"%s\"\t%.45s...\n", filePath.FilePath, filePath.FileName, filePath.FileUrl)
		}
		w.Flush()
		fmt.Println()
	}

	// print last saved paths
	if len(results.LastSavedPaths) > 0 {
		fmt.Fprintf(w, "%s\t%s\t%s\n", "Last Saved Path", "Dorked File Name", "Dorked URL")
		fmt.Fprintf(w, "%s\t%s\t%s\n", "===============", "================", "==========")
		for _, lsp := range results.LastSavedPaths {
			fmt.Fprintf(w, "\"%s\"\t\"%s\"\t%.45s...\n", lsp.Path, lsp.FileName, lsp.FileUrl)
		}
		w.Flush()
		fmt.Println()
	}

	// print printer details
	if len(results.Printers) > 0 {
		fmt.Fprintf(w, "%s\t%s\t%s\n", "Printer", "Dorked File Name", "Dorked URL")
		fmt.Fprintf(w, "%s\t%s\t%s\n", "=======", "================", "==========")
		for _, printer := range results.Printers {
			fmt.Fprintf(w, "%s\t\"%s\"\t%.45s...\n", printer.Printer, printer.FileName, printer.FileUrl)
		}
		w.Flush()
		fmt.Println()
	}

	// print hidden sheets
	if len(results.HiddenSheets) > 0 {
		fmt.Fprintf(w, "%s\t%s\t%s\n", "Hidden Sheet", "Dorked File Name", "Dorked URL")
		fmt.Fprintf(w, "%s\t%s\t%s\n", "============", "================", "==========")
		for _, hiddensheet := range results.HiddenSheets {
			fmt.Fprintf(w, "%s\t\"%s\"\t%.45s...\n", hiddensheet.SheetName, hiddensheet.FileName, hiddensheet.FileUrl)
		}
		w.Flush()
		fmt.Println()
	}

	// print software in use
	if len(results.Softwares) > 0 {
		fmt.Fprintf(w, "%s\t%s\t%s\n", "Software", "Dorked File Name", "Dorked URL")
		fmt.Fprintf(w, "%s\t%s\t%s\n", "========", "================", "==========")
		for _, software := range results.Softwares {
			fmt.Fprintf(w, "%s\t\"%s\"\t%.45s...\n", software.Value, software.FileName, software.FileUrl)
		}
		w.Flush()
		fmt.Println()
	}

	// print hostnames
	if len(results.Hostnames) > 0 {
		fmt.Fprintf(w, "%s\t%s\t%s\n", "Hostname", "Dorked File Name", "Dorked URL")
		fmt.Fprintf(w, "%s\t%s\t%s\n", "========", "================", "==========")
		for _, hostname := range results.Hostnames {
			fmt.Fprintf(w, "%s\t\"%s\"\t%.45s...\n", hostname.Hostname, hostname.FileName, hostname.FileUrl)
		}
		w.Flush()
		fmt.Println()
	}

	// print grep results
	if len(results.GreppedValues) > 0 {
		fmt.Fprintf(w, "%s\t%s\n", "Grep Result", "Dorked URL")
		fmt.Fprintf(w, "%s\t%s\n", "===========", "==========")
		for _, grep := range results.GreppedValues {
			fmt.Fprintf(w, "%s\t%.45s...\n", grep.Value, grep.FileUrl)
		}
		w.Flush()
		fmt.Println()
	}

	// print names of users
	if len(results.Names) > 0 {
		fmt.Fprintf(w, "%s\t%s\t%s\n", "Name", "Dorked File Name", "Dorked URL")
		fmt.Fprintf(w, "%s\t%s\t%s\n", "====", "================", "==========")
		for _, name := range results.Names {
			fmt.Fprintf(w, "%s\t\"%s\"\t%.45s...\n", name.Name, name.FileName, name.FileUrl)
		}
		w.Flush()
		fmt.Println()
	}

	if len(results.Emails) > 0 {
		fmt.Fprintf(w, "%s\t%s\t%s\n", "Email Address", "Dorked File Name", "Dorked URL")
		fmt.Fprintf(w, "%s\t%s\t%s\n", "=============", "================", "==========")
		for _, email := range results.Emails {
			fmt.Fprintf(w, "%s\t\"%s\"\t%.45s...\n", email.EmailAddr, email.FileName, email.FileUrl)
		}
		w.Flush()
		fmt.Println()
	}

	if len(results.Usernames) > 0 {
		fmt.Fprintf(w, "%s\t%s\t%s\n", "Username", "Dorked File Name", "Dorked URL")
		fmt.Fprintf(w, "%s\t%s\t%s\n", "========", "================", "==========")
		for _, user := range results.Usernames {
			fmt.Fprintf(w, "%s\t\"%s\"\t%.45s...\n", user.UserName, user.FileName, user.FileUrl)
		}
		w.Flush()
		fmt.Println()
	}

	return nil
}

// worker processes each file it pulls from the docUrls channel, extracting
// metadata and returning results to the gather channel.
func worker(tracker chan empty, gather chan analysisResult, docUrls chan string) {
	// TODO: process each URL. Identify file type, and send to necessary
	//       function to extract metadata.
	re := regexp.MustCompile(`(\.[a-zA-Z]*)$`)

	for url := range docUrls {
		matches := re.FindStringSubmatch(url)
		var extension string
		if len(matches) > 0 {
			extension = matches[0]
		} else {
			extension = ""
		}
		result := analysisResult{
			url:      url,
			fileType: strings.ToLower(extension),
		}

		if !silent {
			fmt.Println("[i] Processing file:", result.url)
		}

		resp, err := http.Get(result.url)
		if err != nil {
			if !silent {
				fmt.Println("[!] Failed to fetch:", result.url)
				continue
			}
		}

		buf, err := io.ReadAll(resp.Body)
		if err != nil {
			if !silent {
				fmt.Println("[!] Failed to read content of:", result.url)
				continue
			}
		}

		switch result.fileType {
		case ".pdf":
			// Process buf as a PDF
			author, software, err := pdfParse(buf)
			if err != nil {
				if !silent {
					fmt.Println("[!] Error processing PDF file:", result.url)
				}
				resp.Body.Close()
				continue
			}
			if author != "" {
				result.Names = append(result.Names, author)
			}
			if len(software) > 0 {
				result.Software = append(result.Software, software...)
			}

		case ".docx", ".xlsx", ".pptx":
			// process as an office document
			metadata, err := officeParse(buf)
			if err != nil {
				if !silent {
					fmt.Println("[!] Error processing Office file:", result.url)
				}
				resp.Body.Close()
				continue
			}

			// process metadata out into analysisResult
			// probably a cleaner way to do this, but meh.
			result.ExternalLinks = append(result.ExternalLinks, metadata.ExternalLinks...)
			result.ImageLinks = append(result.ImageLinks, metadata.ImageLinks...)
			result.FilePaths = append(result.FilePaths, metadata.FilePaths...)
			result.Printers = append(result.Printers, metadata.Printers...)
			result.Hostnames = append(result.Hostnames, metadata.Hostnames...)
			result.Emails = append(result.Emails, metadata.Emails...)
			result.Names = append(result.Names, metadata.Names...)
			result.Usernames = append(result.Usernames, metadata.Usernames...)
			result.GrepKeywords = append(result.GrepKeywords, metadata.GrepKeywords...)
			result.HiddenSheets = append(result.HiddenSheets, metadata.HiddenSheets...)
			result.LastSavedPath = append(result.LastSavedPath, metadata.LastSavedPath...)
			result.Software = append(result.Software, fmt.Sprintf("Office %s", metadata.AppProperties.GetMajorVersion()))
			result.EmbeddedDocs = metadata.EmbeddedDocs
			result.EmbeddedMedia = metadata.EmbeddedMedia

		}

		// explicitly close - if we defer it stays open while
		// the channel is opening. I want to keep as small of
		// a memory footprint as possible.
		resp.Body.Close()
		gather <- result
	}

	var e empty
	tracker <- e
}
