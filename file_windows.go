package main

import (
	"os"
	"syscall"
	"time"
)

func getFileTime(file *os.File) (time.Time, error) {
	fs := &syscall.ByHandleFileInformation{}
	err := syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), fs)
	if err != nil {
		return time.Time{}, err
	}
	var nano int64
	if fs.CreationTime.Nanoseconds() < fs.LastWriteTime.Nanoseconds() {
		nano = fs.CreationTime.Nanoseconds()
	} else {
		nano = fs.LastWriteTime.Nanoseconds()
	}

	return time.Unix(nano/1e9, 0), nil
}
