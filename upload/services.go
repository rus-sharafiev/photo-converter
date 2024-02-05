package upload

import (
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rus-sharafiev/photo-converter/common/auth"
	"github.com/rus-sharafiev/photo-converter/common/exception"
	"golang.org/x/image/draw"
)

// SERVE UPLOAD -------------------------------------------------------------------
func (c *Controller) serve(w http.ResponseWriter, r *http.Request) {
	// if _, role := auth.Headers(r); role != "ADMIN" {
	// 	exception.Forbidden(w)
	// 	return
	// }

	// TODO check user id, to serve private folder

	w.Header().Add("Cache-Control", "private, max-age=31536000, immutable")
	http.StripPrefix("/upload/", http.FileServer(http.Dir(c.UploadDir))).ServeHTTP(w, r)
}

// HANDLE UPLOAD ------------------------------------------------------------------
func (c *Controller) handle(w http.ResponseWriter, r *http.Request) {

	// Check whether request contains multipart/form-data
	if mr, err := r.MultipartReader(); err == nil {

		// Use user folder if user is authorized
		subFolder := ""
		if userID, _ := auth.Headers(r); len(userID) != 0 {
			subFolder = userID
		}

		sizes := [5]string{Original, Small, Medium, Large, ExtraLarge}
		for _, size := range sizes {
			fullPath := path.Join(c.UploadDir, subFolder, size)

			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				if err := os.Mkdir(fullPath, 0755); err != nil {
					exception.InternalServerError(w, err)
					return
				}
			}
		}

		// Read form
		form, err := mr.ReadForm(32 << 20)
		if err != nil {
			exception.InternalServerError(w, err)
			return
		}

		// Create map to save uploaded files urls
		// filesUrlMap := make(map[string]interface{})
		var galleryImageSlice []GalleryImage

		// Iterate over the range of multipart file fields
		for _, values := range form.File {
			for _, fileHeader := range values {

				// Get file
				file, err := fileHeader.Open()
				if err != nil {
					exception.InternalServerError(w, err)
					return
				}

				// Decode images
				var img image.Image
				switch contentType := fileHeader.Header.Get("Content-Type"); contentType {
				case "image/png":
					img, _ = png.Decode(file)
				case "image/jpeg":
					img, _ = jpeg.Decode(file)
				case "image/heic":
					exception.BadRequestError(w, fmt.Errorf("heic is not supported yet"))
					return
				default:
					exception.BadRequestError(w, fmt.Errorf("file type is not supported. allowed are png, jpeg, heic"))
					return
				}

				imgWidth := img.Bounds().Bounds().Max.X
				imgHeight := img.Bounds().Bounds().Max.Y
				aspectRatio := float32(imgWidth) / float32(imgHeight)

				// Create file name
				id, err := uuid.NewRandom()
				if err != nil {
					exception.InternalServerError(w, err)
					return
				}

				fileName := id.String() + ".png" // filepath.Ext(fileHeader.Filename)
				originalOutputLocation := path.Join(path.Join(c.UploadDir, subFolder, Original), fileName)

				originalOutFile, err := os.Create(originalOutputLocation)
				if err != nil {
					exception.InternalServerError(w, err)
					return
				}
				defer originalOutFile.Close()

				if err := png.Encode(originalOutFile, img); err != nil {
					exception.InternalServerError(w, err)
					return
				}

				// Create sizes
				imageSizes := make(map[string]string)
				for _, size := range sizes[1:] {

					intSize, err := strconv.Atoi(size)
					if err != nil {
						exception.InternalServerError(w, err)
						return
					}

					// Create output file
					outputLocation := path.Join(path.Join(c.UploadDir, subFolder, size), fileName)
					outFile, err := os.Create(outputLocation)
					if err != nil {
						exception.InternalServerError(w, err)
						return
					}
					defer outFile.Close()

					if intSize >= max(imgWidth, imgHeight) {

						// Return original if smaller than target size
						if err := png.Encode(outFile, img); err != nil {
							exception.InternalServerError(w, err)
							return
						}

					} else {

						// Set the size
						var dst *image.RGBA
						if aspectRatio >= 1 {
							dst = image.NewRGBA(image.Rect(0, 0, intSize, int(float32(intSize)/aspectRatio)))
						} else {
							dst = image.NewRGBA(image.Rect(0, 0, int(float32(intSize)*aspectRatio), intSize))
						}

						// Resize
						draw.ApproxBiLinear.Scale(dst, dst.Rect, img, img.Bounds(), draw.Over, nil)

						// Encode to png
						if err := png.Encode(outFile, dst); err != nil {
							exception.InternalServerError(w, err)
							return
						}
					}

					imageSizes[size] = outputLocation
				}

				// Create Gallery Image object
				galleryImage := GalleryImage{
					File:         &fileName,
					Name:         &fileHeader.Filename,
					Url:          &originalOutputLocation,
					OriginalSize: &fileHeader.Size,
					Width:        &imgWidth,
					Height:       &imgHeight,
					Sizes:        &imageSizes,
					CreatedAt:    time.Now(),
				}

				galleryImageSlice = append(galleryImageSlice, galleryImage)

				// Add uploaded file url to map as string or append to existing slice (if multiple with the same field name)
				// if filesUrlMap[name] == nil {

				// 	filesUrlMap[name] = fileName

				// } else {

				// 	if existingStr, ok := filesUrlMap[name].(string); ok {
				// 		filesUrlMap[name] = []string{existingStr, fileName}
				// 	}

				// 	if existingSlice, ok := filesUrlMap[name].([]string); ok {
				// 		filesUrlMap[name] = append(existingSlice, fileName)
				// 	}

				// }
			}

		}

		var (
			resultJson string
		)

		// Convert map with files urls to JSON object
		if len(galleryImageSlice) != 0 {
			filesList, err := json.Marshal(galleryImageSlice)
			if err != nil {
				exception.InternalServerError(w, err)
				return
			}
			resultJson = string(filesList)
		}

		// Write result JSON string to request body
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resultJson)
	} else {
		exception.BadRequest(w)
	}
}
