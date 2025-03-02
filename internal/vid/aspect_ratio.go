package vid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
)

func gcd(a, b int) int {
	if a == b {
		return a
	} else if a > b {
		return gcd(a-b, b)
	} else {
		return gcd(a, b-a)
	}
}

func GetVideoAspectRatio(filepath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	out := bytes.Buffer{}
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		log.Println("Couldnt execute ffprobe: ", out.String())
		return "", err
	}
	log.Println("Command is run")

	type ffprobeOut struct {
		Streams []struct {
			Width              int    `json:"width"`
			Height             int    `json:"height"`
			DisplayAspectRatio string `json:"display_aspect_ratio"`
		} `json:"streams"`
	}
	fout := ffprobeOut{}
	decoder := json.NewDecoder(&out)
	if err := decoder.Decode(&fout); err != nil {
		return "", err
	}
	log.Println(fout)

	if fout.Streams[0].DisplayAspectRatio == "" {
		width := fout.Streams[0].Width
		height := fout.Streams[0].Height
		gcd := gcd(width, height)
		log.Println("gcd: ", gcd)
		log.Println("height: ", height/gcd)
		log.Println("width: ", width/gcd)

		aspectRatio := fmt.Sprintf("%v:%v", width/gcd, height/gcd)
		log.Println("aspect ratio: ", aspectRatio)
		return aspectRatio, nil
	}
	return fout.Streams[0].DisplayAspectRatio, nil

}
