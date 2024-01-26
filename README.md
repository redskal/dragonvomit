[![Go](https://github.com/redskal/dragonvomit/actions/workflows/go.yml/badge.svg)](https://github.com/redskal/dragonvomit/actions/workflows/go.yml)

![Dragon Vomit](assets/logo.png)

#### Overview
A multi-threaded dorking tool that finds and analyses documents for
interesting metadata that can be useful in various engagement types.

Currently uses Bing and Google to dork for Office documents and PDFs.
It then processes them using techniques from [MetadataPlus](https://github.com/nccgroup/MetadataPlus) (with some
additions by me) and basic PDF parsing to look for user's names,
usernames, emails, hostnames, hidden sheets, etc, etc.

NOTE: the dorking is passive, but the requests to grab the docs are
very much active and _not_ rate-limited. Keep that in mind if stealth
is required.

NOTE: Bing gets expensive quickly with the default extensions list.
Google is free for up to 100 searches/day which gets used
quickly, too. (It uses a search per file extension.)
#### Install
Requirements:
- Go version 1.21+
- Bing API key, and/or
- [Google Custom Search API key](https://developers.google.com/custom-search/v1/overview)

To install just run the following command:
```bash
go install github.com/redskal/dragonvomit@latest
```
#### Usage
```
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
```
#### Todo's
List of items to add or improve:
- Parse printer information from Office docs
- Refactor the code - it's a bit "added as I went"
- Implement some missing checks from [FOCA](https://github.com/ElevenPaths/FOCA)
#### License
Released under the [#YOLO Public License](https://github.com/YOLOSecFW/YoloSec-Framework/blob/master/YOLO%20Public%20License)