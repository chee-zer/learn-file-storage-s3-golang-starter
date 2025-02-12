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
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	videoMetaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't find Video", err)
		return
	}
	if videoMetaData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	maxMemory := 10 << 20
	r.ParseMultipartForm(int64(maxMemory))

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "No file in request", err)
		return
	}

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse media type", err)
		return
	}
	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusUnsupportedMediaType, "Please upload a .png or .jpeg", err)
		return
	}

	fileExtension, err := mime.ExtensionsByType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get file extension", err)
		return
	}
	if len(fileExtension) == 0 {
		respondWithError(w, http.StatusBadRequest, "No known file extension for the provided Content-Type", err)
		return
	}
	log.Println(fileExtension)

	randBytes := make([]byte, 32)
	_, err = rand.Read(randBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't generate random string", err)
	}

	encoded := base64.RawURLEncoding.EncodeToString(randBytes)

	filePath := filepath.Join(cfg.assetsRoot, encoded+fileExtension[0])
	log.Println(filePath)

	assetFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create file", err)
		return
	}
	defer assetFile.Close()
	_, err = io.Copy(assetFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't copy file to "+filePath, err)
		return
	}

	dataUrl := fmt.Sprintf("http://localhost:%v/%v", cfg.port, filePath)
	videoMetaData.ThumbnailURL = &dataUrl

	if err := cfg.db.UpdateVideo(videoMetaData); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't Update Thumbnail", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoMetaData)
}
