package cloudinit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

var userDataHeader = []byte("#cloud-config\n")

type Data struct {
	Meta    MetadataConfig
	Network NetworkConfig

	// These fields override the same name keys from User structure
	Hostname string
	Domain   string
	Timezone string

	User map[string]interface{}
}

type MetadataConfig struct {
	DSMode     string `json:"dsmode" yaml:"dsmode"`
	InstanceID string `json:"instance-id" yaml:"instance-id"`

	LocalHostname    string `json:"local-hostname,omitempty" yaml:"local-hostname,omitempty"`
	Platform         string `json:"platform,omitempty" yaml:"platform,omitempty"`
	Subplatform      string `json:"subplatform,omitempty" yaml:"subplatform,omitempty"`
	Cloudname        string `json:"cloud-name,omitempty" yaml:"cloud-name,omitempty"`
	Region           string `json:"region,omitempty" yaml:"region,omitempty"`
	AvailabilityZone string `json:"availability-zone,omitempty" yaml:"availability-zone,omitempty"`
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
		return fmt.Errorf("output file must be set")
	}

	genisoimageBinary, err := exec.LookPath("genisoimage")
	if err != nil {
		return err
	}

	if len(data.Meta.InstanceID) == 0 {
		return fmt.Errorf("instance-id key must be defined")
	}
	if len(strings.TrimSpace(data.Meta.DSMode)) == 0 {
		data.Meta.DSMode = "local"
	}
	if len(strings.TrimSpace(data.Meta.Platform)) == 0 {
		data.Meta.Platform = "nocloud"
	}

	data.Hostname = strings.TrimSpace(data.Hostname)
	data.Domain = strings.TrimSpace(data.Domain)
	data.Timezone = strings.TrimSpace(data.Timezone)

	if len(data.Domain) == 0 {
		data.Domain = "localdomain"
	}

	if data.User == nil {
		data.User = make(map[string]interface{})
	}

	if _, ok := data.User["hostname"]; !ok && len(data.Hostname) > 0 {
		data.User["hostname"] = data.Hostname
		data.User["create_hostname_file"] = true
		data.User["fqdn"] = fmt.Sprintf("%s.%s", data.Hostname, data.Domain)

		data.User["manage_etc_hosts"] = "localhost"
	}

	if _, ok := data.User["timezone"]; !ok && len(data.Timezone) > 0 {
		data.User["timezone"] = data.Timezone
	}

	tmpdir, err := os.MkdirTemp(filepath.Dir(outfile), ".cidata-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	imageFile := filepath.Join(tmpdir, "image")
	userFile := filepath.Join(tmpdir, "user-data")
	metaFile := filepath.Join(tmpdir, "meta-data")
	netFile := filepath.Join(tmpdir, "network-config")

	// user-data
	var userDataContent []byte

	if len(data.User) > 0 {
		if b, err := yaml.Marshal(data.User); err == nil {
			userDataContent = append([]byte("#cloud-config\n"), b...)
		} else {
			return err
		}
	} else {
		userDataContent = []byte("#cloud-config\n")
	}

	if err := os.WriteFile(userFile, userDataContent, 0644); err != nil {
		return err
	}

	// meta-data
	if b, err := yaml.Marshal(data.Meta); err == nil {
		if err := os.WriteFile(metaFile, b, 0644); err != nil {
			return err
		}
	} else {
		return err
	}

	// network-data
	if b, err := yaml.Marshal(data.Network); err == nil {
		if err := os.WriteFile(netFile, b, 0644); err != nil {
			return err
		}
	} else {
		return err
	}

	// generate
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
