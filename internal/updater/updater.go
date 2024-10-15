package updater

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	pb "github.com/0xef53/kvmrun/api/services/system/v1"
	"github.com/0xef53/kvmrun/internal/grpcclient"
	"github.com/0xef53/kvmrun/internal/osprober"
	"github.com/0xef53/kvmrun/internal/systemd"

	empty "github.com/golang/protobuf/ptypes/empty"
)

var (
	Writer io.Writer = io.Discard
)

type Updater interface {
	Run() error
}

func New(urlstr string, installOnly bool) (Updater, error) {
	u, err := url.Parse(urlstr)
	if err != nil {
		return nil, err
	}

	if len(u.Path) == 0 {
		return nil, fmt.Errorf("no URL to package file specified")
	}

	switch u.Scheme {
	case "http", "https", "":
	default:
		return nil, fmt.Errorf("unknown URL scheme: %s", u.Scheme)
	}

	osinfo, err := osprober.Probe("/")
	if err != nil {
		return nil, err
	}
	if osinfo == nil {
		return nil, fmt.Errorf("unable to determine type of host OS")
	}

	switch osinfo.Family {
	case "debian":
		return &updater_DEB{
			url:         u,
			installOnly: installOnly,
		}, nil
	}

	return nil, fmt.Errorf("unsupported type of host OS")
}

func download(urlstr, filepath string) error {
	fd, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer fd.Close()

	resp, err := http.Get(urlstr)
	if err != nil {
		return fmt.Errorf("fetch error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch error: http code = %s", resp.Status)
	}

	if _, err := io.Copy(fd, resp.Body); err != nil {
		return err
	}

	return nil
}

func GracefulRestart() error {
	// Unix socket client
	conn, err := grpcclient.NewConn("unix:@/run/kvmrund.sock", nil, false)
	if err != nil {
		return err
	}
	defer conn.Close()

	systemctl, err := systemd.NewManager()
	if err != nil {
		return err
	}
	defer systemctl.Close()

	if _, err := pb.NewSystemServiceClient(conn).GracefulShutdown(context.Background(), new(empty.Empty)); err != nil {
		return err
	}

	return systemctl.StartAndTest("kvmrund.service", 5*time.Second, nil)
}
