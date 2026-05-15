package handler

import (
	"bufio"
	"os"
	"strings"
)

func UpdateEnvFile(licenseKey, domain, serverURL string) error {
	envPath := "/opt/flvx-svc/.env"
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return nil
	}

	f, err := os.Open(envPath)
	if err != nil {
		return err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	updated := false
	for i, line := range lines {
		kv := strings.SplitN(line, "=", 2)
		if len(kv) == 2 {
			k := strings.TrimSpace(kv[0])
			switch k {
			case "LICENSE_KEY":
				lines[i] = "LICENSE_KEY=" + licenseKey
				updated = true
			case "SERVER_DOMAIN":
				lines[i] = "SERVER_DOMAIN=" + domain
				updated = true
			case "LICENSE_SERVER_URL":
				lines[i] = "LICENSE_SERVER_URL=" + serverURL
				updated = true
			}
		}
	}

	if !updated {
		if licenseKey != "" {
			lines = append(lines, "LICENSE_KEY="+licenseKey)
		}
		if domain != "" {
			lines = append(lines, "SERVER_DOMAIN="+domain)
		}
		if serverURL != "" {
			lines = append(lines, "LICENSE_SERVER_URL="+serverURL)
		}
	} else {
		f.Close()
	}

	content := strings.Join(lines, "\n") + "\n"
	return osWrite(envPath, []byte(content), 0644)
}

func osWrite(name string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}
	return err
}
