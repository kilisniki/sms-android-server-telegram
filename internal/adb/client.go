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

		// Row: 0 _id=1, address=+123, date=123, subscription_id=1, body=Hello
		// Fields are in the order we requested. body is last to make parsing easier!
		
		sms := SMS{}
		
		idStart := strings.Index(line, "_id=")
		if idStart == -1 { continue }
		
		addrStart := strings.Index(line, ", address=")
		dateStart := strings.Index(line, ", date=")
		subIdStart := strings.Index(line, ", subscription_id=")
		bodyStart := strings.Index(line, ", body=")
		
		if addrStart != -1 {
			sms.ID = line[idStart+4 : addrStart]
		}
		
		if dateStart != -1 && addrStart != -1 {
			sms.Address = line[addrStart+10 : dateStart]
		}
		
		// some devices might not have subscription_id
		if bodyStart != -1 {
			if subIdStart != -1 {
				sms.Date = line[dateStart+7 : subIdStart]
				sms.SubID = line[subIdStart+18 : bodyStart]
			} else {
				sms.Date = line[dateStart+7 : bodyStart]
			}
			sms.Body = line[bodyStart+7:] // body is until the end of line
		}
		
		messages = append(messages, sms)
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

		// Row: 0 _id=1, number=+123, type=3, date=123, duration=0, subscription_id=1
		call := Call{}
		
		idStart := strings.Index(line, "_id=")
		if idStart == -1 { continue }
		
		numberStart := strings.Index(line, ", number=")
		typeStart := strings.Index(line, ", type=")
		dateStart := strings.Index(line, ", date=")
		durationStart := strings.Index(line, ", duration=")
		subIdStart := strings.Index(line, ", subscription_id=")
		
		if numberStart != -1 {
			call.ID = line[idStart+4 : numberStart]
		}
		if typeStart != -1 && numberStart != -1 {
			call.Number = line[numberStart+9 : typeStart]
		}
		if dateStart != -1 && typeStart != -1 {
			call.Type = line[typeStart+7 : dateStart]
		}
		if durationStart != -1 && dateStart != -1 {
			call.Date = line[dateStart+7 : durationStart]
		}
		if subIdStart != -1 && durationStart != -1 {
			call.Duration = line[durationStart+10 : subIdStart]
			call.SubID = line[subIdStart+18:] // until the end
		} else if durationStart != -1 {
			call.Duration = line[durationStart+10:]
		}
		
		calls = append(calls, call)
	}

	return calls, nil
}
