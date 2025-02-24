package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/vid"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't get bearer token", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid JWT", err)
		return
	}

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadGateway, "Invalid Video ID", err)
		return
	}

	videoMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't find video", err)
		return
	}

	if userID != videoMetadata.UserID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized: ", err)
		return
	}

	uploadLimit := 1 << 30                                      //1 GB
	r.Body = http.MaxBytesReader(w, r.Body, int64(uploadLimit)) //limit request body size

	maxMemory := 50 << 20
	if err := r.ParseMultipartForm(int64(maxMemory)); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse request", err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "No file in request", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get file type", err)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusUnsupportedMediaType, "File uploaded is not a mp4 file", err)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload*.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create temporary file", err)
		return
	}
	
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	io.Copy(tempFile, file)        //copied stream to tempFile in file system
	tempFile.Seek(0, io.SeekStart) //used io.SeekStart instead of 0 to improve code readability
	
	by := make([]byte, 32)
	rand.Read(by)
	
	aspectRatio, err := vid.GetVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get aspect ratio", err)
		return
	}
	var prefix string
	if aspectRatio == "16:9" {
		prefix = "landscape"
	} else if aspectRatio == "9:16" {
		prefix = "portrait"
	} else {
		prefix = "other"
	}
	bucket := aws.String(cfg.s3Bucket)
	key := aws.String(prefix + base64.RawURLEncoding.EncodeToString(by) + ".mp4")
	objInput := s3.PutObjectInput{
		Bucket:      bucket,
		Key:         key,
		Body:        tempFile,
		ContentType: aws.String(mediaType),
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &objInput)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}
	videoUrl := fmt.Sprintf("https://%v.s3.%v.amazonaws.com/%v", cfg.s3Bucket, cfg.s3Region, *key)
	videoMetadata.VideoURL = &videoUrl

	if err := cfg.db.UpdateVideo(videoMetadata); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't Update Thumbnail", err)
		return
	}
	log.Println("done: ", *videoMetadata.VideoURL)
}
