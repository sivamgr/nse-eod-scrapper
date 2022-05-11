package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-co-op/gocron"
)

var cmEodDataPath string = "/opt/appdata/nse-cm-eod/"

const maxDays int = 365
const maxArchiveKeepDays int = 1000

func downNseCmEodFileForDate(toDownload time.Time) {
	toDownloadfileName := strings.ToUpper(toDownload.Format("02Jan2006"))
	ToDownloadURL := fmt.Sprintf("https://archives.nseindia.com/content/historical/EQUITIES/%s/%s/cm%sbhav.csv.zip", toDownloadfileName[5:], toDownloadfileName[2:5], toDownloadfileName)
	log.Println("To Download ", ToDownloadURL)
	outZipFilePath := cmEodDataPath + "tmp.zip"
	unZipPath := cmEodDataPath + "tmp-unzip"
	extractedFilePath := unZipPath + "/" + fmt.Sprintf("cm%sbhav.csv", toDownloadfileName)
	archiveDataPath := cmEodDataPath + toDownload.Format("20060102") + ".csv"

	os.Remove(outZipFilePath)
	os.Remove(unZipPath)

	err := downloadFile(outZipFilePath, ToDownloadURL)
	if err != nil {
		log.Println(err)
	}
	if exists(outZipFilePath) {
		unzip(outZipFilePath, unZipPath)
		if exists(extractedFilePath) {
			os.Rename(extractedFilePath, archiveDataPath)
		}
	}
}

func syncNSEEodData() {
	log.Println("Sync")
	deleteOldDataFiles(cmEodDataPath)
	toDownload := getNextEodDate(cmEodDataPath)

	for time.Now().After(toDownload) {
		//log.Println(toDownload)
		if (toDownload.Weekday() > time.Sunday) && (toDownload.Weekday() < time.Saturday) {
			downNseCmEodFileForDate(toDownload)
		}
		toDownload = toDownload.Add(24 * time.Hour)
		time.Sleep(1 * time.Second)
	}
}

func tick() {
}

func main() {
	log.Printf("NSE EOD Scrapper\n")
	os.MkdirAll(cmEodDataPath, os.ModePerm)
	if exists(cmEodDataPath) == false {
		cmEodDataPath, _ = os.Getwd()
		cmEodDataPath += "/appdata/nse-cm-eod/"
		os.MkdirAll(cmEodDataPath, os.ModePerm)
		log.Printf("Failed to create required appdata directories in /opt/appdata. Switching to %s\n", cmEodDataPath)
	}

	syncNSEEodData()

	s := gocron.NewScheduler(time.UTC)
	s.Every(1).Week().Monday().Tuesday().Wednesday().Thursday().Friday().At("13:00").Do(syncNSEEodData)
	s.StartBlocking()
}

// exists returns whether the given file or directory exists
func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func deleteOldDataFiles(dirName string) {
	to_date := time.Now()

	filepath.Walk(dirName, func(path string, info os.FileInfo, err error) error {
		//log.Printf("%s\n", path)
		toDelete := false
		if (err != nil) || (info.IsDir()) {
			return nil
		}

		if strings.HasSuffix(info.Name(), ".csv") {
			if info.Size() < 1024 {
				toDelete = true
			} else if len(info.Name()) == 12 {
				this_date, err := time.Parse("20060102", info.Name()[:8])
				if err != nil {
					return nil
				}
				file_age := int(to_date.Sub(this_date).Hours() / 24)

				if file_age > maxArchiveKeepDays {
					toDelete = true
				}
			}
		}
		if toDelete {
			os.Remove(path)
		}
		return nil
	})
}

func getNextEodDate(dirName string) time.Time {
	toDownloadDate := time.Now().Add(time.Duration(maxDays*-24) * time.Hour)
	filepath.Walk(dirName, func(path string, info os.FileInfo, err error) error {
		if (err != nil) || (info.IsDir()) {
			return nil
		}

		if strings.HasSuffix(info.Name(), "csv") {
			if info.Size() < 1024 {
				return nil
			}

			if len(info.Name()) == 12 {
				this_date, err := time.Parse("20060102", info.Name()[:8])
				if err != nil {
					return nil
				}
				if toDownloadDate.Before(this_date) {
					toDownloadDate = this_date

				}
			}
		}
		return nil
	})

	return toDownloadDate.Add(24 * time.Hour)
}

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func downloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	os.MkdirAll(dest, 0755)

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(dest, f.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}
