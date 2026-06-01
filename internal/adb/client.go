package adb

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
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

func (c *Client) QuerySMS() ([]SMS, error) {
	cmd := exec.Command("adb", "shell", "content query --uri content://sms/inbox --projection _id:address:date:subscription_id:body")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("adb error: %s", string(output))
	}

	var messages []SMS
	lines := strings.Split(string(output), "\n")
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
			SubID:   parsed["subscription_id"],
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
	cmd := exec.Command("adb", "shell", "content query --uri content://call_log/calls --projection _id:number:type:date:duration:subscription_id")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("adb error: %s", string(output))
	}

	var calls []Call
	lines := strings.Split(string(output), "\n")
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
			SubID:    parsed["subscription_id"],
		}
		
		if call.ID != "" {
			calls = append(calls, call)
		}
	}

	return calls, nil
}
