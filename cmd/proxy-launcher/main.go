package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/0xef53/kvmrun/kvmrun"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/urfave/cli/v2"
)

var (
	Error = log.New(os.Stdout, "Error: ", 0)
)

func main() {
	app := cli.NewApp()

	app.Name = "proxy-launcher"
	app.Usage = "TCP proxy launcher"

	app.Action = run

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "instance-name",
			Usage:       "systemd instance name",
			DefaultText: "not set",
			EnvVars:     []string{"INSTANCE_NAME"},
			Required:    true,
		},
		&cli.StringFlag{
			Name:    "delimiter",
			Usage:   "systemd instance name delimiter",
			Value:   "@",
			EnvVars: []string{"INSTANCE_NAME_DELIMETER"},
		},
	}

	if err := app.Run(os.Args); err != nil {
		Error.Fatalln(err)
	}
}

type ProxyConfiguration struct {
	Path         string            `json:"path"`
	Command      string            `json:"command"`
	Environments map[string]string `json:"envs"`
}

func run(c *cli.Context) error {
	parts := strings.Split(c.String("instance-name"), c.String("delimiter"))
	if len(parts) != 2 {
		return fmt.Errorf("incorrect instance name: %s", c.String("instance-name"))
	}

	vmname := parts[0]
	diskname := parts[1]

	b, err := func() ([]byte, error) {
		var b []byte
		var err error

		// Running config
		b, err = os.ReadFile(filepath.Join(kvmrun.CHROOTDIR, vmname, "run/backend_proxy"))
		if err == nil {
			fmt.Printf("Config file: %s\n", filepath.Join(kvmrun.CHROOTDIR, vmname, "run/backend_proxy"))

			return b, nil
		}

		if os.IsNotExist(err) {
			var tmp map[string]*json.RawMessage

			// Standard virt.machine config
			b, err = os.ReadFile(filepath.Join(kvmrun.CONFDIR, vmname, "config"))
			if err != nil {
				return nil, err
			}

			fmt.Printf("Config file: %s\n", filepath.Join(kvmrun.CONFDIR, vmname, "config"))

			if err := json.Unmarshal(b, &tmp); err != nil {
				return nil, err
			}

			if v, ok := tmp["proxy"]; ok {
				return *v, nil
			}

			return []byte{}, nil
		}

		return nil, err
	}()

	if err != nil {
		return err
	}

	servers := make([]kvmrun.Proxy, 0)

	if err := json.Unmarshal(b, &servers); err != nil {
		return err
	}

	if len(servers) == 0 {
		return fmt.Errorf("no one proxy found for %s", vmname)
	}

	sigc := make(chan os.Signal, 1)
	defer close(sigc)

	signal.Notify(sigc, syscall.SIGUSR1)
	defer signal.Stop(sigc)

	go func() {
		<-sigc

		if ok, err := daemon.SdNotify(false, daemon.SdNotifyReady); !ok {
			Error.Printf("unable to send systemd notify: %s\n", err)
		}
	}()

	for _, proxy := range servers {
		if u, err := url.Parse(proxy.Path); err == nil {
			if filepath.Base(u.Path) != diskname {
				continue
			}
		} else {
			Error.Printf("failed to parse proxy endpoint %s: %s\n", proxy.Path, err)
			continue
		}

		if len(proxy.Command) == 0 {
			return fmt.Errorf("empty proxy command")
		}

		proxyCmd := exec.Command(proxy.Command)

		proxyCmd.Stdin = os.Stdin
		proxyCmd.Stderr = os.Stderr
		proxyCmd.Stdout = os.Stdout

		for k, v := range proxy.Envs {
			env := strings.ToUpper(k) + "=" + v

			fmt.Printf("Environment: %s\n", env)

			proxyCmd.Env = append(proxyCmd.Env, env)
		}

		fmt.Printf("Proxy command: %v\n", proxyCmd)

		return proxyCmd.Run()
	}

	return fmt.Errorf("configuration not found")
}
