package iscsi

import (
	"fmt"
	"strconv"
	"strings"
)

// We support iscsi url's on the form
// iscsi://[<username>%<password>@]<host>[:<port>]/<targetname>/<lun>
// E.g.:
// iscsi://client%secret@192.168.0.254/iqn.2018-02.ru.netangels.cvds:mailstorage/0
type URL struct {
	Portal     string
	Host       string
	Port       int
	Iqn        string
	UniqueName string
	Lun        int
	User       string
	Pass       string
}

func ParseFullURL(rawurl string) (*URL, error) {
	u := URL{}

	var rest string

	if !strings.HasPrefix(rawurl, "iscsi://") {
		return nil, fmt.Errorf("unknown scheme")
	}

	rest = rawurl[len("iscsi://"):]

	// Are there CHAP user and pass ?
	if sep := strings.Index(rest, "@"); sep != -1 {
		var authority string

		authority, rest = rest[:sep], rest[sep+1:]

		if sep := strings.Index(authority, "%"); sep != -1 {
			u.User, u.Pass = authority[:sep], authority[sep+1:]
		} else {
			u.User = authority
		}
	}

	parts := strings.Split(rest, "/")

	if len(parts) != 3 {
		return nil, fmt.Errorf("incorrect format")
	}

	portal, iqn, lun := parts[0], parts[1], parts[2]

	// Host/port
	if sep := strings.LastIndex(portal, ":"); sep != -1 {
		u.Host = portal[:sep]
		if i, err := strconv.Atoi(portal[sep+1:]); err == nil && len(portal[sep+1:]) > 0 {
			u.Port = i
		} else {
			return nil, fmt.Errorf("unable to parse host/port")
		}
	} else {
		u.Host = portal
	}
	if strings.HasPrefix(u.Host, "[") {
		if !strings.HasSuffix(u.Host, "]") {
			return nil, fmt.Errorf("missing ']' in host")
		}
		u.Host = strings.Trim(u.Host, "[]")
	}

	// IQN + unique name
	u.Iqn = iqn

	if sep := strings.Index(iqn, ":"); sep != -1 {
		u.UniqueName = iqn[sep+1:]
	}

	// Lun
	if i, err := strconv.Atoi(lun); err == nil {
		u.Lun = i
	} else {
		return nil, fmt.Errorf("lun should be an integer")
	}

	return &u, nil
}
