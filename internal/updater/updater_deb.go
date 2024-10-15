package updater

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type updater_DEB struct {
	url         *url.URL
	installOnly bool
}

func (u *updater_DEB) Run() error {
	var debfile string

	// Get
	if u.url.Scheme == "" {
		fmt.Fprintf(Writer, "--> Using an existing %s\n", u.url.Path)

		debfile = u.url.Path
	} else {
		fmt.Fprintf(Writer, "--> Downloading %s ...\n", u.url.String())

		tmpdir, err := os.MkdirTemp("", ".kvmrun_updater-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpdir)

		debfile = filepath.Join(tmpdir, fmt.Sprintf("kvmrun.%d.deb", os.Getpid()))

		if err := download(u.url.String(), debfile); err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
	}

	// Validate
	pkgver, err := u.validate(debfile)
	if err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	// Install
	fmt.Fprintf(Writer, "--> Installing the package version %s\n", pkgver)

	if err := u.install(debfile); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	if u.installOnly {
		fmt.Fprintf(Writer, "\n")
		fmt.Fprintf(Writer, "The kvmrund process will not be restarted\n")

		return nil
	}

	// Restart
	fmt.Fprintf(Writer, "--> Restarting the kvmrund process\n")

	if err := GracefulRestart(); err != nil {
		return fmt.Errorf("restart kvmrund failed: %w", err)
	}

	fmt.Fprintf(Writer, "--> Done\n")

	return nil
}

func (u *updater_DEB) validate(debfile string) (string, error) {
	out, err := exec.Command("/usr/bin/dpkg", "-f", debfile, "Version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("dpkg check error: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return strings.TrimSpace(string(out)), nil
}

func (u *updater_DEB) install(debfile string) error {
	out, err := exec.Command("/usr/bin/dpkg", "-i", "--force-confold", debfile).CombinedOutput()
	if err != nil {
		return fmt.Errorf("dpkg install error: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}
