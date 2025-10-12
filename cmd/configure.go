package cmd

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/goccy/go-json"
	"github.com/pelican-dev/wings/config"
	"github.com/spf13/cobra"
)

var configureArgs struct {
	PanelURL      string
	Token         string
	ConfigPath    string
	Node          string
	Override      bool
	AllowInsecure bool
}

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Use a token to configure wings automatically",
	Run:   configureCmdRun,
}

func init() {
	configureCmd.PersistentFlags().StringVarP(&configureArgs.PanelURL, "panel-url", "p", "", "The base URL for this daemon's panel")
	configureCmd.PersistentFlags().StringVarP(&configureArgs.Token, "token", "t", "", "The API key to use for fetching node information")
	configureCmd.PersistentFlags().StringVarP(&configureArgs.Node, "node", "n", "", "The ID of the node which will be connected to this daemon")
	configureCmd.PersistentFlags().StringVarP(&configureArgs.ConfigPath, "config-path", "c", config.DefaultLocation, "The path where the configuration file should be made")
	configureCmd.PersistentFlags().BoolVar(&configureArgs.Override, "override", false, "Set to true to override an existing configuration for this node")
	configureCmd.PersistentFlags().BoolVar(&configureArgs.AllowInsecure, "allow-insecure", false, "Set to true to disable certificate checking")
}

func configureCmdRun(cmd *cobra.Command, args []string) {
	if configureArgs.AllowInsecure {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	if _, err := os.Stat(configureArgs.ConfigPath); err == nil && !configureArgs.Override {
		err := huh.NewConfirm().
			Title("Override existing configuration file?").
			Value(&configureArgs.Override).
			Run()
		if err != nil {
			if err == huh.ErrUserAborted {
				return
			}
			panic(err)
		}
		if !configureArgs.Override {
			fmt.Println("Aborting process; a configuration file already exists for this node.")
			os.Exit(1)
		}
	} else if err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	var fields []huh.Field

	if err := validateField("url", configureArgs.PanelURL); err != nil {
		fields = append(fields, huh.NewInput().
			Title("Panel URL: ").
			Validate(func(str string) error {
				return validateField("url", str)
			}).
			Value(&configureArgs.PanelURL),
		)
	}

	if err := validateField("token", configureArgs.Token); err != nil {
		fields = append(fields, huh.NewInput().
			Title("API Token: ").
			Validate(func(str string) error {
				return validateField("token", str)
			}).
			Value(&configureArgs.Token),
		)
	}

	if err := validateField("node", configureArgs.Node); err != nil {
		fields = append(fields, huh.NewInput().
			Title("Node ID: ").
			Validate(func(str string) error {
				return validateField("node", str)
			}).
			Value(&configureArgs.Node),
		)
	}
	if len(fields) > 0 {
		if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
			if err == huh.ErrUserAborted {
				return
			}
			panic(err)
		}
	}

	c := &http.Client{
		Timeout: time.Second * 30,
	}

	req, err := getRequest()
	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v %s\n", req.Header, req.URL.String())

	res, err := c.Do(req)
	if err != nil {
		fmt.Println("Failed to fetch configuration from the panel.\n", err.Error())
		os.Exit(1)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusUnauthorized {
		fmt.Println("The authentication credentials provided were not valid.")
		os.Exit(1)
	} else if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)

		fmt.Println("An error occurred while processing this request.\n", string(b))
		os.Exit(1)
	}

	b, err := io.ReadAll(res.Body)

	cfg, err := config.NewAtPath(configPath)
	if err != nil {
		panic(err)
	}

	if err := json.Unmarshal(b, cfg); err != nil {
		panic(err)
	}

	// Manually specify the Panel URL as it won't be decoded from JSON.
	cfg.PanelLocation = configureArgs.PanelURL

	if err = config.WriteToDisk(cfg); err != nil {
		panic(err)
	}

	fmt.Println("Successfully configured wings.")
}

func getRequest() (*http.Request, error) {
	u, err := url.Parse(configureArgs.PanelURL)
	if err != nil {
		panic(err)
	}

	u.Path = path.Join(u.Path, fmt.Sprintf("api/application/nodes/%s/configuration", configureArgs.Node))

	r, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", configureArgs.Token))

	return r, nil
}

func validateField(name string, str string) error {
	switch name {
	case "url":
		u, err := url.Parse(str)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" {
			return fmt.Errorf("please provide a valid panel URL")
		}
	case "token":
		if !regexp.MustCompile(`^(peli|papp)_(\w{43})$`).Match([]byte(str)) {
			return fmt.Errorf("please provide a valid authentication token")
		}
	case "node":
		if !regexp.MustCompile(`^(\d+)$`).Match([]byte(str)) {
			return fmt.Errorf("please provide a valid numeric node ID")
		}
	}
	return nil
}
