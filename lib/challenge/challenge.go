// Package challenge holds challenge-specific actions that don't fit the generic
// machine/submit helpers — currently downloading a challenge's files.
package challenge

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/0xb0nzi/htb-cli/config"
	"github.com/0xb0nzi/htb-cli/lib/utils"
)

// Download fetches a challenge's files to destPath. The HTB API answers
// /challenge/download/{id} with a 302 to a signed S3 URL; the shared HtbRequest
// client intentionally does not follow redirects (it uses them to detect an
// expired token), so we follow the S3 hop here with a plain client.
func Download(id int, destPath string) (string, error) {
	url := fmt.Sprintf("%s/challenge/download/%d", config.BaseHackTheBoxAPIURL, id)
	resp, err := utils.HtbRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var src io.Reader = resp.Body
	switch resp.StatusCode {
	case http.StatusOK:
		// Body is the file directly.
	case http.StatusFound, http.StatusMovedPermanently, http.StatusTemporaryRedirect:
		loc := resp.Header.Get("Location")
		if loc == "" {
			return "", fmt.Errorf("download redirect had no Location header")
		}
		if strings.Contains(loc, "/login") {
			return "", fmt.Errorf("HTB token appears invalid or expired")
		}
		// The S3 URL is pre-signed, so no auth header is sent; the default
		// client follows any further redirects and validates TLS.
		s3, err := http.Get(loc)
		if err != nil {
			return "", err
		}
		defer s3.Body.Close()
		if s3.StatusCode != http.StatusOK {
			return "", fmt.Errorf("download failed: storage returned status %d", s3.StatusCode)
		}
		src = s3.Body
	default:
		return "", fmt.Errorf("download failed: status %d (challenge may have no files)", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, src); err != nil {
		return "", err
	}
	return destPath, nil
}
