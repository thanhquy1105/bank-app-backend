// Package fs implements github.com/tinode/chat/server/media interface by storing media objects in a single
// directory in the file system.
// This module won't perform well with tens of thousand of files because it stores all files in a single directory.
package fs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/thanhquy1105/simplebank/media"
	"github.com/thanhquy1105/simplebank/util"
)

const (
	handlerName = "fs"
)

type configType struct {
	FileUploadDirectory string `json:"upload_dir"`
}

type fshandler struct {
	// In case of a cluster fileUploadLocation must be accessible to all cluster members.
	fileUploadLocation string
}

func (fh *fshandler) Init(mediaConfig util.MediaConfig) error {
	fh.fileUploadLocation = mediaConfig.FSUpLoadDir
	if fh.fileUploadLocation == "" {
		return errors.New("missing upload location")
	}

	// Make sure the upload directory exists.
	return os.MkdirAll(fh.fileUploadLocation, 0777)
}

// Upload processes request for file upload. The file is given as io.Reader.
func (fh *fshandler) Upload(filename string, file io.ReadSeeker) (string, int64, error) {
	// FIXME: create two-three levels of nested directories. Serving from a single directory
	// with tens of thousands of files in it will not perform well.

	// Generate a unique file name and attach it to path. Using base32 instead of base64 to avoid possible
	// file name collisions on Windows due to case-insensitive file names there.
	location := filepath.Join(fh.fileUploadLocation, filename)

	outfile, err := os.Create(location)
	if err != nil {
		log.Error().Msg(fmt.Sprintln("Upload: failed to create file", location, err))
		return "", 0, err
	}

	size, err := io.Copy(outfile, file)
	outfile.Close()
	if err != nil {
		os.Remove(location)
		return "", 0, err
	}

	return filename, size, nil
}

// Delete deletes files from storage by provided slice of locations.
func (fh *fshandler) Delete(locations []string) error {
	for _, loc := range locations {
		if err, _ := os.Remove(fh.fileUploadLocation + "/" + loc).(*os.PathError); err != nil {
			if err != os.ErrNotExist {
				log.Error().Msg(fmt.Sprintln("fs: error deleting file: ", loc, err))
			}
		}
	}
	return nil
}

func init() {
	media.RegisterMediaHandler(handlerName, &fshandler{})
}
