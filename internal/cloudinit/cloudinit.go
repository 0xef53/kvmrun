package cloudinit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Data struct {
	Meta    MetadataConfig
	Network NetworkConfig

	// These fields override the same name keys from User structure
	Hostname string
	Domain   string
	Timezone string

	Vendor map[string]interface{}
	User   map[string]interface{}
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
		MacAddress string `json:"macaddress" yaml:"macaddress"`
	} `json:"match" yaml:"match"`
	Addresses []string `json:"addresses" yaml:"addresses"`
	Gateway4  string   `json:"gateway4,omitempty" yaml:"gateway4,omitempty"`
	Gateway6  string   `json:"gateway6,omitempty" yaml:"gateway6,omitempty"`
}

// Function is used to provide backward compatibility
// with Phoenix Guest Agent, which looks at the "mac_address" field.
func (c EthernetConfig) MarshalYAML() (interface{}, error) {
	ovrd := struct {
		Match struct {
			MacAddress  string `yaml:"macaddress"`
			Mac_Address string `yaml:"mac_address"`
		} `yaml:"match"`
		Addresses []string `yaml:"addresses"`
		Gateway4  string   `yaml:"gateway4,omitempty"`
		Gateway6  string   `yaml:"gateway6,omitempty"`
	}{}

	ovrd.Match.MacAddress = c.Match.MacAddress
	ovrd.Match.Mac_Address = c.Match.MacAddress
	ovrd.Addresses = c.Addresses
	ovrd.Gateway4 = c.Gateway4
	ovrd.Gateway6 = c.Gateway6

	return &ovrd, nil
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

	if data.Vendor == nil {
		data.Vendor = make(map[string]interface{})
	}

	if data.User == nil {
		data.User = make(map[string]interface{})
	}

	if _, ok := data.Vendor["hostname"]; !ok && len(data.Hostname) > 0 {
		data.Vendor["hostname"] = data.Hostname
		data.Vendor["create_hostname_file"] = true
		data.Vendor["fqdn"] = fmt.Sprintf("%s.%s", data.Hostname, data.Domain)

		data.Vendor["manage_etc_hosts"] = "localhost"
	}

	if _, ok := data.Vendor["timezone"]; !ok && len(data.Timezone) > 0 {
		data.Vendor["timezone"] = data.Timezone
	}

	tmpdir, err := os.MkdirTemp(filepath.Dir(outfile), ".cidata-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	imageFile := filepath.Join(tmpdir, "image")
	userFile := filepath.Join(tmpdir, "user-data")
	metaFile := filepath.Join(tmpdir, "meta-data")
	vendorFile := filepath.Join(tmpdir, "vendor-data")
	netFile := filepath.Join(tmpdir, "network-config")

	// vendor-data
	var vendorDataContent []byte

	if len(data.Vendor) > 0 {
		if b, err := yaml.Marshal(data.Vendor); err == nil {
			vendorDataContent = append([]byte("#cloud-config\n"), b...)
		} else {
			return err
		}
	} else {
		vendorDataContent = []byte("#cloud-config\n")
	}

	if err := os.WriteFile(vendorFile, vendorDataContent, 0644); err != nil {
		return err
	}

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
		vendorFile,
		netFile,
	}

	out, err := exec.Command(genisoimageBinary, opts...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("genisoimage failed (%s): %s", err, strings.TrimSpace(string(out)))
	}

	return os.Rename(imageFile, outfile)
}
