package adb

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

// StartServer explicitly starts the adb daemon so it doesn't interfere with later commands.
func (c *Client) StartServer() {
	log.Println("Starting ADB server...")
	cmd := exec.Command("adb", "start-server")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: adb start-server: %s %v", string(output), err)
	} else {
		log.Println("ADB server started successfully")
	}
}

// WaitForDevice blocks until a device is available, with a timeout.
func (c *Client) WaitForDevice(timeout time.Duration) bool {
	cmd := exec.Command("adb", "wait-for-device")
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		return err == nil
	case <-time.After(timeout):
		cmd.Process.Kill()
		return false
	}
}

// TrackDevices listens for connect/disconnect events and sends true/false to stateCh
func (c *Client) TrackDevices(stateCh chan<- bool) {
	for {
		cmd := exec.Command("adb", "track-devices")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		if err := cmd.Start(); err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "device") && !strings.Contains(line, "offline") {
				stateCh <- true
			} else {
				stateCh <- false
			}
		}

		cmd.Wait()
		time.Sleep(2 * time.Second) // Reconnect if adb server crashes
	}
}

type SMS struct {
	ID      string
	Address string
	Body    string
	Date    string
	SubID   string
}

func parseContentRow(line string, lastKey string) map[string]string {
	result := make(map[string]string)

	// Strip "Row: X "
	firstEq := strings.Index(line, "=")
	if firstEq != -1 {
		spaceIdx := strings.LastIndex(line[:firstEq], " ")
		if spaceIdx != -1 {
			line = line[spaceIdx+1:]
		}
	}

	var prefix string
	if lastKey != "" {
		lastIdx := strings.Index(line, ", "+lastKey+"=")
		if lastIdx != -1 {
			prefix = line[:lastIdx]
			result[lastKey] = line[lastIdx+len(", "+lastKey+"="):]
		} else {
			prefix = line
		}
	} else {
		prefix = line
	}

	parts := strings.Split(prefix, ", ")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

// runAdbShell runs an adb shell command, retrying once after wait-for-device if the first attempt fails.
func (c *Client) runAdbShell(shellCmd string) (string, error) {
	cmd := exec.Command("adb", "shell", shellCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(output))
		// If device not ready, wait and retry once
		if strings.Contains(outStr, "no devices") || strings.Contains(outStr, "device not found") {
			log.Println("Device not ready, waiting...")
			if c.WaitForDevice(10 * time.Second) {
				cmd2 := exec.Command("adb", "shell", shellCmd)
				output2, err2 := cmd2.CombinedOutput()
				if err2 != nil {
					return "", fmt.Errorf("adb retry error: %s", strings.TrimSpace(string(output2)))
				}
				return string(output2), nil
			}
			return "", fmt.Errorf("device not available after wait")
		}
		return "", fmt.Errorf("adb error: %s", outStr)
	}
	return string(output), nil
}

func (c *Client) QuerySMS() ([]SMS, error) {
	// Use only columns that are guaranteed to exist on all Android versions.
	// subscription_id does NOT exist on many devices.
	output, err := c.runAdbShell("content query --uri content://sms/inbox --projection _id:address:body:date --sort \"date DESC\"")
	if err != nil {
		return nil, err
	}

	var messages []SMS
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Row: ") {
			continue
		}

		parsed := parseContentRow(line, "body")

		sms := SMS{
			ID:      parsed["_id"],
			Address: parsed["address"],
			Date:    parsed["date"],
			Body:    parsed["body"],
		}

		if sms.ID != "" {
			messages = append(messages, sms)
		}
	}

	return messages, nil
}

type Call struct {
	ID       string
	Number   string
	Type     string // 1=Incoming, 3=Missed
	Date     string
	Duration string
	SubID    string
}

func (c *Client) QueryCalls() ([]Call, error) {
	// Call types: 1 = Incoming, 2 = Outgoing, 3 = Missed, 4 = Voicemail, 5 = Rejected, 6 = Blocked
	// Don't include subscription_id — may not exist on all devices.
	output, err := c.runAdbShell("content query --uri content://call_log/calls --projection _id:number:type:date:duration --sort \"date DESC\"")
	if err != nil {
		return nil, err
	}

	var calls []Call
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Row: ") {
			continue
		}

		parsed := parseContentRow(line, "")

		call := Call{
			ID:       parsed["_id"],
			Number:   parsed["number"],
			Type:     parsed["type"],
			Date:     parsed["date"],
			Duration: parsed["duration"],
		}

		if call.ID != "" {
			calls = append(calls, call)
		}
	}

	return calls, nil
}

