package utils

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
)

// ImageToJpgBuffer Convert and image to a jpg buffer to write to output
func ImageToJpgBuffer(image image.Image, options *jpeg.Options) (*[]byte, error) {
	buf := new(bytes.Buffer)

	err := jpeg.Encode(buf, image, options)
	if err != nil {
		return nil, errors.New("jpeg encode error")
	}
	Buffer := buf.Bytes()
	return &Buffer, nil
}

// ImageToPngBuffer Convert and image to a png buffer to write to output
func ImageToPngBuffer(image image.Image) (*[]byte, error) {
	buf := new(bytes.Buffer)

	err := png.Encode(buf, image)
	if err != nil {
		return nil, errors.New("png encode error")
	}
	Buffer := buf.Bytes()
	return &Buffer, nil
}
