package main

import (
	"context"
	"fmt"

	"github.com/redskal/dragonvomit/pkg/bing"
	customsearch "google.golang.org/api/customsearch/v1"
	"google.golang.org/api/option"
)

// googleDork dorks through Google and adds URLs to a channel
func googleDork(apiKey, customSearchId, domain, fileType string, urls chan dorkResult, tracker chan empty) error {
	ctx := context.Background()
	customsearchService, err := customsearch.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		var e empty
		tracker <- e
		return err
	}

	searchQuery := fmt.Sprintf("site:%s & filetype:%s", domain, fileType)

	resp, err := customsearchService.Cse.List().Cx(customSearchId).Q(searchQuery).Do()
	if err != nil {
		var e empty
		tracker <- e
		return err
	}

	if len(resp.Items) == 0 {
		var e empty
		tracker <- e
		return err
	}

	for _, result := range resp.Items {
		urls <- dorkResult{
			searchEngine: "google",
			url:          result.Link,
		}
	}

	var e empty
	tracker <- e
	return nil
}

// bingDork dorks through Bing and adds URLs to a channel
func bingDork(apiKey, domain, fileType string, urls chan dorkResult, tracker chan empty) error {
	bingClient := bing.NewClient(apiKey)
	searchQuery := fmt.Sprintf("site:%s && filetype:%s && instreamset:(url title):%s", domain, fileType, fileType)
	resp, err := bingClient.Search(searchQuery)
	if err != nil {
		var e empty
		tracker <- e
		return err
	}

	if resp.WebPages.TotalEstimatedMatches == 0 {
		var e empty
		tracker <- e
		return fmt.Errorf("[bing] no matches found")
	}
	// write URLs to our output channel
	for _, result := range resp.WebPages.Value {
		urls <- dorkResult{
			searchEngine: "bing",
			url:          result.URL,
		}
	}

	var e empty
	tracker <- e
	return nil
}
