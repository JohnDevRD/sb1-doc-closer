package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	ServiceLayerURL string `json:"service_layer_url"`
	CompanyDB       string `json:"company_db"`
}

const configFile = "config.json"

func LoadConfig() (Config, error) {
	var cfg Config
	file, err := os.Open(configFile)
	if err != nil {
		return cfg, err
	}
	defer file.Close()
	err = json.NewDecoder(file).Decode(&cfg)
	return cfg, err
}

func SaveConfig(cfg Config) error {
	file, err := os.Create(configFile)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cfg)
}

func ConfigurePrompt(currentCfg Config) Config {
	ClearScreen()
	fmt.Println("=== CONFIGURACIÓN DE CONEXIÓN ===")
	fmt.Printf("URL Service Layer [%s]: ", currentCfg.ServiceLayerURL)
	var url string
	fmt.Scanln(&url)
	if url != "" {
		currentCfg.ServiceLayerURL = url
	}

	fmt.Printf("Company DB [%s]: ", currentCfg.CompanyDB)
	var db string
	fmt.Scanln(&db)
	if db != "" {
		currentCfg.CompanyDB = db
	}

	if err := SaveConfig(currentCfg); err != nil {
		fmt.Printf("Error al guardar la configuración: %v\n", err)
	} else {
		fmt.Println("Configuración guardada correctamente.")
	}
	Pause("")
	return currentCfg
}