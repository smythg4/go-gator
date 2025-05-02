package config

import (
	"encoding/json"
	"os"
	"io"
	"path/filepath"
	"fmt"
)

type Config struct {
	DbUrl				string 		`json:"db_url"`
	CurrentUserName		string		`json:"current_user_name"`
}

func getConfigFilePath() (string, error) {
	basepath, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("Error finding user home directory: %w", err)
	}
	filepath := filepath.Join(basepath, ".gatorconfig.json")
	return filepath, nil
}

func write(cfg Config) error {

	//json marshall into a json object
	jsondata, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("Error marshaling JSON: %w", err)
	}
	//open config file
	filepath, err := getConfigFilePath()
	file, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644) //wtf is all this?
	if err != nil {
		return fmt.Errorf("Error opening file %s: %w", filepath, err)
	}
	defer file.Close()
	//write json object into config file
	_, err = file.Write(jsondata)
	if err != nil {
		return fmt.Errorf("Error writing new json to config file: %w", err)
	}

	return nil
}

func Read() (Config, error) {

	filepath, err := getConfigFilePath()
	file, err := os.Open(filepath)
	if err != nil {
		return Config{}, fmt.Errorf("Error opening file %s: %w", filepath, err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return Config{}, fmt.Errorf("Error reading file contents: %w", err)
	}

	var conf Config 
	err = json.Unmarshal(data, &conf)
	if err != nil {
		return Config{}, fmt.Errorf("Error unmarshaling JSON: %w", err)
	}

	return conf, nil
}

func (c *Config) SetUser(username string) error {
	c.CurrentUserName = username 
	err := write(*c)
	return err
}