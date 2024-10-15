package osprober

import "os"

var (
	PROBERS = []Prober{
		OsReleaseProber{},
		DebianVersionProber{},
		CentosReleaseProber{},
		DefaultProber{},
	}
)

type OSReleaseInfo struct {
	Source     string `json:"source"`
	Family     string `json:"family"`
	Distrib    string `json:"distrib"`
	Version    string `json:"version"`
	CodeName   string `json:"codename"`
	Name       string `json:"name"`
	PrettyName string `json:"pretty_name"`
}

func Probe(rootdir string) (*OSReleaseInfo, error) {
	var info *OSReleaseInfo

	for _, p := range PROBERS {
		i, err := p.Probe(rootdir)

		switch {
		case err == nil:
		case os.IsNotExist(err):
			continue
		default:
			return nil, err
		}

		info = i

		break
	}

	return info, nil
}

type Prober interface {
	Probe(string) (*OSReleaseInfo, error)
}

type DefaultProber struct{}

func (p DefaultProber) probe() (*OSReleaseInfo, error) {
	return nil, nil
}

func (p DefaultProber) Probe(rootdir string) (*OSReleaseInfo, error) {
	return p.probe()
}
