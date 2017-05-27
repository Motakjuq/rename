package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xor-gate/goexif2/exif"
)

type Config struct {
	OriginPath      string
	DestinyPath     string
	Digits          int
	KeepOriginals   bool
	Verbose         bool
	fileNameFormat  string
	directoryFormat string
}

type Rename struct {
	config      Config
	fileList    []*fileInfo
	directories map[string]int
}

type fileInfo struct {
	path    string
	created time.Time
	ext     string
}

// Process performs the rename operation
func Process(conf Config) error {
	path, err := filepath.Abs(conf.OriginPath)
	if err != nil {
		return err
	}

	sourceStat, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !sourceStat.IsDir() {
		return fmt.Errorf("Source path is not a directory")
	}
	conf.OriginPath = path

	path, err = filepath.Abs(conf.DestinyPath)
	if err != nil {
		return err
	}
	conf.DestinyPath = path
	idx := strings.LastIndex(conf.DestinyPath, string(os.PathSeparator))
	if idx == -1 {
		return fmt.Errorf("Invalid destination file format")
	}
	format := conf.DestinyPath[idx+1:]
	if len(format) == 0 || !strings.Contains(format, "{count}") {
		return fmt.Errorf("Invalid destination file format")
	}
	conf.directoryFormat = conf.DestinyPath[:idx]
	conf.fileNameFormat = format

	if conf.Digits < 0 {
		return fmt.Errorf("Invalid digits value")
	}

	r := &Rename{
		config:      conf,
		directories: make(map[string]int),
	}

	return r.run()
}

func (r *Rename) run() error {
	err := r.walk(r.config.OriginPath)
	if err != nil {
		return err
	}
	sort.Sort(fileInfoList(r.fileList))

	return r.copy()
}

func createReplacer(ts time.Time) *strings.Replacer {
	const format string = "%02d"
	rep := strings.NewReplacer("%Y", fmt.Sprintf("%d", ts.Year()),
		"%M", fmt.Sprintf(format, int(ts.Month())),
		"%d", fmt.Sprintf(format, ts.Day()),
		"%h", fmt.Sprintf(format, ts.Hour()),
		"%m", fmt.Sprintf(format, ts.Minute()),
		"%s", fmt.Sprintf(format, ts.Second()))
	return rep
}

func (r *Rename) copy() error {
	for d := range r.directories {
		log.Println("Creating directory:", d)
		err := os.MkdirAll(d, 755)
		if err != nil {
			return err
		}
	}

	countFormat := fmt.Sprintf("%s0%dd", "%", r.config.Digits)
	filesCount := make(map[string]int)
	buf := &bytes.Buffer{}
	existCount := 1
	for _, f := range r.fileList {
		rep := createReplacer(f.created)
		fileName := rep.Replace(r.config.fileNameFormat)

		if _, has := filesCount[fileName]; !has {
			filesCount[fileName] = 1
		}

		dir := rep.Replace(r.config.directoryFormat)
		buf.WriteString(dir)
		buf.WriteString(string(os.PathSeparator))
		buf.WriteString(strings.Replace(fileName, "{count}", fmt.Sprintf(countFormat, filesCount[fileName]), -1))
		buf.WriteString(".")
		buf.WriteString(f.ext)
		dest := buf.String()

		_, err := os.Stat(dest)
		if err == nil { // file exists
			buf.Truncate(buf.Len() - 1 - len(f.ext))
			buf.WriteString("_")
			buf.WriteString(strconv.Itoa(existCount))
			buf.WriteString(".")
			buf.WriteString(f.ext)
			dest = buf.String()
		}

		if r.config.KeepOriginals {
			copyFile(f.path, dest)
		} else {
			moveFile(f.path, dest)
		}

		r.directories[dir]++
		filesCount[fileName]++
		buf.Reset()
	}

	return nil
}

func moveFile(source, dest string) error {
	log.Println("Moving from:", source, "to:", dest)
	return os.Rename(source, dest)
}

func copyFile(source, dest string) error {
	log.Println("Copying from:", source, "to:", dest)
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()

	fdest, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer fdest.Close()

	_, err = io.Copy(fdest, file)
	if err != nil {
		return err
	}

	return nil
}

func (r *Rename) walk(dirPath string) error {
	content, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, f := range content {
		path := formatPath(dirPath, f.Name())
		if f.IsDir() {
			err = r.walk(path)
			if err != nil {
				return err
			}
			continue
		}
		log.Println("Found", path)
		file, err := os.Open(path)
		if err != nil {
			return err
		}

		err = r.appendFile(path, file)
		if err != nil {
			fmt.Println(err)
		}
		file.Close()
	}

	return nil
}

func formatPath(parent, child string) string {
	if len(child) == 0 {
		return parent
	}
	return fmt.Sprintf("%s%s%s", parent, string(os.PathSeparator), child)
}

func (r *Rename) appendFile(filePath string, file *os.File) error {
	idx := strings.LastIndex(filePath, ".")
	if idx == -1 {
		return fmt.Errorf("Invalid file format")
	}
	fi := &fileInfo{
		path: filePath,
		ext:  filePath[idx+1:],
	}
	_, err := file.Seek(0, 0)
	if err != nil {
		return err
	}

	x, err := exif.Decode(file)
	if err != nil {
		// Get created info from file
		fi.created, err = getFileTime(file)
		if err != nil {
			return err
		}
	} else {
		// Get created info from exif
		fi.created, err = x.DateTime()
		if err != nil {
			// if cannot find the date from exif
			fi.created, err = getFileTime(file)
			if err != nil {
				return err
			}
		}

	}
	r.fileList = append(r.fileList, fi)
	r.directories[r.destDirectory(fi.created)] = 1
	return nil
}

func (r *Rename) destDirectory(ts time.Time) string {
	rep := createReplacer(ts)
	path := rep.Replace(r.config.DestinyPath)
	idx := strings.LastIndex(path, string(os.PathSeparator))
	if idx != -1 {
		return path[:idx]
	}
	return ""
}

type fileInfoList []*fileInfo

func (fl fileInfoList) Len() int {
	return len(fl)
}

func (fl fileInfoList) Less(i, j int) bool {
	return fl[i].created.Before(fl[j].created)
}

func (fl fileInfoList) Swap(i, j int) {
	fl[i], fl[j] = fl[j], fl[i]
}
