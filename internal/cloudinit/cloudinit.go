package cloudinit

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

type Data struct {
	Metadata *MetadataConfig
	Network  *NetworkConfig
}

type MetadataConfig struct {
	DSMode     string `json:"dsmode" yaml:"dsmode"`
	InstanceID string `json:"instance-id" yaml:"instance-id"`
}

type NetworkConfig struct {
	Version   int                       `json:"version" yaml:"version"`
	Ethernets map[string]EthernetConfig `json:"ethernets" yaml:"ethernets"`
}

type EthernetConfig struct {
	Match struct {
		MacAddress string `json:"mac_address" yaml:"mac_address"`
	} `json:"match" yaml:"match"`
	Addresses []string `json:"addresses" yaml:"addresses"`
	Gateway4  string   `json:"gateway4,omitempty" yaml:"gateway4,omitempty"`
	Gateway6  string   `json:"gateway6,omitempty" yaml:"gateway6,omitempty"`
}

func GenImage(data *Data, outfile string) error {
	if len(filepath.Base(outfile)) == 0 {
		return fmt.Errorf("empty output file")
	}

	genisoimageBinary, err := exec.LookPath("genisoimage")
	if err != nil {
		return err
	}

	tmpdir, err := ioutil.TempDir(filepath.Dir(outfile), ".cidata-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	imageFile := filepath.Join(tmpdir, "image")
	userFile := filepath.Join(tmpdir, "user-data")
	metaFile := filepath.Join(tmpdir, "meta-data")
	netFile := filepath.Join(tmpdir, "network-config")

	if err := ioutil.WriteFile(userFile, []byte("#cloud-config\n"), 0644); err != nil {
		return err
	}

	if b, err := yaml.Marshal(data.Metadata); err == nil {
		if err := ioutil.WriteFile(metaFile, b, 0644); err != nil {
			return err
		}
	} else {
		return err
	}

	if b, err := yaml.Marshal(data.Network); err == nil {
		if err := ioutil.WriteFile(netFile, b, 0644); err != nil {
			return err
		}
	} else {
		return err
	}

	opts := []string{
		"-output", imageFile,
		"-volid", "cidata",
		"-joliet",
		"-rock",
		userFile,
		metaFile,
		netFile,
	}

	out, err := exec.Command(genisoimageBinary, opts...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("genisoimage failed (%s): %s", err, strings.TrimSpace(string(out)))
	}

	return os.Rename(imageFile, outfile)
}
