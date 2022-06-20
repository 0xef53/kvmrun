package nbd

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
)

type URI struct {
	Scheme     string
	Authority  string
	Host       string
	Port       int
	ExportName string
}

// nbd://example.com:10809/export
func ParseURI(rawuri string) (*URI, error) {
	u, err := url.Parse(rawuri)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "nbd", "nbds":
	default:
		return nil, fmt.Errorf("unknown NBD scheme: %s", rawuri)
	}

	nbdURI := URI{
		Scheme:     u.Scheme,
		Authority:  u.Host,
		ExportName: filepath.Base(u.Path),
	}

	if h, p, err := net.SplitHostPort(u.Host); err == nil {
		if i, err := strconv.Atoi(p); err == nil {
			nbdURI.Port = i
		} else {
			return nil, fmt.Errorf("NBD port should be an integer")
		}
		nbdURI.Host = h
	} else {
		return nil, err
	}

	return &nbdURI, nil
}
