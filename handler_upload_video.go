package main

import (
	"crypto/rand"
	"encoding/base64"
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

	tempFile, err := os.CreateTemp("", "tubely-upload_*.mp4")
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

	processedFilepath, err := vid.ProcessVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't process file for fast start", err)
		return
	}

	processedFile, err := os.Open(processedFilepath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't open fast start file", err)
		return
	}

	defer os.Remove(processedFilepath)
	defer processedFile.Close()

	var prefix string
	if aspectRatio == "16:9" {
		prefix = "landscape/"
	} else if aspectRatio == "9:16" {
		prefix = "portrait/"
	} else {
		prefix = "other/"
	}

	// convert these to *string cuz aws works with those:
	// because strings cant have nil values, but pointers can. Its to differentiate between "" and nil
	bucket := aws.String(cfg.s3Bucket)
	key := aws.String(prefix + base64.RawURLEncoding.EncodeToString(by) + ".mp4")
	objInput := s3.PutObjectInput{
		Bucket:      bucket,
		Key:         key,
		Body:        processedFile,
		ContentType: aws.String(mediaType),
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &objInput)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}
	videoUrl := cfg.s3CfDistribution +"/"+ *key
	log.Println("video url: ", videoUrl)
	videoMetadata.VideoURL = &videoUrl

	if err := cfg.db.UpdateVideo(videoMetadata); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't Update Video", err)
		return
	}
	log.Println("done: ", *videoMetadata.VideoURL)
}

// func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
// 	pClient := s3.NewPresignClient(s3Client)
// 	signedReq, err := pClient.PresignGetObject(context.Background(), &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)}, s3.WithPresignExpires(expireTime))
// 	if err != nil {
// 		return "", err
// 	}

// 	return signedReq.URL, nil
// }

// func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
// 	videoUrl := video.VideoURL
// 	if videoUrl == nil || *videoUrl == "" {
//         return video, nil
//     }
// 	log.Println("HELLO: ", *videoUrl)
// 	videoUrlParts := strings.Split(*videoUrl, ",")
// 	if len(videoUrlParts) < 2 {
// 		return database.Video{}, fmt.Errorf("invalid url")
// 	}
// 	bucket := videoUrlParts[0]
// 	key := videoUrlParts[1]
// 	presignedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, 15*time.Minute)
// 	if err != nil {
// 		return database.Video{}, err
// 	}
// 	video.VideoURL = &presignedURL
// 	return video, nil
// }
