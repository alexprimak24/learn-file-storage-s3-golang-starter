package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 1 << 30 // limit to 1GB
	
	// get video ID and parse it
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	// authenticate the user
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


	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)


	// get metadata of video from the db 
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't find the video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized to update this video", err)
		return
	}

	file, header, err := r.FormFile("video")

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	if mediaType != "video/mp4"{
		respondWithError(w, http.StatusBadRequest, "Invalid file type", nil)
		return
	}

	// create temporary file
	tmpFile, err := os.CreateTemp("", "tubely-upload-*.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating temp file", err)
	}
	defer tmpFile.Close()
	defer os.Remove(tmpFile.Name())

    // copy the file contents
	_, err = io.Copy(tmpFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error copying file contents", err)
		return
	}

	// reset pointer to the beginning
	_, err = tmpFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to rewind temp file", err)
		return
	}

	// put obj to S3
	key := make([]byte, 32)
	rand.Read(key)

	base64_key := base64.RawURLEncoding.EncodeToString(key)
	
	file_extention, err := mime.ExtensionsByType(mediaType)
	if err != nil || len(file_extention) == 0 {
		respondWithError(w, http.StatusInternalServerError, "Error while reading extention from header",err)
		return
	}

	finalFilename :=  base64_key + file_extention[0]

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket: aws.String(cfg.s3Bucket),
		Key: aws.String(finalFilename),
		Body: tmpFile,
		ContentType: aws.String(mediaType),
	})

	// construct public URL
	urlPath := fmt.Sprintf("http://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, finalFilename)
	// update thumbnail in database
	video.VideoURL = &urlPath
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update the video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
