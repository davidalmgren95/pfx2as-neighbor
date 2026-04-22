// Package downloader provides functionality to retrieve the latest prefix2AS
// file from the CAIDA RouteViews dataset. It navigates the directory structure
// of the dataset, identifies the most recent file, and returns an
// io.ReadCloser for that file.
package downloader

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"time"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

const baseURL string = "https://publicdata.caida.org/datasets/routing/routeviews-prefix2as"

// urlArgsOrder is the query string used to sort the directory listing by modification time in ascending order.
const urlArgsOrder string = "?C=M;O=A"

// listOpenDirectory retrieves the content of the specified URL and extracts links that match the provided regular expression pattern.
func listOpenDirectory(url, pattern string) ([]string, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request to %s returned status code %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(pattern)
	matches := re.FindAllSubmatch(body, -1)
	strMatches := make([]string, 0, len(matches))
	for _, match := range matches {
		strMatches = append(strMatches, string(match[1]))
	}

	return strMatches, nil
}

// findLatestPrefix2ASURL navigates the CAIDA RouteViews directory structure to find the URL of the latest prefix2AS file.
func findLatestPrefix2ASURL() (string, error) {
	years, err := listOpenDirectory(baseURL, `href="([0-9]+)/"`)
	if err != nil {
		return "", fmt.Errorf("listing directory %s: %w", baseURL, err)
	}

	if len(years) == 0 {
		return "", fmt.Errorf("no years found in directory %s", baseURL)
	}

	latestYear := years[len(years)-1]

	latestYearURL := fmt.Sprintf("%s/%s", baseURL, latestYear)
	months, err := listOpenDirectory(latestYearURL, `href="([0-9]+)/"`)
	if err != nil {
		return "", fmt.Errorf("listing directory %s: %w", latestYearURL, err)
	}

	if len(months) == 0 {
		return "", fmt.Errorf("no months found in directory %s", latestYearURL)
	}

	latestMonth := months[len(months)-1]

	latestURL := fmt.Sprintf("%s/%s/%s/%s", baseURL, latestYear, latestMonth, urlArgsOrder)
	files, err := listOpenDirectory(latestURL, `href="(routeviews.*pfx2as.gz)"`)
	if err != nil {
		return "", fmt.Errorf("listing directory %s: %w", latestURL, err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no prefix2AS files found in directory %s", latestURL)
	}

	latestFile := files[len(files)-1]

	return fmt.Sprintf("%s/%s/%s/%s", baseURL, latestYear, latestMonth, latestFile), nil
}

// DownloadLatestPrefix2AS retrieves the latest prefix2AS file from the CAIDA
// RouteViews dataset. The caller is responsible for closing the returned body.
func DownloadLatestPrefix2AS() (io.ReadCloser, error) {
	latestFileURL, err := findLatestPrefix2ASURL()
	if err != nil {
		return nil, fmt.Errorf("finding latest prefix2AS URL: %w", err)
	}

	slog.Info("downloading prefix2as file", "url", latestFileURL)
	resp, err := httpClient.Get(latestFileURL)
	if err != nil {
		return nil, fmt.Errorf("downloading file %s: %w", latestFileURL, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request to %s returned status code %d", latestFileURL, resp.StatusCode)
	}

	return resp.Body, nil
}
