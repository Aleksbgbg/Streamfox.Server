package controllers

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"streamfox-backend/codec"
	"streamfox-backend/models"
	"streamfox-backend/utils"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
)

type VideoCreatedInfo struct {
	Id          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Visibility  models.Visibility `json:"visibility"`
}

func CreateVideo(c *gin.Context) {
	user := getUserParam(c)

	video, err := models.NewVideo(user)

	if ok := checkServerError(c, err, errGenericDatabaseIo); !ok {
		return
	}

	c.JSON(http.StatusCreated, VideoCreatedInfo{
		Id:          video.IdSnowflake().Base58(),
		Name:        video.Name,
		Description: video.Description,
		Visibility:  video.Visibility,
	})
}

const VIDEO_PARAM_KEY = "video"

func ExtractVideoMiddleware(c *gin.Context) {
	videoId, err := snowflake.ParseBase58([]byte(c.Param("id")))

	if ok := checkUserError(c, err, errVideoInvalidId); !ok {
		return
	}

	video, err := models.FetchVideo(videoId)

	if ok := checkUserError(c, err, errVideoIdNonExistent); !ok {
		return
	}

	c.Set(VIDEO_PARAM_KEY, video)
}

func getVideoParam(c *gin.Context) *models.Video {
	return c.MustGet(VIDEO_PARAM_KEY).(*models.Video)
}

func EnsureCompleteVideoMiddleware(c *gin.Context) {
	video := getVideoParam(c)

	if video.Status < models.COMPLETE {
		userError(c, errVideoUploadIncomplete)
	}
}

func EnsureVisibleVideoMiddleware(c *gin.Context) {
	video := getVideoParam(c)

	if video.Visibility == models.PRIVATE {
		if !hasUserParam(c) {
			userError(c, errUserRequired)
			return
		}

		if !video.IsCreator(getUserParam(c)) {
			userError(c, errGenericAccessForbidden)
			return
		}
	}
}

func EnsureIsOwnerMiddleware(c *gin.Context) {
	user := getUserParam(c)
	video := getVideoParam(c)

	if !video.IsCreator(user) {
		userError(c, errVideoNotOwned)
	}
}

type VideoUpdateInfo struct {
	Name        string             `json:"name"        binding:"required,min=2,max=256"`
	Description *string            `json:"description" binding:"required"`
	Visibility  *models.Visibility `json:"visibility"  binding:"required,min=0,max=2"`
}

func UpdateVideo(c *gin.Context) {
	var update VideoUpdateInfo

	if ok := checkValidationError(c, c.ShouldBindJSON(&update)); !ok {
		return
	}

	video := getVideoParam(c)
	video.Name = update.Name
	video.Description = *update.Description
	video.Visibility = *update.Visibility
	err := video.Save()

	if ok := checkServerError(c, err, errGenericDatabaseIo); !ok {
		return
	}

	c.Status(http.StatusNoContent)
}

func UploadVideo(c *gin.Context) {
	video := getVideoParam(c)

	if video.Status > models.UPLOADING {
		userError(c, errVideoCannotOverwrite)
		return
	}

	video.Status = models.UPLOADING
	defer video.Save()

	dataRoot := utils.GetEnvVar(utils.DATA_ROOT)

	videoDir := fmt.Sprintf("%s/videos/%s", dataRoot, video.IdSnowflake().Base58())
	err := os.MkdirAll(videoDir, os.ModePerm)

	if ok := checkServerError(c, err, errGenericFileIo); !ok {
		return
	}

	filepath := fmt.Sprintf("%s/videos/%s/video", dataRoot, video.IdSnowflake().Base58())
	file, err := os.Create(filepath)
	if ok := checkServerError(c, err, errGenericFileIo); !ok {
		return
	}

	_, err = io.Copy(file, c.Request.Body)

	if ok := checkServerError(c, err, errGenericFileIo); !ok {
		file.Close()
		os.Remove(filepath)
		return
	}

	err = file.Close()

	if ok := checkServerError(c, err, errGenericFileIo); !ok {
		os.Remove(filepath)
		return
	}

	probe, err := codec.Probe(filepath)

	if err != nil {
		os.Remove(filepath)

		if errors.Is(err, codec.ErrInvalidVideoType) {
			userError(c, errVideoInvalidFormat)
		} else {
			serverError(c, err, errVideoProbe)
		}
		return
	}

	info, err := os.Stat(filepath)

	if ok := checkServerError(c, err, errVideoGetSize); !ok {
		return
	}

	video.MimeType = probe.MimeType
	video.DurationSecs = probe.DurationSecs
	video.SizeBytes = info.Size()
	video.Status = models.PROCESSING

	err = codec.GenerateThumbnail(videoDir)

	if ok := checkServerError(c, err, errVideoGenerateThumbnail); !ok {
		return
	}

	video.Status = models.COMPLETE

	c.Status(http.StatusNoContent)
}

type VideoInfo struct {
	Id           string            `json:"id"`
	Creator      UserInfo          `json:"creator"`
	DurationSecs int32             `json:"duration_secs"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Visibility   models.Visibility `json:"visibility"`
	Views        int64             `json:"views"`
	Likes        int64             `json:"likes"`
	Dislikes     int64             `json:"dislikes"`
}

func getVideoInfo(video *models.Video) (*VideoInfo, error) {
	views, err := models.CountViews(video)

	if err != nil {
		return nil, err
	}

	return &VideoInfo{
		Id:           video.IdSnowflake().Base58(),
		Creator:      getUserInfo(&video.Creator),
		DurationSecs: video.DurationSecs,
		Name:         video.Name,
		Description:  video.Description,
		Visibility:   video.Visibility,
		Views:        views,
		Likes:        0,
		Dislikes:     0,
	}, nil
}

func GetVideos(c *gin.Context) {
	videos, err := models.FetchAllVideos()

	if ok := checkServerError(c, err, errGenericDatabaseIo); !ok {
		return
	}

	videoInfos := make([]*VideoInfo, 0)
	for _, video := range videos {
		videoInfo, err := getVideoInfo(&video)

		if ok := checkServerError(c, err, errGenericDatabaseIo); !ok {
			return
		}

		videoInfos = append(videoInfos, videoInfo)
	}

	c.JSON(http.StatusOK, videoInfos)
}

func GetVideoInfo(c *gin.Context) {
	video := getVideoParam(c)

	videoInfo, err := getVideoInfo(video)

	if ok := checkServerError(c, err, errGenericDatabaseIo); !ok {
		return
	}

	c.JSON(http.StatusOK, videoInfo)
}

func GetVideoThumbnail(c *gin.Context) {
	video := getVideoParam(c)
	dataRoot := utils.GetEnvVar(utils.DATA_ROOT)
	filepath := fmt.Sprintf("%s/videos/%s/thumbnail", dataRoot, video.IdSnowflake().Base58())

	c.File(filepath)
}

func loadImage(decoder func(io.Reader) (image.Image, error), path string) (image.Image, error) {
	reader, err := os.Open(path)

	if err != nil {
		return nil, err
	}

	defer reader.Close()

	return decoder(reader)
}

func GetVideoPreview(c *gin.Context) {
	video := getVideoParam(c)
	dataRoot := utils.GetEnvVar(utils.DATA_ROOT)

	thumbnail, err := loadImage(
		jpeg.Decode,
		fmt.Sprintf("%s/videos/%s/thumbnail", dataRoot, video.IdSnowflake().Base58()),
	)

	if ok := checkServerError(c, err, errGenericFileIo); !ok {
		return
	}

	logo, err := loadImage(png.Decode, "logo_preview.png")

	if ok := checkServerError(c, err, errGenericFileIo); !ok {
		return
	}

	rect := thumbnail.Bounds()
	preview := image.NewRGBA(rect)
	for x := rect.Min.X; x <= rect.Max.X; x++ {
		for y := rect.Min.Y; y <= rect.Max.Y; y++ {
			preview.Set(x, y, thumbnail.At(x, y))
		}
	}

	drawStart := image.Pt(thumbnail.Bounds().Dx()-logo.Bounds().Dx()-10, 10)
	draw.Draw(
		preview,
		image.Rectangle{drawStart, drawStart.Add(logo.Bounds().Size())},
		logo,
		logo.Bounds().Min,
		draw.Over,
	)

	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, preview, nil)

	if ok := checkServerError(c, err, errGenericFileIo); !ok {
		return
	}

	c.Data(http.StatusOK, "image/jpeg", buf.Bytes())
}

func GetVideoStream(c *gin.Context) {
	user := getUserParam(c)
	video := getVideoParam(c)
	dataRoot := utils.GetEnvVar(utils.DATA_ROOT)
	filepath := fmt.Sprintf("%s/videos/%s/video", dataRoot, video.IdSnowflake().Base58())

	c.File(filepath)
	c.Header("Content-Type", video.MimeType)

	bytesStreamed := int64(c.Writer.Size())

	if bytesStreamed <= 0 {
		return
	}

	err := video.ProcessStream(user, bytesStreamed)

	recordError(c, err)
}

func GetRequiredWatchTimeMs(c *gin.Context) {
	user := getUserParam(c)
	video := getVideoParam(c)

	code, requiredWatchTime, err := video.RequiredWatchTimeMs(user)

	switch code {
	case models.WATCH_TIME_FAILURE:
		serverError(c, err, errVideoGetWatchTime)
	case models.WATCH_TIME_ALREADY_WATCHED:
		c.JSON(http.StatusOK, -1)
	case models.WATCH_TIME_SUCCESS:
		c.JSON(http.StatusOK, requiredWatchTime)
	}
}

func StillWatching(c *gin.Context) {
	user := getUserParam(c)
	video := getVideoParam(c)

	result, err := video.TryAddView(user)

	if ok := checkServerError(c, err, errVideoProcessStillWatching); !ok {
		return
	}

	switch result.Code {
	case models.ADD_VIEW_SUCCESS:
		c.Status(http.StatusNoContent)
	case models.ADD_VIEW_DUPLICATE:
		userError(c, errVideoViewAlreadyCounted)
	case models.ADD_VIEW_TIME_NOT_PASSED:
		validationError(c, fmt.Sprintf("You need to watch another %dms.", result.TimeLeftMs))
	case models.ADD_VIEW_VIDEO_NOT_STREAMED_ENOUGH:
		validationError(c, fmt.Sprintf("You need to stream another %d bytes.", result.BytesLeft))
	}
}
