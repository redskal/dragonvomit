package settings

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"strings"
)

type UserSettings struct {
	BingKey string `json:"bing"`
	// https://developers.google.com/custom-search/v1/using_rest
	GoogleKey string `json:"googleKey"` // API key
	GoogleId  string `json:"googleId"`  // Custom Search Engine ID
}

// ReadUserSettings reads the settings file
func ReadUserSettings(fileName string) (UserSettings, error) {
	var settings UserSettings

	jsonFile, err := os.Open(fileName)
	if err != nil {
		return settings, errors.New("error opening user settings")
	}
	defer jsonFile.Close()

	fileContentBytes, err := io.ReadAll(jsonFile)
	if err != nil {
		return settings, errors.New("error reading user settings")
	}

	if err = json.Unmarshal(fileContentBytes, &settings); err != nil {
		log.Print("error unmarshalling user settings")
	}

	return settings, nil
}

// SetUserSettings writes settings to file
func SetUserSettings(args, fileName string) error {
	var newSettings UserSettings

	// pre-load newSettings so we don't bork things (if file exists)
	if _, err := os.Stat(fileName); err == nil {
		newSettings, err = ReadUserSettings(fileName)
		if err != nil {
			return err
		}
	}

	convertedSettings := processSettingsString(args)

	for k, v := range convertedSettings {
		switch k {
		case "bing":
			newSettings.BingKey = v
		case "googleKey":
			newSettings.GoogleKey = v
		case "googleId":
			newSettings.GoogleId = v
		}
	}

	jsonData, err := json.Marshal(newSettings)
	if err != nil {
		return errors.New("error marshalling settings into JSON")
	}

	if err := os.WriteFile(fileName, jsonData, 0644); err != nil {
		return errors.New("error writing JSON to settings file")
	}

	return nil
}

// processSettingsString converts "bing=1,googleKey=2" into a map object
func processSettingsString(args string) map[string]string {
	settingsReturn := make(map[string]string)
	var tmp []string

	for _, value := range strings.Split(args, ",") {
		tmp = strings.Split(value, "=")
		settingsReturn[tmp[0]] = tmp[1]
	}

	return settingsReturn
}
