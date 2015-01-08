package main

/*
#cgo pkg-config: taglib_c
#include <stdlib.h>
#include <taglib/tag_c.h>
*/
import "C"

import (
	"path/filepath"
	"os"
	"log"
	"unsafe"
	"sync"
)

type TrackInfo struct {
	artist string
	title string
	album string
}

type TrackChannel <-chan TrackInfo
type TrackHandler func(TrackChannel)

// prevent concurrent access to taglib_* functions
var taglib_lock sync.Mutex

func goString(cString *C.char) string {
	if cString == nil {
		return ""
	}

	defer C.free(unsafe.Pointer(cString))
	return C.GoString(cString)
}

func readTracks(dirs ...string) TrackChannel {
	trackChannel := make(chan TrackInfo)
	go func() {
		defer close(trackChannel)
		for _,dir := range dirs {
			err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					log.Printf("ERROR \"%s\" %s", path, err.Error())
				}

				if info.Mode().IsRegular() {
					file := C.CString(path)
					defer C.free(unsafe.Pointer(file))

					taglib_lock.Lock()
					defer taglib_lock.Unlock()

					track := C.taglib_file_new(file)
					if track == nil || C.taglib_file_is_valid(track) == 0 {
						log.Printf("Skipping file: %s", path)
						return nil // skip file
					}
					defer C.taglib_file_free(track)

					tag := C.taglib_file_tag(track)
					trackChannel <- TrackInfo {
						artist: goString(C.taglib_tag_artist(tag)),
						title: goString(C.taglib_tag_title(tag)),
						album: goString(C.taglib_tag_album(tag)),
					}		
				}
				return nil
			})
			if err != nil {
				log.Fatal(err)
			}
		}
	}()
	return trackChannel
}

func init() {
	C.taglib_set_string_management_enabled(0)
}

func ReadMedia(processor TrackHandler, dirs ...string) {
	processor(readTracks(dirs...))
}
