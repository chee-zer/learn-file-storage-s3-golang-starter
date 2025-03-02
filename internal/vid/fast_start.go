package vid

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"strings"
)

func ProcessVideoForFastStart(filepath string) (string, error) {
	filename := strings.TrimPrefix(filepath, "/tmp/")
	log.Println("entered fast start, filepath: ", filepath)
	tempFile, err := os.CreateTemp("", "PROCESSING*"+filename)
	if err != nil {
		return "", err
	}
	log.Println("New file path : ", tempFile.Name())

	out := bytes.Buffer{}

	cmd := exec.Command("ffmpeg", "-y", "-i", filepath, "-c", "copy", "-movflags", "+faststart", "-f", "mp4", tempFile.Name())
	cmd.Stdout = &out
	cmd.Stderr = &out

	err = cmd.Run()
	if err != nil {
		log.Println("error executing command: ", out.String())
		return "", err
	}
	log.Println("output: ", out.String())
	return tempFile.Name(), nil
}
