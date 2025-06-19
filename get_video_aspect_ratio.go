package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
)

type ffprobeOutput struct {
	Streams []struct {
		Width int `json:"width"`
		Height int `json:"height"`
	} `json:"streams"`
}


func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	// capture output to buffer
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf

	err := cmd.Run()

	if err != nil {
		return "", err
	}	

	// parse json output
	var output ffprobeOutput
	err = json.Unmarshal(outBuf.Bytes(), &output)
	if err != nil {
		return "", err
	}

	return getAspectRatio(output.Streams[0].Width, output.Streams[0].Height), nil	
}

// helper functions to calculate aspect ratio
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func getAspectRatio(width, height int) string {
	divisor := gcd(width, height)
	aspectRatio := [2]int{width / divisor, height / divisor}

	if aspectRatio[0] == 16*aspectRatio[1]/9 {
		return "16:9"
	} else if aspectRatio[1] == 16*aspectRatio[0]/9 {
		return "9:16"
	}
	return "other"
}

func determineDirectory(filePath string) (string, error) {
	ratio,err := getVideoAspectRatio(filePath)
	if err != nil {
		return "", err
	}

	switch ratio {
	case "16:9":
		return "landscape", nil 
	case "9:16":
		return "portrait", nil
	default:
		return "other", nil
	}
}