package tasks

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/controlplaneio/sandbox-probe/pkg/models"
)

func getProxyMacOSCmdLine() (*models.ProxyConfig, error) {
	cmd := exec.Command("scutil", "--proxy")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	cfg := &models.ProxyConfig{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " : ", 2)
		if len(parts) != 2 {
			continue
		}

		key, val := parts[0], parts[1]

		switch key {
		case "HTTPEnable":
			if val != "1" {
				cfg.HTTPProxy = ""
			}
		case "HTTPProxy":
			if val != "" {
				cfg.HTTPProxy = val
			}
		case "HTTPPort":
			if cfg.HTTPProxy != "" {
				cfg.HTTPProxy = fmt.Sprintf("%s:%s", cfg.HTTPProxy, val)
			}
		case "HTTPSProxy":
			if val != "" {
				cfg.HTTPSProxy = val
			}
		case "HTTPSPort":
			if cfg.HTTPSProxy != "" {
				cfg.HTTPSProxy = fmt.Sprintf("%s:%s", cfg.HTTPSProxy, val)
			}
		case "SOCKSEnable":
			if val != "1" {
				cfg.SOCKSProxy = ""
			}
		case "SOCKSProxy":
			if val != "" {
				cfg.SOCKSProxy = val
			}
		case "SOCKSPort":
			if cfg.SOCKSProxy != "" {
				cfg.SOCKSProxy = fmt.Sprintf("%s:%s", cfg.SOCKSProxy, val)
			}
		case "ProxyAutoConfigURLString":
			if val != "" {
				cfg.PACURL = val
			}
		}
	}

	return cfg, nil
}
