package db

import (
	bytes2 "bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

func LoadAndUpdateFile(url string, filePath string, etag string, logger *zap.SugaredLogger) (*os.File, string, error) {
	//create file if not exist
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		_, err = os.Create(filePath)
		if err != nil {
			logger.Errorf("Failed to create file %v - %v\n", filePath, err)
			return nil, "", err
		}

		// Clear etag if file did not exist, otherwise can cause "no new update" with an empty file
		etag = ""
	}

	var file *os.File = nil

	//try to check if there is a new version
	//if so, save the file
	bytes, newEtag, err := downloadBytesFromUrl(url, etag)
	if err == nil {
		//validate json structure
		var test map[string]interface{}
		err = decodeToJsonObject(bytes2.NewReader(bytes), &test)
		if err == nil {
			file, err = saveFile(bytes, filePath)
			etag = newEtag
		} else {
			logger.Infof("ignoring new update [%v], reason - [mailformed json file]", url)
		}
	} else {
		logger.Infof("file [%v] was not downloaded, reason - [%v]", url, err)
	}

	if file == nil {
		//load file
		file, err = os.Open(filePath)
		if err != nil {
			logger.Infof("ignoring new update [%v], reason - [mailformed json file]", url)
			return nil, "", err
		}

		fileInfo, err := os.Stat(filePath)
		if err != nil || fileInfo.Size() == 0 {
			logger.Infof("Local file is empty, or corrupted")
			return nil, "", errors.New("unable to download switch titles db")
		}
	}

	return file, etag, err
}

func decodeToJsonObject(reader io.Reader, target interface{}) error {
	err := json.NewDecoder(reader).Decode(target)
	return err
}

func downloadBytesFromUrl(url string, etag string) ([]byte, string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("If-None-Match", etag)
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 3 * time.Second,
		}).DialContext,
	}
	client := http.Client{
		Transport: transport,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}

	if resp.StatusCode >= 400 {
		return nil, "", errors.New("got a non 200 response - " + resp.Status)
	}
	defer resp.Body.Close()
	//getting the new etag
	etag = resp.Header.Get("Etag")

	if resp.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, "", err
		}
		return body, etag, nil
	}

	return nil, "", errors.New("no new updates")
}

func saveFile(bytes []byte, fileName string) (*os.File, error) {

	err := ioutil.WriteFile(fileName, bytes, 0644)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	return file, nil
}
