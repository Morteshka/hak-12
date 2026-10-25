package service

import (
	"errors"
	"net/url"
)

// validHTTPURL — абсолютная ссылка http/https с хостом.
func validHTTPURL(v string, maxLen int) error {
	if len(v) > maxLen {
		return errors.New("too long")
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("bad url")
	}
	return nil
}
