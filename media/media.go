// Package media defines an interface which must be implemented by media upload/download handlers.
package media

import (
	"io"

	"github.com/thanhquy1105/simplebank/util"
)

// Handler is an interface which must be implemented by media handlers (uploaders-downloaders).
type Handler interface {
	// Init initializes the media upload handler.
	Init(util.MediaConfig) error

	// Upload processes request for file upload. Returns file URL, file size, error.
	Upload(filename string, file io.ReadSeeker) (string, int64, error)

	// Delete deletes file from storage.
	Delete(locations []string) error
}

var mediaHandler Handler
var fileHandlers map[string]Handler

// RegisterMediaHandler saves reference to a media handler (file upload-download handler).
func RegisterMediaHandler(name string, mh Handler) {
	if fileHandlers == nil {
		fileHandlers = make(map[string]Handler)
	}

	if mh == nil {
		panic("RegisterMediaHandler: handler is nil")
	}
	if _, dup := fileHandlers[name]; dup {
		panic("RegisterMediaHandler: called twice for handler " + name)
	}
	fileHandlers[name] = mh
}

// UseMediaHandler sets specified media handler as default.
func UseMediaHandler(config util.MediaConfig) (Handler, error) {
	mediaHandler = fileHandlers[config.UseHandle]
	if mediaHandler == nil {
		panic("UseMediaHandler: unknown handler '" + config.UseHandle + "'")
	}

	if err := mediaHandler.Init(config); err != nil {
		return nil, err
	}

	return mediaHandler, nil
}
